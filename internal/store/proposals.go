package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Statement proposals (ADR-014). A proposal is a whole replacement statement
// filed against a claim's durable key. It waits in the review queue until a
// person approves it (an ordinary save, D3), rejects it with a note, or
// snoozes it. A newer proposal against the same key supersedes the older.

// ProposedCitation is one citation of a proposed statement, keyed by URL so
// a proposal can be filed before the source row exists. It doubles as the
// shape the queue shows the current citations in, so old and new line up.
type ProposedCitation struct {
	URL       string `json:"url"`
	Publisher string `json:"publisher,omitempty"`
	Kind      string `json:"kind,omitempty"`
	Locator   string `json:"locator,omitempty"`
	Quote     string `json:"quote,omitempty"`
	// Checked says the proposer confirmed the quote verbatim in the live
	// page when it filed. Approval re-checks every quote live and, only when
	// the source cannot be read from the authoring server, stamps checked_at
	// on the proposer's word. A quote matched against a snapshot is never
	// Checked.
	Checked bool `json:"checked,omitempty"`
	// CheckedVia says how the proposer obtained the text ("direct fetch,
	// html", "headless render"), for the reviewer and for the stamp that
	// approval writes on the proposer's word.
	CheckedVia string `json:"checked_via,omitempty"`
	// Editorial marks the site's own guidance citation (ADR-003), which has
	// no URL, quote, or locator.
	Editorial bool `json:"editorial,omitempty"`
}

// ProposedStatement is the statement as it should read after approval.
type ProposedStatement struct {
	BodyMD    string             `json:"body_md"`
	Concept   string             `json:"concept,omitempty"`
	TopicRef  string             `json:"topic_ref,omitempty"`
	Citations []ProposedCitation `json:"citations"`
}

type Proposal struct {
	ID           int64
	StatementKey string
	PlaybookID   int64
	Reason       string
	// Proposed is nil for a work item: the proposer found a problem and has
	// no replacement to offer (D4).
	Proposed     *ProposedStatement
	Evidence     json.RawMessage
	ProposedBy   string
	Status       string
	DecidedBy    string
	DecidedAt    *time.Time
	DecisionNote string
	SnoozedUntil *time.Time
	CreatedAt    time.Time
}

// ProposalRow is a proposal as the queue lists it: with the page an approval
// would edit and the statement as it currently reads there.
type ProposalRow struct {
	Proposal
	// Target is the page an approval edits: the draft revision in the slot
	// when there is one, else the page the proposal was filed against.
	TargetPlaybookID int64
	TargetStatus     string
	Title            string
	JurisdictionName string
	TopicName        string
	Language         string
	// Position is the statement's 1-based place on the target page, or 0
	// when no statement with this key is on it any more.
	Position         int
	CurrentBody      string
	CurrentCitations []ProposedCitation
}

// OnPage reports whether the target page still carries the statement.
func (r ProposalRow) OnPage() bool { return r.Position > 0 }

type FileProposalParams struct {
	StatementKey string
	// PlaybookID is the page the proposer observed the statement on. Zero
	// resolves it from the key, preferring a draft.
	PlaybookID int64
	Reason     string
	Proposed   *ProposedStatement
	Evidence   json.RawMessage
	ProposedBy string
}

type ApproveProposalParams struct {
	ID int64
	By string
	// Statement is the replacement, already resolved to source rows and
	// with its quotes checked, because those steps need the fetcher and the
	// reference-only rule that live with the caller. Key and Language are
	// set here from the proposal and the page.
	Statement IngestStatementParams
}

var (
	ErrProposalNotPending = errors.New("proposal is not pending")
	// ErrProposalTargetGone reports that no statement with the proposal's key
	// is on the page an approval would edit: it was deleted, or a revision
	// dropped it. There is nothing to replace.
	ErrProposalTargetGone = errors.New("the statement this proposal replaces is no longer on the page")
)

var reasonRE = regexp.MustCompile(`^(source-drift|agent-pass:[^\s:]+|source-quality:[a-z][a-z0-9-]*)$`)

// ValidReason reports whether a reason code is in the closed vocabulary (D4).
func ValidReason(r string) bool { return reasonRE.MatchString(r) }

