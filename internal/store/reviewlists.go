package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Group review (ADR-018 D4, D5). Review is per statement, so it can be done
// across pages: every statement citing one source, or one concept stated by
// several jurisdictions. These lists and actions read and write the same
// stamp as the page view; nothing here is a second gate.

// ReviewRow is one statement in a group list, with the page it sits on.
type ReviewRow struct {
	PlaybookID   int64
	PageTitle    string
	PageStatus   string
	PageKind     string
	Jurisdiction string
	Topic        string
	Position     int // 1-based
	Stmt         CitedStatement
}

// SourceReviewSummary is one source's standing across the site: how many
// statements cite it, how many of those lack a review stamp, how many of
// its quotes were never confirmed, and whether the checker could read it.
type SourceReviewSummary struct {
	SourceID      int64
	URL           string
	Publisher     string
	Kind          string
	Statements    int
	Unreviewed    int
	Unconfirmed   int
	LastCheckedAt *time.Time
	LastFetchNote string
	// Unreadable reports that the checker's most recent attempt could not
	// read the source: attesting quotes by hand is the way through.
	Unreadable bool
}

// ConceptReviewSummary is one concept's standing: statements tagged with it
// on draft or live pages, and how many lack a review stamp.
type ConceptReviewSummary struct {
	Slug       string
	Name       string
	TopicSlug  string
	Statements int
	Unreviewed int
}

const reviewScopeSQL = `pb.status IN ('draft', 'published') AND pb.language = ANY($1)`

