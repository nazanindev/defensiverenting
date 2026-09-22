package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Review is per statement; publishing is per page (ADR-018). A statement's
// stamp is who reviewed it, when, and a hash of what they saw, and it is
// honoured only while that hash still matches the content. The hash has one
// definition, the statement_review_hash view (migration 000041); every writer
// and reader here joins it as alias h.

// SQL fragments for the statement queries. They expect statements as s and
// the hash view as h, and read the stamp the way the gate does: a stamp on
// content that has since changed is no stamp.
const (
	reviewedAtSQL = `CASE WHEN s.last_reviewed_at IS NOT NULL AND s.reviewed_hash = h.hash THEN s.last_reviewed_at END`
	reviewedBySQL = `CASE WHEN s.last_reviewed_at IS NOT NULL AND s.reviewed_hash = h.hash THEN s.reviewed_by ELSE '' END`
	// A pending proposal is an undecided question about the claim. Snoozed
	// ones are a deliberate deferral and do not count.
	undecidedSQL = `EXISTS (SELECT 1 FROM statement_proposals sp WHERE sp.statement_key = s.key AND sp.status = 'pending')`
	// A pending proposal that is not a reviewer note: something proposes to
	// change the claim, or the checker lost its quote.
	proposalPendingSQL = `EXISTS (SELECT 1 FROM statement_proposals sp WHERE sp.statement_key = s.key AND sp.status = 'pending' AND sp.reason <> 'agent-pass:flag')`
	// Expects sources as src. True when the last fetch attempt failed to
	// read the source and no successful check has happened since.
	sourceUnreadableSQL = `(src.last_fetch_note <> '' AND (src.last_checked_at IS NULL OR src.last_fetch_at > src.last_checked_at))`
)

// isReviewer reports whether a save actor is a person whose edits count as
// review. The drafting agent, the source checker, and the review agent are
// the non-human writers; everything else is a named person or a tool a
// person runs on purpose (ingest, promote).
func isReviewer(by string) bool {
	by = strings.TrimSpace(by)
	return by != "" && by != ActorDraftingAgent && by != ActorSourceCheck && by != ActorReviewAgent
}

// carryStampIfWidened keeps a person's review stamp on a statement whose
// only change since they read it is a quote grown to hold more of the same
// passage (ADR-021 D2). The stamp is a hash of the body, tags, and every
// citation, so a widened quote would otherwise read as unreviewed; but the
// claim and its evidence are what they were, with more of the evidence
// shown. The stamp moves onto the new hash under the same name and time.
// Anything else that differs, a word of the body, a source, a locator, a
// quote that does not contain the old one, leaves the stamp behind. An
// undecided queue item does not stop the carry: it stops a new stamp, and
// this is the old one, which the item never invalidated.
func carryStampIfWidened(ctx context.Context, tx pgx.Tx, stmtID int64) error {
	type cite struct{ URL, Locator, Quote string }
	read := func(id int64) (body, concept, topic string, cites []cite, err error) {
		if err = tx.QueryRow(ctx, `
			SELECT s.body_md, COALESCE(co.slug, ''), COALESCE(tr.slug, '')
			FROM statements s LEFT JOIN concepts co ON co.id = s.concept_id LEFT JOIN topics tr ON tr.id = s.topic_ref
			WHERE s.id = $1`, id).Scan(&body, &concept, &topic); err != nil {
			return
		}
		rows, err := tx.Query(ctx, `
			SELECT src.url, c.locator, c.quote FROM citations c JOIN sources src ON src.id = c.source_id
			WHERE c.statement_id = $1 ORDER BY src.url, c.locator, c.id`, id)
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			var c cite
			if err = rows.Scan(&c.URL, &c.Locator, &c.Quote); err != nil {
				return
			}
			cites = append(cites, c)
		}
		err = rows.Err()
		return
	}
	var priorID int64
	var reviewedAt *time.Time
	var reviewedBy string
	err := tx.QueryRow(ctx, `
		SELECT p.id, p.last_reviewed_at, p.reviewed_by
		FROM statements s JOIN statements p ON p.key = s.key AND p.id <> s.id
		JOIN statement_review_hash ph ON ph.statement_id = p.id
		WHERE s.id = $1 AND p.last_reviewed_at IS NOT NULL AND p.reviewed_hash = ph.hash
		ORDER BY p.id DESC LIMIT 1`, stmtID).Scan(&priorID, &reviewedAt, &reviewedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // nothing reviewed to carry
	}
	if err != nil {
		return err
	}
	nb, nc, nt, ncites, err := read(stmtID)
	if err != nil {
		return err
	}
	pb, pc, pt, pcites, err := read(priorID)
	if err != nil {
		return err
	}
	if nb != pb || nc != pc || nt != pt || len(ncites) != len(pcites) {
		return nil
	}
	for i := range ncites {
		if ncites[i].URL != pcites[i].URL || ncites[i].Locator != pcites[i].Locator {
			return nil
		}
		if !strings.Contains(collapseWS(ncites[i].Quote), collapseWS(pcites[i].Quote)) {
			return nil
		}
	}
	_, err = tx.Exec(ctx, `
		UPDATE statements s
		   SET last_reviewed_at = $2, reviewed_by = $3, reviewed_hash = h.hash
		  FROM statement_review_hash h
		 WHERE s.id = $1 AND h.statement_id = s.id`, stmtID, reviewedAt, reviewedBy)
	return err
}