// FileProposal records a proposal and supersedes any pending or snoozed one
// against the same key, so the queue never shows two competing edits for one
// claim. It returns the new proposal's id.
func (pg *PG) FileProposal(ctx context.Context, p FileProposalParams) (int64, error) {
	key := strings.ToLower(strings.TrimSpace(p.StatementKey))
	if !uuidRE.MatchString(key) {
		return 0, fmt.Errorf("statement key %q is not a statement key", p.StatementKey)
	}
	if !ValidReason(p.Reason) {
		return 0, fmt.Errorf("reason %q is not one of source-drift, agent-pass:<name>, source-quality:<signal>", p.Reason)
	}
	if strings.TrimSpace(p.ProposedBy) == "" {
		return 0, errors.New("proposed_by is required")
	}
	if p.Proposed != nil && strings.TrimSpace(p.Proposed.BodyMD) == "" {
		return 0, errors.New("a proposed statement needs a body; omit proposed entirely for a work item")
	}
	var proposed []byte
	if p.Proposed != nil {
		var err error
		if proposed, err = json.Marshal(p.Proposed); err != nil {
			return 0, err
		}
	}
	evidence := p.Evidence
	if len(evidence) == 0 {
		evidence = json.RawMessage(`{}`)
	}
	var id int64
	err := pgx.BeginTxFunc(ctx, pg.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		playbookID := p.PlaybookID
		if playbookID == 0 {
			err := tx.QueryRow(ctx, `
				SELECT pb.id FROM playbook_statements ps
				JOIN statements s ON s.id = ps.statement_id
				JOIN playbooks pb ON pb.id = ps.playbook_id
				WHERE s.key = $1::uuid AND pb.status IN ('draft', 'published')
				ORDER BY (pb.status = 'draft') DESC LIMIT 1`, key).Scan(&playbookID)
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("no page carries a statement with key %s", key)
			}
			if err != nil {
				return err
			}
		}
		var err error
		id, err = insertProposal(ctx, tx, key, playbookID, p.Reason, proposed, evidence, p.ProposedBy)
		return err
	})
	return id, err
}

// insertProposal writes one proposal inside tx, superseding any pending or
// snoozed one against the same key. Shared by FileProposal and the save path
// that files reviewer notes (ADR-018 D1).
func insertProposal(ctx context.Context, tx pgx.Tx, key string, playbookID int64, reason string, proposed []byte, evidence json.RawMessage, by string) (int64, error) {
	if _, err := tx.Exec(ctx, `
		UPDATE statement_proposals SET status = 'superseded'
		WHERE statement_key = $1::uuid AND status IN ('pending', 'snoozed')`, key); err != nil {
		return 0, fmt.Errorf("supersede: %w", err)
	}
	var id int64
	err := tx.QueryRow(ctx, `
		INSERT INTO statement_proposals (statement_key, playbook_id, reason, proposed, evidence, proposed_by)
		VALUES ($1::uuid, $2, $3, $4, $5, $6) RETURNING id`,
		key, playbookID, reason, proposed, evidence, by).Scan(&id)
	return id, err
}

// ReasonReviewerFlag is the reason code of a work item filed from a
// statement's reviewer note at save time (ADR-018 D1). The evidence is
// {"note": "..."}: the proposer's doubt, in their words.
const ReasonReviewerFlag = "agent-pass:flag"

// ReviewerFlagEvidence is the evidence shape behind ReasonReviewerFlag.
type ReviewerFlagEvidence struct {
	Note string `json:"note"`
}

