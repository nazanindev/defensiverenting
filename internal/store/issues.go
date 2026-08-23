package store

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

// The authoring portal separates capture from guarantee (ADR-013). Saving a
// draft records whatever the author has, complete or not: a half-written page
// lost to a validation error is work destroyed, and an author who cannot save
// keeps their work in a browser tab instead of the database. The invariants
// that protect renters — every claim cited, every citation carrying a verbatim
// quote that was actually confirmed at its source, every statute naming its
// provision — are enforced where content crosses to the public site: publishing
// a draft, or saving a page that is already live.
//
// This file is that boundary's single implementation. The dashboard's issue
// badges, the view page's issue list, and the publish gate all read the same
// checks, so what the author is shown is exactly what publishing will refuse.

// PageIssue is one violation of the publish invariants on one page.
type PageIssue struct {
	// Code names the invariant, stable across wording changes:
	// no-title, no-statements, empty-statement, uncited-statement,
	// missing-quote, unverified-quote, source-unreachable, statute-locator,
	// source-no-publisher.
	Code string
	// Detail is the reviewer-facing sentence, naming the statement or source.
	Detail string
	// Stmt is the 1-based position of the statement the issue sits on, so the
	// editor can scroll to it; 0 for page-level issues.
	Stmt int
}

// NotPublishableError reports why a page may not cross to the public site. It
// is returned by AuthorPublishPlaybook, and by AuthorUpdatePlaybook when the
// save would rewrite a page that is already live.
type NotPublishableError struct {
	Issues []PageIssue
}

func (e *NotPublishableError) Error() string {
	details := make([]string, len(e.Issues))
	for i, is := range e.Issues {
		details[i] = is.Detail
	}
	return fmt.Sprintf("cannot publish: %d critical issue(s) — %s",
		len(e.Issues), strings.Join(details, "; "))
}

// rowQuerier is the one thing the issue checks need, satisfied by both a pool
// and a transaction so the publish gate can run inside the publishing tx.
type rowQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// AuthorPlaybookIssues returns one page's critical issues, in the order the
// page reads: page-level problems first, then per-statement ones by position.
func (pg *PG) AuthorPlaybookIssues(ctx context.Context, id int64) ([]PageIssue, error) {
	m, err := collectIssues(ctx, pg.pool, "pb.id = $1", id)
	if err != nil {
		return nil, err
	}
	return m[id], nil
}

// AuthorDraftIssues returns critical issues for every draft, keyed by playbook
// id, for the dashboard's issue badges. Drafts only: a published page is
// checked when it is saved or re-published, and a superseded page is history.
func (pg *PG) AuthorDraftIssues(ctx context.Context) (map[int64][]PageIssue, error) {
	return collectIssues(ctx, pg.pool, "pb.status = 'draft'")
}

// validatePublishable is the publish gate: it refuses when the page carries
// any critical issue. Run inside the transaction that publishes (or that saves
// a live page), so a refused page changes nothing.
func validatePublishable(ctx context.Context, q rowQuerier, playbookID int64) error {
	m, err := collectIssues(ctx, q, "pb.id = $1", playbookID)
	if err != nil {
		return err
	}
	if issues := m[playbookID]; len(issues) > 0 {
		return &NotPublishableError{Issues: issues}
	}
	return nil
}

