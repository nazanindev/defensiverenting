package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Source proposals (ADR-014 D7). A source no page cites is filed as a
// proposal to delete it. It waits in the same review queue as statement
// proposals until a person approves the deletion, rejects it with a note
// (the record of why the row is kept), or snoozes it.

// SourceProposal is one unused-source proposal as the queue lists it.
type SourceProposal struct {
	ID int64
	// SourceID is nil once the source has been deleted.
	SourceID     *int64
	URL          string
	Publisher    string
	Kind         string
	Reason       string
	ProposedBy   string
	Status       string
	DecidedBy    string
	DecidedAt    *time.Time
	DecisionNote string
	SnoozedUntil *time.Time
	CreatedAt    time.Time
	// InUse says a page cites the source again since the proposal was filed.
	// Approving it would be refused; the reviewer rejects it instead.
	InUse bool
	// OrphanCitations counts citations from statements no page carries any
	// more. They go with the source when it is deleted.
	OrphanCitations int
}

// ErrSourceInUse reports that a page cites the source after all, so the
// deletion an approval would perform is refused.
var ErrSourceInUse = errors.New("a page cites this source now; there is nothing to delete")

// unusedSourceSQL is the condition ListUnusedSources applies: no citation
// from any statement still linked to a page, and not the site's own
// editorial source, which is permanent plumbing.
const unusedSourceSQL = `
	s.url <> '/editorial'
	AND NOT EXISTS (
		SELECT 1 FROM citations c
		JOIN playbook_statements ps ON ps.statement_id = c.statement_id
		WHERE c.source_id = s.id)`