// fileReviewerFlag files a statement's reviewer note as a work-item proposal
// inside the saving transaction, so the doubt and the claim land together or
// not at all. A note already on file for this key, in any status, is not
// filed again: a save that carries the same note forward must not reopen a
// doubt a reviewer has already decided, and must not stack duplicates while
// one is pending.
func fileReviewerFlag(ctx context.Context, tx pgx.Tx, key string, playbookID int64, note, by string) error {
	note = strings.TrimSpace(note)
	if note == "" {
		return nil
	}
	evidence, err := json.Marshal(ReviewerFlagEvidence{Note: note})
	if err != nil {
		return err
	}
	var dup bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM statement_proposals
			WHERE statement_key = $1::uuid AND reason = $2 AND evidence = $3::jsonb)`,
		key, ReasonReviewerFlag, evidence).Scan(&dup); err != nil {
		return fmt.Errorf("look for an existing note: %w", err)
	}
	if dup {
		return nil
	}
	_, err = insertProposal(ctx, tx, key, playbookID, ReasonReviewerFlag, nil, evidence, by)
	return err
}

// proposalRowSQL joins each proposal to the page an approval would edit and
// to the statement as it reads there now. The target is the draft in the
// observed page's slot when one exists (a revision is where that slot's next
// version is being assembled, D3), else the observed page itself.
const proposalRowSQL = `
	SELECT p.id, p.statement_key::text, p.playbook_id, p.reason, p.proposed, p.evidence, p.proposed_by,
	       p.status, p.decided_by, p.decided_at, p.decision_note, p.snoozed_until, p.created_at,
	       tgt.id, tgt.status, tgt.title, j.name, t.name, tgt.language,
	       COALESCE(cur.position, -1), COALESCE(cur.body_md, ''), COALESCE(cur.citations, '[]'::json)
	FROM statement_proposals p
	JOIN playbooks obs ON obs.id = p.playbook_id
	JOIN LATERAL (
		SELECT pb.* FROM playbooks pb
		WHERE pb.jurisdiction_id = obs.jurisdiction_id AND pb.topic_id = obs.topic_id
		  AND pb.language = obs.language AND pb.status IN ('draft', 'published')
		ORDER BY (pb.status = 'draft') DESC LIMIT 1
	) tgt ON true
	JOIN jurisdictions j ON j.id = tgt.jurisdiction_id
	JOIN topics t ON t.id = tgt.topic_id
	LEFT JOIN LATERAL (
		SELECT ps.position, s.body_md,
		       (SELECT json_agg(json_build_object(
		            'url', src.url, 'publisher', src.publisher, 'kind', src.kind,
		            'locator', c.locator, 'quote', c.quote,
		            'checked', c.checked_at IS NOT NULL OR c.manually_verified,
		            'editorial', src.kind = 'editorial') ORDER BY c.source_id)
		          FROM citations c JOIN sources src ON src.id = c.source_id
		         WHERE c.statement_id = s.id) AS citations
		FROM playbook_statements ps JOIN statements s ON s.id = ps.statement_id
		WHERE ps.playbook_id = tgt.id AND s.key = p.statement_key
		LIMIT 1
	) cur ON true`

func scanProposalRow(row pgx.Row) (ProposalRow, error) {
	var r ProposalRow
	var proposed, evidence, citations []byte
	var position int
	err := row.Scan(
		&r.ID, &r.StatementKey, &r.PlaybookID, &r.Reason, &proposed, &evidence, &r.ProposedBy,
		&r.Status, &r.DecidedBy, &r.DecidedAt, &r.DecisionNote, &r.SnoozedUntil, &r.CreatedAt,
		&r.TargetPlaybookID, &r.TargetStatus, &r.Title, &r.JurisdictionName, &r.TopicName, &r.Language,
		&position, &r.CurrentBody, &citations,
	)
	if err != nil {
		return r, err
	}
	if len(proposed) > 0 {
		r.Proposed = &ProposedStatement{}
		if err := json.Unmarshal(proposed, r.Proposed); err != nil {
			return r, fmt.Errorf("proposal %d: proposed: %w", r.ID, err)
		}
	}
	r.Evidence = json.RawMessage(evidence)
	if err := json.Unmarshal(citations, &r.CurrentCitations); err != nil {
		return r, fmt.Errorf("proposal %d: current citations: %w", r.ID, err)
	}
	r.Position = position + 1
	return r, nil
}

// ListProposals returns proposals with the given status, in reading order:
// by page, then by the statement's position on it, then oldest first. The
// "pending" listing also includes snoozed proposals whose date has come.
func (pg *PG) ListProposals(ctx context.Context, status string) ([]ProposalRow, error) {
	// Only targets in an active content language are listed (ADR-015 D3):
	// a proposal against a parked translation has no reviewer.
	cond, args := `p.status = $2`, []any{ContentLanguages, status}
	if status == "pending" {
		cond, args = `(p.status = 'pending' OR (p.status = 'snoozed' AND p.snoozed_until <= NOW()))`, []any{ContentLanguages}
	}
	rows, err := pg.pool.Query(ctx, proposalRowSQL+` WHERE tgt.language = ANY($1) AND `+cond+`
		ORDER BY j.name, t.name, tgt.language, COALESCE(cur.position, 1e9), p.created_at, p.id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProposalRow
	for rows.Next() {
		r, err := scanProposalRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (pg *PG) GetProposal(ctx context.Context, id int64) (ProposalRow, error) {
	r, err := scanProposalRow(pg.pool.QueryRow(ctx, proposalRowSQL+` WHERE p.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrNotFound
	}
	return r, err
}

// CountPendingProposals is the dashboard's "N proposals waiting".
func (pg *PG) CountPendingProposals(ctx context.Context) (int, error) {
	var n int
	err := pg.pool.QueryRow(ctx, `
		SELECT count(*) FROM statement_proposals p
		JOIN playbooks obs ON obs.id = p.playbook_id
		WHERE obs.language = ANY($1)
		  AND (p.status = 'pending' OR (p.status = 'snoozed' AND p.snoozed_until <= NOW()))`, ContentLanguages).Scan(&n)
	return n, err
}

// DecideProposal records a decision that changes no page: rejected (with a
// note saying why, which is the record of why a source was judged fine),
// snoozed until a date, or approved for a work item that carried no
// replacement and was resolved in the editor.
func (pg *PG) DecideProposal(ctx context.Context, id int64, status, by, note string, snoozedUntil *time.Time) error {
	switch status {
	case "rejected", "snoozed", "approved":
	default:
		return fmt.Errorf("decision %q is not rejected, snoozed, or approved", status)
	}
	if status == "snoozed" && snoozedUntil == nil {
		return errors.New("snoozing needs a date to return on")
	}
	tag, err := pg.pool.Exec(ctx, `
		UPDATE statement_proposals
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

// ApproveProposal applies the replacement statement to the target page
// through the ordinary author save (D3) and marks the proposal approved, in
// one transaction. On a live page the save runs the publish gate; a refusal
// comes back as a NotPublishableError with nothing written, and the proposal
// stays pending.
func (pg *PG) ApproveProposal(ctx context.Context, p ApproveProposalParams) error {
	return pgx.BeginTxFunc(ctx, pg.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var key, status string
		var targetID int64
		err := tx.QueryRow(ctx, `
			SELECT p.statement_key::text, p.status, tgt.id
			FROM statement_proposals p
			JOIN playbooks obs ON obs.id = p.playbook_id
			JOIN LATERAL (
				SELECT pb.id FROM playbooks pb
				WHERE pb.jurisdiction_id = obs.jurisdiction_id AND pb.topic_id = obs.topic_id
				  AND pb.language = obs.language AND pb.status IN ('draft', 'published')
				ORDER BY (pb.status = 'draft') DESC LIMIT 1
			) tgt ON true
			WHERE p.id = $1 FOR UPDATE OF p`, p.ID).Scan(&key, &status, &targetID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if status != "pending" && status != "snoozed" {
			return ErrProposalNotPending
		}

		var params AuthorUpdatePlaybookParams
		params.ID = targetID
		params.UpdatedBy = p.By
		if err := tx.QueryRow(ctx, `
			SELECT jurisdiction_id, topic_id, language, slug, title, intro_md, page_kind, author_notes
			FROM playbooks WHERE id = $1`, targetID,
		).Scan(&params.JurisdictionID, &params.TopicID, &params.Language, &params.Slug,
			&params.Title, &params.IntroMD, &params.PageKind, &params.AuthorNotes); err != nil {
			return fmt.Errorf("read target page: %w", err)
		}
		current, err := statementParams(ctx, tx, targetID)
		if err != nil {
			return err
		}
		replaced := false
		for i := range current {
			if current[i].Key == key {
				st := p.Statement
				st.Key = key
				st.Language = params.Language
				current[i] = st
				replaced = true
				break
			}
		}
		if !replaced {
			return ErrProposalTargetGone
		}
		params.Statements = current
		if err := authorUpdatePlaybookTx(ctx, tx, params); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			UPDATE statement_proposals
			   SET status = 'approved', decided_by = $2, decided_at = NOW(), snoozed_until = NULL
			 WHERE id = $1`, p.ID, p.By)
		return err
	})
}

// statementParams reads a page's statements back in the shape the save
// takes, keys included, so one of them can be swapped and the page re-saved.
// Citations are carried with CheckedNow false: the save then inherits each
// quote's existing confirmation stamp rather than re-stamping it as checked
// today, which nobody did.
func statementParams(ctx context.Context, tx pgx.Tx, playbookID int64) ([]IngestStatementParams, error) {
	rows, err := tx.Query(ctx, `
		SELECT s.key::text, s.body_md, s.language, COALESCE(co.slug, ''), COALESCE(tr.slug, ''),
		       c.source_id, c.locator, c.quote, c.manually_verified
		FROM playbook_statements ps
		JOIN statements s ON s.id = ps.statement_id
		LEFT JOIN concepts co ON co.id = s.concept_id
		LEFT JOIN topics tr ON tr.id = s.topic_ref
		LEFT JOIN citations c ON c.statement_id = s.id
		WHERE ps.playbook_id = $1
		ORDER BY ps.position, c.source_id`, playbookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []IngestStatementParams
	for rows.Next() {
		var sp IngestStatementParams
		var sourceID *int64
		var locator, quote *string
		var manual *bool
		if err := rows.Scan(&sp.Key, &sp.BodyMD, &sp.Language, &sp.ConceptSlug, &sp.TopicRefSlug,
			&sourceID, &locator, &quote, &manual); err != nil {
			return nil, err
		}
		if len(out) == 0 || out[len(out)-1].Key != sp.Key {
			out = append(out, sp)
		}
		if sourceID != nil {
			last := &out[len(out)-1]
			last.Sources = append(last.Sources, IngestCitationParams{
				SourceID: *sourceID, Locator: deref(locator), Quote: deref(quote),
				ManuallyVerified: manual != nil && *manual,
			})
		}
	}
	return out, rows.Err()
}

// LanguageOfStatementKey returns the language of the page carrying this
// statement key (a draft is preferred when both exist), or ErrNotFound.
func (pg *PG) LanguageOfStatementKey(ctx context.Context, key string) (string, error) {
	key = strings.ToLower(strings.TrimSpace(key))
	if !uuidRE.MatchString(key) {
		return "", ErrNotFound
	}
	var lang string
	err := pg.pool.QueryRow(ctx, `
		SELECT pb.language FROM playbook_statements ps
		JOIN statements s ON s.id = ps.statement_id
		JOIN playbooks pb ON pb.id = ps.playbook_id
		WHERE s.key = $1::uuid AND pb.status IN ('draft', 'published')
		ORDER BY (pb.status = 'draft') DESC LIMIT 1`, key).Scan(&lang)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return lang, err
}

// StatementByKey returns the statement carrying this key (a draft is
// preferred over the live page, as everywhere in the queue) as a
// ProposedStatement, or ErrNotFound.
func (pg *PG) StatementByKey(ctx context.Context, key string) (ProposedStatement, error) {
	key = strings.ToLower(strings.TrimSpace(key))
	if !uuidRE.MatchString(key) {
		return ProposedStatement{}, ErrNotFound
	}
	var out ProposedStatement
	var citations []byte
	err := pg.pool.QueryRow(ctx, `
		SELECT s.body_md, COALESCE(co.slug, ''), COALESCE(tr.slug, ''),
		       COALESCE((SELECT json_agg(json_build_object(
		            'url', src.url, 'publisher', src.publisher, 'kind', src.kind,
		            'locator', c.locator, 'quote', c.quote,
		            'checked', c.checked_at IS NOT NULL OR c.manually_verified,
		            'editorial', src.kind = 'editorial') ORDER BY c.source_id)
		          FROM citations c JOIN sources src ON src.id = c.source_id
		         WHERE c.statement_id = s.id), '[]'::json)
		FROM playbook_statements ps
		JOIN statements s ON s.id = ps.statement_id
		JOIN playbooks pb ON pb.id = ps.playbook_id
		LEFT JOIN concepts co ON co.id = s.concept_id
		LEFT JOIN topics tr ON tr.id = s.topic_ref
		WHERE s.key = $1::uuid AND pb.status IN ('draft', 'published')
		ORDER BY (pb.status = 'draft') DESC LIMIT 1`, key,
	).Scan(&out.BodyMD, &out.Concept, &out.TopicRef, &citations)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrNotFound
	}
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(citations, &out.Citations); err != nil {
		return out, err
	}
	return out, nil
}

// DriftAlreadyFiled is true when a source-drift proposal for this key is
// pending or snoozed (a person will see it), or was rejected for the same
// missing quote (a person decided the source is fine as it is).
func (pg *PG) DriftAlreadyFiled(ctx context.Context, key, missingQuote string) (bool, error) {
	key = strings.ToLower(strings.TrimSpace(key))
	if !uuidRE.MatchString(key) {
		return false, nil
	}
	var exists bool
	err := pg.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM statement_proposals
			WHERE statement_key = $1::uuid AND reason = 'source-drift'
			  AND (status IN ('pending', 'snoozed')
			       OR (status = 'rejected' AND evidence->>'old_quote' = $2)))`, key, missingQuote).Scan(&exists)
	return exists, err
}
