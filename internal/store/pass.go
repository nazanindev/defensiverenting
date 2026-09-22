package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ReasonAgentPass is the reason code of a PASS (ADR-022): the review agent
// read a statement against its cited source and found the passage that
// supports the claim as written. The row is filed already decided, under the
// agent's name, so the audit lists it like every other agent decision.
const ReasonAgentPass = "agent-pass:pass"

// PassEvidence is what a PASS puts on record: the source read, the passage
// that supports the claim, verbatim in that source as fetched, and why.
type PassEvidence struct {
	SourceURL string `json:"source_url"`
	Passage   string `json:"passage"`
	Reason    string `json:"reason"`
}

// ErrUndecidedItem is PassStatement's refusal while a queue item is open on
// the statement: the agent decides the item first, or a person does.
var ErrUndecidedItem = errors.New("the statement has an undecided item in the queue")

// PassStatement records a PASS and stamps the statement reviewed under the
// review agent's name (ADR-022). The stamp is the ordinary review stamp,
// bound to the content hash, so an edit clears it the way it clears a
// person's; reviewed_by says who. The caller has confirmed the passage at
// the live source; this only refuses a statement that is not on the page or
// has an open queue item.
func (pg *PG) PassStatement(ctx context.Context, playbookID int64, key string, ev PassEvidence) error {
	key = strings.ToLower(strings.TrimSpace(key))
	if !uuidRE.MatchString(key) {
		return fmt.Errorf("%q is not a statement key", key)
	}
	if strings.TrimSpace(ev.Passage) == "" || strings.TrimSpace(ev.SourceURL) == "" || strings.TrimSpace(ev.Reason) == "" {
		return fmt.Errorf("a PASS needs the source, the passage, and the reason")
	}
	evidence, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	return pgx.BeginTxFunc(ctx, pg.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var stmtID int64
		err := tx.QueryRow(ctx, `
			SELECT s.id FROM statements s JOIN playbook_statements ps ON ps.statement_id = s.id
			WHERE ps.playbook_id = $1 AND s.key = $2::uuid FOR UPDATE OF s`, playbookID, key).Scan(&stmtID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		var undecided bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM statement_proposals sp WHERE sp.statement_key = $1::uuid AND sp.status = 'pending')`, key).Scan(&undecided); err != nil {
			return err
		}
		if undecided {
			return ErrUndecidedItem
		}
		note := "Review agent, PASS: " + strings.TrimSpace(ev.Reason)
		if _, err := tx.Exec(ctx, `
			INSERT INTO statement_proposals (statement_key, playbook_id, reason, proposed, evidence, proposed_by, status, decided_by, decided_at, decision_note)
			VALUES ($1::uuid, $2, $3, NULL, $4, $5, 'approved', $5, NOW(), $6)`,
			key, playbookID, ReasonAgentPass, evidence, ActorReviewAgent, note); err != nil {
			return fmt.Errorf("record pass: %w", err)
		}
		tag, err := tx.Exec(ctx, `
			UPDATE statements s
			   SET last_reviewed_at = NOW(), reviewed_by = $2, reviewed_hash = h.hash
			  FROM statement_review_hash h
			 WHERE s.id = $1 AND h.statement_id = s.id`, stmtID, ActorReviewAgent)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return fmt.Errorf("stamp statement %d: %d rows", stmtID, tag.RowsAffected())
		}
		return nil
	})
}

// FlagStatement files a person's doubt about a statement as a reviewer note
// (ADR-024): the reader read the page and something looked wrong, so the
// triage agent takes it from there. The note carries the person's name, so
// the loop can tell a person's flag from an agent's.
//
// When the statement currently carries a review agent's stamp, the note
// records that it overturns it. That is the measure ADR-021 named and had
// no way to collect: a rule whose passes a person keeps flagging is a rule
// that has to narrow.
func (pg *PG) FlagStatement(ctx context.Context, playbookID int64, key, note, by string) error {
	key = strings.ToLower(strings.TrimSpace(key))
	if !uuidRE.MatchString(key) {
		return fmt.Errorf("%q is not a statement key", key)
	}
	if strings.TrimSpace(note) == "" {
		return fmt.Errorf("a flag needs a reason: what looks wrong")
	}
	if !isReviewer(by) {
		return fmt.Errorf("%q cannot flag a statement: a flag is a person's doubt", by)
	}
	return pgx.BeginTxFunc(ctx, pg.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var stampedBy string
		err := tx.QueryRow(ctx, `
			SELECT COALESCE(`+reviewedBySQL+`, '')
			FROM statements s JOIN playbook_statements ps ON ps.statement_id = s.id
			JOIN statement_review_hash h ON h.statement_id = s.id
			WHERE ps.playbook_id = $1 AND s.key = $2::uuid`, playbookID, key).Scan(&stampedBy)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		ev := ReviewerFlagEvidence{Note: strings.TrimSpace(note)}
		if stampedBy != "" && !isReviewer(stampedBy) {
			ev.Overturned = stampedBy
		}
		evidence, err := json.Marshal(ev)
		if err != nil {
			return err
		}
		var dup bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM statement_proposals
				WHERE statement_key = $1::uuid AND reason = $2 AND status = 'pending' AND evidence = $3::jsonb)`,
			key, ReasonReviewerFlag, evidence).Scan(&dup); err != nil {
			return err
		}
		if dup {
			return nil
		}
		_, err = insertProposal(ctx, tx, key, playbookID, ReasonReviewerFlag, nil, evidence, by)
		return err
	})
}