// collapseWS is the whitespace tolerance of the verbatim check: any run of
// whitespace reads as one space. Two extractors of one page differ in line
// breaks, never in words.
func collapseWS(s string) string { return strings.Join(strings.Fields(s), " ") }

// stampIfWritten stamps a freshly saved statement as reviewed by the person
// saving it, when they wrote it: the statement is new to its key, or its
// content differs from every earlier row with that key. A statement they
// carried forward unchanged keeps whatever stamp it inherited, which may be
// none — saving a page is not reading every statement on it. A statement
// with an undecided queue item is never stamped.
func stampIfWritten(ctx context.Context, tx pgx.Tx, stmtID int64, by string) error {
	_, err := tx.Exec(ctx, `
		UPDATE statements s
		   SET last_reviewed_at = NOW(), reviewed_by = $2, reviewed_hash = h.hash
		  FROM statement_review_hash h
		 WHERE s.id = $1 AND h.statement_id = s.id
		   AND NOT (s.last_reviewed_at IS NOT NULL AND s.reviewed_hash = h.hash)
		   AND NOT EXISTS (
		       SELECT 1 FROM statements p JOIN statement_review_hash ph ON ph.statement_id = p.id
		        WHERE p.key = s.key AND p.id <> s.id AND ph.hash = h.hash)
		   AND NOT `+undecidedSQL, stmtID, by)
	return err
}

// stampUnreviewed stamps every statement on a page that lacks a valid stamp,
// for the two page-level sign-offs: publishing a directory page, and
// ingesting a page straight to published (ADR-018 D3). Undecided items are
// left alone; the gate has already refused a page that carries one.
func stampUnreviewed(ctx context.Context, tx pgx.Tx, playbookID int64, by string) error {
	_, err := tx.Exec(ctx, `
		UPDATE statements s
		   SET last_reviewed_at = NOW(), reviewed_by = $2, reviewed_hash = h.hash
		  FROM playbook_statements ps, statement_review_hash h
		 WHERE ps.playbook_id = $1 AND ps.statement_id = s.id AND h.statement_id = s.id
		   AND NOT (s.last_reviewed_at IS NOT NULL AND s.reviewed_hash = h.hash)
		   AND NOT `+undecidedSQL, playbookID, by)
	if err != nil {
		return fmt.Errorf("stamp page %d reviewed: %w", playbookID, err)
	}
	return nil
}