// ReviewSourcesOverview lists every non-editorial source cited on a draft or
// live page, the ones needing attention first.
func (pg *PG) ReviewSourcesOverview(ctx context.Context) ([]SourceReviewSummary, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT src.id, src.url, src.publisher, src.kind, src.last_checked_at, COALESCE(src.last_fetch_note, ''),
		       (src.last_fetch_note <> '' AND (src.last_checked_at IS NULL OR src.last_fetch_at > src.last_checked_at)),
		       count(DISTINCT s.id),
		       count(DISTINCT s.id) FILTER (WHERE NOT (s.last_reviewed_at IS NOT NULL AND s.reviewed_hash = h.hash)),
		       count(DISTINCT c.statement_id) FILTER (WHERE btrim(c.quote) <> '' AND c.checked_at IS NULL AND NOT c.manually_verified)
		FROM sources src
		JOIN citations c ON c.source_id = src.id
		JOIN statements s ON s.id = c.statement_id
		JOIN statement_review_hash h ON h.statement_id = s.id
		JOIN playbook_statements ps ON ps.statement_id = s.id
		JOIN playbooks pb ON pb.id = ps.playbook_id
		WHERE src.kind <> 'editorial' AND `+reviewScopeSQL+`
		GROUP BY src.id
		ORDER BY 9 DESC, 10 DESC, 8 DESC, src.url`, ContentLanguages)
	if err != nil {
		return nil, fmt.Errorf("review sources: %w", err)
	}
	defer rows.Close()
	var out []SourceReviewSummary
	for rows.Next() {
		var r SourceReviewSummary
		if err := rows.Scan(&r.SourceID, &r.URL, &r.Publisher, &r.Kind, &r.LastCheckedAt, &r.LastFetchNote, &r.Unreadable,
			&r.Statements, &r.Unreviewed, &r.Unconfirmed); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ReviewConceptsOverview lists every concept tagged on a draft or live page,
// the ones with unreviewed statements first.
func (pg *PG) ReviewConceptsOverview(ctx context.Context) ([]ConceptReviewSummary, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT co.slug, co.name, t.slug,
		       count(DISTINCT s.id),
		       count(DISTINCT s.id) FILTER (WHERE NOT (s.last_reviewed_at IS NOT NULL AND s.reviewed_hash = h.hash))
		FROM concepts co
		JOIN topics t ON t.id = co.topic_id
		JOIN statements s ON s.concept_id = co.id
		JOIN statement_review_hash h ON h.statement_id = s.id
		JOIN playbook_statements ps ON ps.statement_id = s.id
		JOIN playbooks pb ON pb.id = ps.playbook_id
		WHERE `+reviewScopeSQL+`
		GROUP BY co.slug, co.name, t.slug
		ORDER BY 5 DESC, 4 DESC, co.name`, ContentLanguages)
	if err != nil {
		return nil, fmt.Errorf("review concepts: %w", err)
	}
	defer rows.Close()
	var out []ConceptReviewSummary
	for rows.Next() {
		var r ConceptReviewSummary
		if err := rows.Scan(&r.Slug, &r.Name, &r.TopicSlug, &r.Statements, &r.Unreviewed); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ReviewStatementsBySource returns every statement on a draft or live page
// that cites the source, with all of its citations, in reading order.
func (pg *PG) ReviewStatementsBySource(ctx context.Context, sourceID int64) ([]ReviewRow, error) {
	return pg.reviewRows(ctx, `EXISTS (SELECT 1 FROM citations cx WHERE cx.statement_id = s.id AND cx.source_id = $2)`, sourceID)
}

// ReviewStatementsByConcept returns every statement tagged with the concept
// on a draft or live page, one per jurisdiction in reading order.
func (pg *PG) ReviewStatementsByConcept(ctx context.Context, slug string) ([]ReviewRow, error) {
	return pg.reviewRows(ctx, `co.slug = $2`, slug)
}

// ReviewStatementsOnDrafts returns every statement on every draft, for the
// dashboard worklist to aggregate standings with the one rule in Standing.
func (pg *PG) ReviewStatementsOnDrafts(ctx context.Context) ([]ReviewRow, error) {
	return pg.reviewRows(ctx, `pb.status = $2`, "draft")
}

// ReviewStatementsWithNotes returns every statement carrying a pending
// reviewer note, on any draft or live page.
func (pg *PG) ReviewStatementsWithNotes(ctx context.Context) ([]ReviewRow, error) {
	return pg.reviewRows(ctx, `$2::text = 'note' AND EXISTS (SELECT 1 FROM statement_proposals sp WHERE sp.statement_key = s.key AND sp.status = 'pending' AND sp.reason = 'agent-pass:flag')`, "note")
}

func (pg *PG) reviewRows(ctx context.Context, scope string, arg any) ([]ReviewRow, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT pb.id, pb.title, pb.status, pb.page_kind, j.name, t.name, ps.position,
		       s.id, s.key::text, s.body_md, COALESCE(co.slug, ''), COALESCE(tr.slug, ''),
		       `+reviewedAtSQL+`, `+reviewedBySQL+`, `+undecidedSQL+`, `+proposalPendingSQL+`,
		       c.source_id, c.locator, c.quote, c.manually_verified, c.checked_at, c.checked_by,
		       src.url, src.publisher, src.kind, COALESCE(`+sourceUnreadableSQL+`, false)
		FROM playbook_statements ps
		JOIN playbooks pb ON pb.id = ps.playbook_id
		JOIN jurisdictions j ON j.id = pb.jurisdiction_id
		JOIN topics t ON t.id = pb.topic_id
		JOIN statements s ON s.id = ps.statement_id
		JOIN statement_review_hash h ON h.statement_id = s.id
		LEFT JOIN concepts co ON co.id = s.concept_id
		LEFT JOIN topics tr ON tr.id = s.topic_ref
		LEFT JOIN citations c ON c.statement_id = s.id
		LEFT JOIN sources src ON src.id = c.source_id
		WHERE `+reviewScopeSQL+` AND `+scope+`
		ORDER BY j.name, t.name, pb.status, ps.position, c.source_id`, ContentLanguages, arg)
	if err != nil {
		return nil, fmt.Errorf("review statements: %w", err)
	}
	defer rows.Close()
	var out []ReviewRow
	idx := map[int64]int{} // statement id -> index in out
	var keys []string
	for rows.Next() {
		var (
			r          ReviewRow
			stmtID     int64
			position   int
			reviewedAt *time.Time
			reviewedBy string
			undecided  bool
			pending    bool
			unreadable bool
			c          CitationWithSource
			sourceID   *int64
			loc, quote *string
			manual     *bool
			checkedBy  *string
			url, pub   *string
			kind       *string
		)
		if err := rows.Scan(&r.PlaybookID, &r.PageTitle, &r.PageStatus, &r.PageKind, &r.Jurisdiction, &r.Topic, &position,
			&stmtID, &r.Stmt.Key, &r.Stmt.BodyMD, &r.Stmt.ConceptSlug, &r.Stmt.TopicRefSlug,
			&reviewedAt, &reviewedBy, &undecided, &pending,
			&sourceID, &loc, &quote, &manual, &c.CheckedAt, &checkedBy, &url, &pub, &kind, &unreadable); err != nil {
			return nil, err
		}
		i, ok := idx[stmtID]
		if !ok {
			r.Position = position + 1
			r.Stmt.ID, r.Stmt.ReviewedAt, r.Stmt.ReviewedBy, r.Stmt.Undecided = stmtID, reviewedAt, reviewedBy, undecided
			r.Stmt.ProposalPending = pending
			i = len(out)
			out = append(out, r)
			idx[stmtID] = i
			keys = append(keys, r.Stmt.Key)
		}
		if sourceID == nil {
			continue
		}
		c.SourceID = *sourceID
		c.Locator, c.Quote = deref(loc), deref(quote)
		c.ManuallyVerified = manual != nil && *manual
		c.CheckedBy = deref(checkedBy)
		c.SourceURL, c.Publisher, c.SourceKind = deref(url), deref(pub), deref(kind)
		c.SourceUnreadable = unreadable
		out[i].Stmt.Citations = append(out[i].Stmt.Citations, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	notes, err := pg.notesByKey(ctx, keys)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Stmt.Notes = notes[out[i].Stmt.Key]
	}
	return out, nil
}

// notesByKey loads pending reviewer notes for a set of statement keys.
func (pg *PG) notesByKey(ctx context.Context, keys []string) (map[string][]StatementNote, error) {
	byKey := map[string][]StatementNote{}
	if len(keys) == 0 {
		return byKey, nil
	}
	rows, err := pg.pool.Query(ctx, `
		SELECT sp.statement_key::text, sp.id, COALESCE(sp.evidence->>'note', ''), sp.proposed_by, sp.created_at
		FROM statement_proposals sp
		WHERE sp.statement_key = ANY($1::uuid[]) AND sp.reason = $2 AND sp.status = 'pending'
		ORDER BY sp.id`, keys, ReasonReviewerFlag)
	if err != nil {
		return nil, fmt.Errorf("load reviewer notes: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var n StatementNote
		if err := rows.Scan(&key, &n.ID, &n.Note, &n.By, &n.At); err != nil {
			return nil, err
		}
		byKey[key] = append(byKey[key], n)
	}
	return byKey, rows.Err()
}

// AttestSourceQuotes records a person's attestation for every unconfirmed
// quote cited from one source on a draft or live page (ADR-018 D5): they
// opened the source themselves and found the words. Returns how many
// citations were stamped.
func (pg *PG) AttestSourceQuotes(ctx context.Context, sourceID int64, by string) (int, error) {
	if !isReviewer(by) {
		return 0, fmt.Errorf("%q cannot attest quotes: attestation is a person's act", by)
	}
	tag, err := pg.pool.Exec(ctx, `
		UPDATE citations c
		   SET manually_verified = true, checked_at = NOW(), checked_by = $3
		 WHERE c.source_id = $2 AND btrim(c.quote) <> '' AND c.checked_at IS NULL
		   AND EXISTS (SELECT 1 FROM playbook_statements ps JOIN playbooks pb ON pb.id = ps.playbook_id
		               WHERE ps.statement_id = c.statement_id AND `+reviewScopeSQL+`)`, ContentLanguages, sourceID, by)
	if err != nil {
		return 0, fmt.Errorf("attest quotes: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// PublishOutcome is one draft's result from a publish-ready run.
type PublishOutcome struct {
	PlaybookID   int64
	Title        string
	Jurisdiction string
	Topic        string
	Published    bool
	Issues       []PageIssue
	Err          error
}

// PublishReadyDrafts publishes every draft that carries no critical issue,
// one transaction per page through the ordinary gate (ADR-018 D5). Drafts
// that carry issues are reported, not touched. A draft that gains an issue
// between the listing and its publish is refused by the gate like any other.
//
// jurisdictionIDs narrows the run to drafts in those places; none means every
// draft.
func (pg *PG) PublishReadyDrafts(ctx context.Context, by string, jurisdictionIDs ...int64) ([]PublishOutcome, error) {
	if !isReviewer(by) {
		return nil, fmt.Errorf("%q cannot publish", by)
	}
	issues, err := pg.AuthorDraftIssues(ctx)
	if err != nil {
		return nil, err
	}
	var scope any
	if len(jurisdictionIDs) > 0 {
		scope = jurisdictionIDs
	}
	rows, err := pg.pool.Query(ctx, `
		SELECT pb.id, pb.title, j.name, t.name
		FROM playbooks pb JOIN jurisdictions j ON j.id = pb.jurisdiction_id JOIN topics t ON t.id = pb.topic_id
		WHERE pb.status = 'draft' AND pb.language = ANY($1)
		  AND ($2::bigint[] IS NULL OR pb.jurisdiction_id = ANY($2::bigint[]))
		ORDER BY j.name, t.name`, ContentLanguages, scope)
	if err != nil {
		return nil, err
	}
	var out []PublishOutcome
	for rows.Next() {
		var o PublishOutcome
		if err := rows.Scan(&o.PlaybookID, &o.Title, &o.Jurisdiction, &o.Topic); err != nil {
			rows.Close()
			return nil, err
		}
		o.Issues = issues[o.PlaybookID]
		out = append(out, o)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		if len(out[i].Issues) > 0 {
			continue
		}
		err := pg.AuthorPublishPlaybook(ctx, out[i].PlaybookID, by)
		switch e := err.(type) {
		case nil:
			out[i].Published = true
		case *NotPublishableError:
			out[i].Issues = e.Issues
		default:
			out[i].Err = err
		}
	}
	return out, nil
}

// ListProposalsByReason is ListProposals narrowed to one reason family:
// "note" for reviewer notes, "drift" for the checker's findings, "" for all.
func (pg *PG) ListProposalsByReason(ctx context.Context, status, family string) ([]ProposalRow, error) {
	rows, err := pg.ListProposals(ctx, status)
	if err != nil || family == "" {
		return rows, err
	}
	out := rows[:0]
	for _, r := range rows {
		switch family {
		case "note":
			if r.Reason == ReasonReviewerFlag {
				out = append(out, r)
			}
		case "drift":
			if r.Reason == "source-drift" {
				out = append(out, r)
			}
		default:
			if !strings.HasPrefix(r.Reason, "agent-pass:flag") && r.Reason != "source-drift" {
				out = append(out, r)
			}
		}
	}
	return out, nil
}

// AttestStatementQuotes records a person's attestation for every unconfirmed
// quote on one statement whose source the checker cannot read: the card's
// "attest" action. Quotes on readable sources are not attested here; they are
// rechecked. Returns how many citations were stamped.
func (pg *PG) AttestStatementQuotes(ctx context.Context, playbookID int64, key, by string) (int, error) {
	if !isReviewer(by) {
		return 0, fmt.Errorf("%q cannot attest quotes: attestation is a person's act", by)
	}
	key = strings.ToLower(strings.TrimSpace(key))
	if !uuidRE.MatchString(key) {
		return 0, fmt.Errorf("%q is not a statement key", key)
	}
	tag, err := pg.pool.Exec(ctx, `
		UPDATE citations c
		   SET manually_verified = true, checked_at = NOW(), checked_by = $3
		  FROM playbook_statements ps, statements s, sources src
		 WHERE ps.playbook_id = $1 AND s.id = ps.statement_id AND s.key = $2::uuid
		   AND c.statement_id = s.id AND src.id = c.source_id
		   AND btrim(c.quote) <> '' AND c.checked_at IS NULL
		   AND `+sourceUnreadableSQL, playbookID, key, by)
	if err != nil {
		return 0, fmt.Errorf("attest statement quotes: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