// FileReaderNote files what a reader left as a reviewer note on the
// statement, under the review agent's name (ADR-022): the reason a PASS or
// an apply was refused becomes a work item the triage agent proposes a fix
// for, instead of a line in a file. An identical note already on the key
// is not filed twice.
func (pg *PG) FileReaderNote(ctx context.Context, playbookID int64, key, note string) error {
	key = strings.ToLower(strings.TrimSpace(key))
	if !uuidRE.MatchString(key) {
		return fmt.Errorf("%q is not a statement key", key)
	}
	return pgx.BeginTxFunc(ctx, pg.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return fileReviewerFlag(ctx, tx, key, playbookID, note, ActorReviewAgent)
	})
}

// ReasonSourceMoved is the note a checker run files when the law around a
// quote changed although the quote itself survived, or when a claim's date
// is about to pass (ADR-024). It is a reviewer note like any other: the
// triage agent proposes the fix, a judge applies it, a reader re-reads.
const NoteSourceMoved = "The text around this quote changed at the source since it was last checked. Read the statement against the section as it reads now."

// FileCheckerNote files a note on a statement from the automated checker,
// under the source check's name. Deduplicated on the note text, so a run
// that sees the same change again does not pile up items.
func (pg *PG) FileCheckerNote(ctx context.Context, key, note string) error {
	key = strings.ToLower(strings.TrimSpace(key))
	if !uuidRE.MatchString(key) || strings.TrimSpace(note) == "" {
		return fmt.Errorf("a checker note needs a statement key and a reason")
	}
	return pgx.BeginTxFunc(ctx, pg.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var playbookID int64
		err := tx.QueryRow(ctx, `
			SELECT ps.playbook_id FROM playbook_statements ps JOIN statements s ON s.id = ps.statement_id
			JOIN playbooks pb ON pb.id = ps.playbook_id
			WHERE s.key = $1::uuid AND pb.status IN ('draft','published')
			ORDER BY (pb.status = 'draft') DESC LIMIT 1`, key).Scan(&playbookID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // the statement is not on a live or draft page
		}
		if err != nil {
			return err
		}
		return fileReviewerFlag(ctx, tx, key, playbookID, note, ActorSourceCheck)
	})
}

// StaleStatement is a statement whose claim depends on a date that is about
// to pass, for the checker to file a note about.
type StaleStatement struct {
	Key        string
	BodyMD     string
	StaleAfter time.Time
}

// StatementsGoingStale lists statements whose stale_after falls within the
// window, on a live or draft page. The window is deliberately ahead of the
// date: the point is to have the fix in the queue while the page is still
// right (ADR-024).
func (pg *PG) StatementsGoingStale(ctx context.Context, within time.Duration) ([]StaleStatement, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT DISTINCT s.key::text, s.body_md, s.stale_after
		FROM statements s JOIN playbook_statements ps ON ps.statement_id = s.id
		JOIN playbooks pb ON pb.id = ps.playbook_id
		WHERE s.stale_after IS NOT NULL AND s.stale_after <= (CURRENT_DATE + $1::int)
		  AND pb.status IN ('draft','published')
		ORDER BY s.stale_after`, int(within.Hours()/24))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StaleStatement
	for rows.Next() {
		var st StaleStatement
		if err := rows.Scan(&st.Key, &st.BodyMD, &st.StaleAfter); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}