// collectIssues runs every invariant check over the playbooks selected by
// cond (a WHERE fragment over alias pb, e.g. "pb.id = $1"). One implementation
// serves both shapes — one page inside a tx, every draft for the dashboard —
// so the two can never disagree about what an issue is.
func collectIssues(ctx context.Context, q rowQuerier, cond string, args ...any) (map[int64][]PageIssue, error) {
	out := map[int64][]PageIssue{}
	add := func(id int64, stmt int, code, detail string) {
		out[id] = append(out[id], PageIssue{Code: code, Detail: detail, Stmt: stmt})
	}
	// Positions arrive as text columns; they are always digits.
	pos := func(f string) int { n, _ := strconv.Atoi(f); return n }

	// A page with no title has no headline, no <title>, and no search result.
	if err := scanIssueRows(ctx, q, `
		SELECT pb.id FROM playbooks pb
		WHERE `+cond+` AND btrim(pb.title) = ''`, args,
		func(id int64, _ []string) { add(id, 0, "no-title", "the page has no title") },
	); err != nil {
		return nil, err
	}

	// A page with no statements would publish as an empty shell.
	if err := scanIssueRows(ctx, q, `
		SELECT pb.id FROM playbooks pb
		WHERE `+cond+` AND NOT EXISTS
			(SELECT 1 FROM playbook_statements ps WHERE ps.playbook_id = pb.id)`, args,
		func(id int64, _ []string) { add(id, 0, "no-statements", "the page has no statements") },
	); err != nil {
		return nil, err
	}

	// An empty statement card is a placeholder the author meant to come back to.
	if err := scanIssueRows(ctx, q, `
		SELECT pb.id, (ps.position + 1)::text
		FROM playbook_statements ps
		JOIN playbooks pb ON pb.id = ps.playbook_id
		JOIN statements s ON s.id = ps.statement_id
		WHERE `+cond+` AND btrim(s.body_md) = ''
		ORDER BY pb.id, ps.position`, args,
		func(id int64, f []string) {
			add(id, pos(f[0]), "empty-statement", fmt.Sprintf("statement %s has no text", f[0]))
		},
	); err != nil {
		return nil, err
	}

	// Every claim must be traceable to a source (ADR-003).
	if err := scanIssueRows(ctx, q, `
		SELECT pb.id, (ps.position + 1)::text
		FROM playbook_statements ps
		JOIN playbooks pb ON pb.id = ps.playbook_id
		WHERE `+cond+` AND NOT EXISTS
			(SELECT 1 FROM citations c WHERE c.statement_id = ps.statement_id)
		ORDER BY pb.id, ps.position`, args,
		func(id int64, f []string) {
			add(id, pos(f[0]), "uncited-statement", fmt.Sprintf("statement %s has no citation — cite a source or mark it editorial guidance", f[0]))
		},
	); err != nil {
		return nil, err
	}

	// The verbatim quote is what makes a citation checkable (ADR-003): without
	// one, the source checker can never verify the claim, which is how 17 of 19
	// legacy pages reached production carrying citations nobody could check.
	// Editorial sources are exempt as everywhere else: they cite no external
	// text by design, so there is nothing to quote.
	if err := scanIssueRows(ctx, q, `
		SELECT pb.id, (ps.position + 1)::text, src.url
		FROM playbook_statements ps
		JOIN playbooks pb ON pb.id = ps.playbook_id
		JOIN citations c ON c.statement_id = ps.statement_id
		JOIN sources src ON src.id = c.source_id
		WHERE `+cond+` AND src.kind <> 'editorial' AND btrim(c.quote) = ''
		ORDER BY pb.id, ps.position, src.url`, args,
		func(id int64, f []string) {
			add(id, pos(f[0]), "missing-quote", fmt.Sprintf("statement %s cites %s with no verbatim quote", f[0], f[1]))
		},
	); err != nil {
		return nil, err
	}

	// A quote that was typed but never confirmed at its source is exactly what
	// the old save-time check refused. Saving it is capture; publishing it
	// would put unverified words in front of renters. checked_at is stamped by
	// every path that actually looks (a live fetch that found the quote, a
	// reviewer's attestation, a check-sources run), so NULL here is honest.
	//
	// Reported per SOURCE, not per statement: one blocked source behind five
	// statements used to read as five separate complaints the reviewer had to
	// work backwards from. A source the checker has never managed to examine
	// (last_checked_at NULL, despite the periodic runs) gets its own code —
	// the fix there is opening the source yourself, not re-checking.
	if err := scanIssueRows(ctx, q, `
		SELECT pb.id, src.url,
		       string_agg((ps.position + 1)::text, ', ' ORDER BY ps.position),
		       (src.last_checked_at IS NULL)::text
		FROM playbook_statements ps
		JOIN playbooks pb ON pb.id = ps.playbook_id
		JOIN citations c ON c.statement_id = ps.statement_id
		JOIN sources src ON src.id = c.source_id
		WHERE `+cond+` AND src.kind <> 'editorial'
		  AND btrim(c.quote) <> '' AND c.checked_at IS NULL AND NOT c.manually_verified
		GROUP BY pb.id, src.url, src.last_checked_at
		ORDER BY pb.id, src.url`, args,
		func(id int64, f []string) {
			first := pos(strings.SplitN(f[1], ",", 2)[0])
			if f[2] == "true" {
				add(id, first, "source-unreachable", fmt.Sprintf("the checker has never managed to read %s; it may block automated fetching. Statement(s) %s cite it with unconfirmed quotes — open the source yourself and attest each quote in the editor", f[0], f[1]))
				return
			}
			add(id, first, "unverified-quote", fmt.Sprintf("quotes from %s were never confirmed at the source (statement(s) %s) — re-check them in the editor, or attest by hand", f[0], f[1]))
		},
	); err != nil {
		return nil, err
	}

	// A statute citation must name its provision (see locator.go): a locator
	// that names none is a pointer at a document that merely discusses the law.
	if err := scanIssueRows(ctx, q, `
		SELECT pb.id, (ps.position + 1)::text, src.url, c.locator
		FROM playbook_statements ps
		JOIN playbooks pb ON pb.id = ps.playbook_id
		JOIN citations c ON c.statement_id = ps.statement_id
		JOIN sources src ON src.id = c.source_id
		WHERE `+cond+` AND src.kind = 'statute'
		ORDER BY pb.id, ps.position, src.url`, args,
		func(id int64, f []string) {
			if looksLikeSection(f[2]) {
				return
			}
			add(id, pos(f[0]), "statute-locator", fmt.Sprintf("statement %s cites %s as a statute but its locator %q does not name a provision (for example %q or %q)", f[0], f[1], f[2], "§ 15B", "RCW 59.18.060"))
		},
	); err != nil {
		return nil, err
	}

	// A source with no publisher renders a citation chip with no name.
	if err := scanIssueRows(ctx, q, `
		SELECT DISTINCT pb.id, src.url
		FROM playbook_statements ps
		JOIN playbooks pb ON pb.id = ps.playbook_id
		JOIN citations c ON c.statement_id = ps.statement_id
		JOIN sources src ON src.id = c.source_id
		WHERE `+cond+` AND src.kind <> 'editorial' AND btrim(src.publisher) = ''
		ORDER BY pb.id, src.url`, args,
		func(id int64, f []string) {
			add(id, 0, "source-no-publisher", fmt.Sprintf("the source %s has no publisher name", f[0]))
		},
	); err != nil {
		return nil, err
	}

	return out, nil
}

// scanIssueRows runs one check query and hands each row — the playbook id plus
// any further text columns — to emit.
func scanIssueRows(ctx context.Context, q rowQuerier, sql string, args []any, emit func(id int64, fields []string)) error {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("check page issues: %w", err)
	}
	defer rows.Close()
	n := len(rows.FieldDescriptions()) - 1
	for rows.Next() {
		var id int64
		fields := make([]string, n)
		dest := make([]any, 0, n+1)
		dest = append(dest, &id)
		for i := range fields {
			dest = append(dest, &fields[i])
		}
		if err := rows.Scan(dest...); err != nil {
			return fmt.Errorf("scan page issue: %w", err)
		}
		emit(id, fields)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("check page issues: %w", err)
	}
	return nil
}