// FileUnusedSourceProposals files an unused-source proposal for every source
// no page cites that has no proposal waiting, snoozed, or rejected. A
// rejection means a person chose to keep the row, and an unused source stays
// unused, so it is not asked about again. It returns how many were filed.
func (pg *PG) FileUnusedSourceProposals(ctx context.Context, by string) (int, error) {
	if by == "" {
		return 0, errors.New("proposed_by is required")
	}
	tag, err := pg.pool.Exec(ctx, `
		INSERT INTO source_proposals (source_id, url, publisher, kind, reason, proposed_by)
		SELECT s.id, s.url, s.publisher, s.kind, 'unused-source', $1
		FROM sources s
		WHERE `+unusedSourceSQL+`
		  AND NOT EXISTS (
			SELECT 1 FROM source_proposals sp
			WHERE sp.source_id = s.id AND sp.status IN ('pending', 'snoozed', 'rejected'))
		ORDER BY s.publisher, s.id`, by)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

const sourceProposalSQL = `
	SELECT sp.id, sp.source_id, sp.url, sp.publisher, sp.kind, sp.reason, sp.proposed_by,
	       sp.status, sp.decided_by, sp.decided_at, sp.decision_note, sp.snoozed_until, sp.created_at,
	       EXISTS (SELECT 1 FROM citations c JOIN playbook_statements ps ON ps.statement_id = c.statement_id
	               WHERE c.source_id = sp.source_id),
	       (SELECT count(*) FROM citations c WHERE c.source_id = sp.source_id)
	FROM source_proposals sp`

func scanSourceProposal(row pgx.Row) (SourceProposal, error) {
	var p SourceProposal
	err := row.Scan(&p.ID, &p.SourceID, &p.URL, &p.Publisher, &p.Kind, &p.Reason, &p.ProposedBy,
		&p.Status, &p.DecidedBy, &p.DecidedAt, &p.DecisionNote, &p.SnoozedUntil, &p.CreatedAt,
		&p.InUse, &p.OrphanCitations)
	return p, err
}

// ListSourceProposals returns source proposals with the given status, by
// publisher then oldest first. Like ListProposals, "pending" includes snoozed
// ones whose date has come.
func (pg *PG) ListSourceProposals(ctx context.Context, status string) ([]SourceProposal, error) {
	cond, args := `sp.status = $1`, []any{status}
	if status == "pending" {
		cond, args = `(sp.status = 'pending' OR (sp.status = 'snoozed' AND sp.snoozed_until <= NOW()))`, nil
	}
	rows, err := pg.pool.Query(ctx, sourceProposalSQL+` WHERE `+cond+` ORDER BY sp.publisher, sp.url, sp.created_at, sp.id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SourceProposal
	for rows.Next() {
		p, err := scanSourceProposal(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (pg *PG) GetSourceProposal(ctx context.Context, id int64) (SourceProposal, error) {
	p, err := scanSourceProposal(pg.pool.QueryRow(ctx, sourceProposalSQL+` WHERE sp.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}

// CountPendingSourceProposals is the queue's share of "N proposals waiting"
// that concerns sources rather than statements.
func (pg *PG) CountPendingSourceProposals(ctx context.Context) (int, error) {
	var n int
	err := pg.pool.QueryRow(ctx, `
		SELECT count(*) FROM source_proposals
		WHERE status = 'pending' OR (status = 'snoozed' AND snoozed_until <= NOW())`).Scan(&n)
	return n, err
}

// DecideSourceProposal records a rejection or a snooze, neither of which
// touches the source.
func (pg *PG) DecideSourceProposal(ctx context.Context, id int64, status, by, note string, snoozedUntil *time.Time) error {
	switch status {
	case "rejected", "snoozed":
	default:
		return fmt.Errorf("decision %q is not rejected or snoozed; approval deletes the source and has its own call", status)
	}
	if status == "snoozed" && snoozedUntil == nil {
		return errors.New("snoozing needs a date to return on")
	}
	tag, err := pg.pool.Exec(ctx, `
		UPDATE source_proposals
		   SET status = $2, decided_by = $3, decided_at = NOW(), decision_note = $4, snoozed_until = $5
		 WHERE id = $1 AND status IN ('pending', 'snoozed')`, id, status, by, note, snoozedUntil)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrProposalNotPending
	}
	return nil
}

// ApproveSourceProposal deletes the source the proposal names and marks the
// proposal approved, in one transaction. The source must still be unused at
// that moment: a page that came to cite it while the proposal waited makes
// the approval fail with ErrSourceInUse and nothing is written. Citations
// left on orphaned statements (ones no page carries) are removed with it,
// and any discovery candidate that was approved into this source is
// unlinked rather than deleted, since the candidate is its own record.
func (pg *PG) ApproveSourceProposal(ctx context.Context, id int64, by string) error {
	return pgx.BeginTxFunc(ctx, pg.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var status string
		var sourceID *int64
		err := tx.QueryRow(ctx, `SELECT status, source_id FROM source_proposals WHERE id = $1 FOR UPDATE`, id).Scan(&status, &sourceID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if status != "pending" && status != "snoozed" {
			return ErrProposalNotPending
		}
		if sourceID == nil {
			return errors.New("the source is already gone")
		}
		var unused bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM sources s WHERE s.id = $1 AND `+unusedSourceSQL+`)`, *sourceID).Scan(&unused); err != nil {
			return err
		}
		if !unused {
			return ErrSourceInUse
		}
		if _, err := tx.Exec(ctx, `DELETE FROM citations WHERE source_id = $1`, *sourceID); err != nil {
			return fmt.Errorf("remove orphan citations: %w", err)
		}
		if _, err := tx.Exec(ctx, `UPDATE source_candidates SET source_id = NULL WHERE source_id = $1`, *sourceID); err != nil {
			return fmt.Errorf("unlink candidates: %w", err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM sources WHERE id = $1`, *sourceID); err != nil {
			return fmt.Errorf("delete source: %w", err)
		}
		_, err = tx.Exec(ctx, `
			UPDATE source_proposals
			   SET status = 'approved', decided_by = $2, decided_at = NOW(), snoozed_until = NULL
			 WHERE id = $1`, id, by)
		return err
	})
}