// ReviewStamp is the outcome of a mark-reviewed action.
type ReviewStamp struct {
	// Stamped counts statements stamped by this call.
	Stamped int
	// Undecided counts statements skipped because a queue item is pending
	// against them; the reviewer decides it first.
	Undecided int
}

// MarkStatementsReviewed stamps the named statements on one page as reviewed
// by a person (ADR-018 D2). keys nil means every statement on the page that
// lacks a valid stamp; a named key is re-stamped even if already reviewed,
// since naming it is a deliberate second look. The caller has rendered the
// statements with their citations in front of the reviewer; the stamp
// records that content by hash.
func (pg *PG) MarkStatementsReviewed(ctx context.Context, playbookID int64, keys []string, by string) (ReviewStamp, error) {
	var out ReviewStamp
	if !isReviewer(by) {
		return out, fmt.Errorf("%q cannot mark statements reviewed: review is a person's sign-off", by)
	}
	for i, k := range keys {
		keys[i] = strings.ToLower(strings.TrimSpace(k))
		if !uuidRE.MatchString(keys[i]) {
			return out, fmt.Errorf("%q is not a statement key", k)
		}
	}
	err := pgx.BeginTxFunc(ctx, pg.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			SELECT count(*) FROM playbook_statements ps
			JOIN statements s ON s.id = ps.statement_id
			WHERE ps.playbook_id = $1 AND ($2::uuid[] IS NULL OR s.key = ANY($2::uuid[]))
			  AND `+undecidedSQL, playbookID, nullableKeys(keys)).Scan(&out.Undecided); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `
			UPDATE statements s
			   SET last_reviewed_at = NOW(), reviewed_by = $3, reviewed_hash = h.hash
			  FROM playbook_statements ps, statement_review_hash h
			 WHERE ps.playbook_id = $1 AND ps.statement_id = s.id AND h.statement_id = s.id
			   AND ($2::uuid[] IS NULL OR s.key = ANY($2::uuid[]))
			   AND ($2::uuid[] IS NOT NULL OR NOT (s.last_reviewed_at IS NOT NULL AND s.reviewed_hash = h.hash))
			   AND NOT `+undecidedSQL, playbookID, nullableKeys(keys), by)
		if err != nil {
			return err
		}
		out.Stamped = int(tag.RowsAffected())
		return nil
	})
	return out, err
}

// nullableKeys turns an empty key list into SQL NULL, which the queries above
// read as "every statement on the page".
func nullableKeys(keys []string) any {
	if len(keys) == 0 {
		return nil
	}
	return keys
}

// ReviewCount is a page's review standing for the dashboard: how many of its
// statements carry a valid stamp.
type ReviewCount struct {
	Reviewed, Total int
	// Undecided counts statements with a pending queue item.
	Undecided int
}

// AuthorDraftReviewCounts returns review standing for every draft.
func (pg *PG) AuthorDraftReviewCounts(ctx context.Context) (map[int64]ReviewCount, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT pb.id,
		       count(*) FILTER (WHERE s.last_reviewed_at IS NOT NULL AND s.reviewed_hash = h.hash),
		       count(*),
		       count(*) FILTER (WHERE `+undecidedSQL+`)
		FROM playbooks pb
		JOIN playbook_statements ps ON ps.playbook_id = pb.id
		JOIN statements s ON s.id = ps.statement_id
		JOIN statement_review_hash h ON h.statement_id = s.id
		WHERE pb.status = 'draft'
		GROUP BY pb.id`)
	if err != nil {
		return nil, fmt.Errorf("count reviewed statements: %w", err)
	}
	defer rows.Close()
	out := map[int64]ReviewCount{}
	for rows.Next() {
		var id int64
		var c ReviewCount
		if err := rows.Scan(&id, &c.Reviewed, &c.Total, &c.Undecided); err != nil {
			return nil, err
		}
		out[id] = c
	}
	return out, rows.Err()
}
