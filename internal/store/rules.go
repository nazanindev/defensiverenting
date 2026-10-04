package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Rules pages, coverage records and gaps (ADR-028 D4, D5, D9).
//
// The concept tag is the hinge. A concept page is one concept across every
// place; a rules page is one place across every concept homed in a topic.
// Each statement is stored once and shows on both. This file answers the
// three questions a rules page needs: which concepts it answers, where each
// answer already lives, and which the law was searched for and not found.

// CoverageRecord is "we searched and found no law" for one place and concept
// (D5). It is a site fact about our search, never a legal claim.
type CoverageRecord struct {
	JurisdictionID   int64
	JurisdictionName string
	JurisdictionSlug string
	ConceptSlug      string
	SourcesChecked   []string
	Note             string
	CheckedBy        string
	CheckedAt        time.Time
}

// FileCoverageParams files or refreshes a coverage record.
type FileCoverageParams struct {
	JurisdictionSlug string
	ConceptSlug      string
	SourcesChecked   []string
	Note             string
	By               string
}

// ErrConceptAnswered refuses a coverage record for a place that already has
// a statement with the concept: a cited answer always wins over "no law
// found", and the two must never stand side by side.
var ErrConceptAnswered = errors.New("this place already has a statement with this concept")

// FileCoverageRecord records that the law was searched for and not found.
// Filing again refreshes the date and the sources, which is what the weekly
// re-check does. Invariants: at least one official place searched, a named
// person or agent, and no statement with the concept in that place.
func (pg *PG) FileCoverageRecord(ctx context.Context, p FileCoverageParams) error {
	var sources []string
	for _, s := range p.SourcesChecked {
		if s = strings.TrimSpace(s); s != "" {
			sources = append(sources, s)
		}
	}
	if len(sources) == 0 {
		return errors.New("a coverage record names at least one place that was searched")
	}
	if strings.TrimSpace(p.By) == "" {
		return errors.New("a coverage record names who searched")
	}
	return pgx.BeginTxFunc(ctx, pg.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var jID, cID int64
		var kind string
		var national bool
		err := tx.QueryRow(ctx, `
			SELECT j.id, j.kind, c.id, c.national_only
			FROM jurisdictions j, concepts c
			WHERE j.slug = $1 AND c.slug = $2`, p.JurisdictionSlug, p.ConceptSlug).Scan(&jID, &kind, &cID, &national)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("no place %q or no concept %q: %w", p.JurisdictionSlug, p.ConceptSlug, ErrNotFound)
		}
		if err != nil {
			return err
		}
		if national && kind != "country" {
			return fmt.Errorf("concept %q is answered nationally; a state needs no record for it", p.ConceptSlug)
		}
		var answered bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM statements s
				JOIN playbook_statements ps ON ps.statement_id = s.id
				JOIN playbooks pb ON pb.id = ps.playbook_id
				WHERE pb.jurisdiction_id = $1 AND s.concept_id = $2
				  AND pb.status IN ('published', 'draft') AND pb.language = 'en')`, jID, cID).Scan(&answered); err != nil {
			return err
		}
		if answered {
			return ErrConceptAnswered
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO coverage_records (jurisdiction_id, concept_id, sources_checked, note, checked_by)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (jurisdiction_id, concept_id) DO UPDATE
			   SET sources_checked = EXCLUDED.sources_checked, note = EXCLUDED.note,
			       checked_by = EXCLUDED.checked_by, checked_at = NOW()`,
			jID, cID, sources, strings.TrimSpace(p.Note), p.By)
		return err
	})
}

// coverageRecords lists records, filtered by place or concept (0 = any).
func (pg *PG) coverageRecords(ctx context.Context, jurisdictionID, conceptID int64) ([]CoverageRecord, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT j.id, j.name, j.slug, c.slug, r.sources_checked, r.note, r.checked_by, r.checked_at
		FROM coverage_records r
		JOIN jurisdictions j ON j.id = r.jurisdiction_id
		JOIN concepts c ON c.id = r.concept_id
		WHERE ($1 = 0 OR r.jurisdiction_id = $1) AND ($2 = 0 OR r.concept_id = $2)
		ORDER BY j.name, c.position, c.slug`, jurisdictionID, conceptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CoverageRecord
	for rows.Next() {
		var r CoverageRecord
		if err := rows.Scan(&r.JurisdictionID, &r.JurisdictionName, &r.JurisdictionSlug, &r.ConceptSlug,
			&r.SourcesChecked, &r.Note, &r.CheckedBy, &r.CheckedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListCoverageRecords returns every record, oldest check first, for the
// weekly check to list (D5).
func (pg *PG) ListCoverageRecords(ctx context.Context) ([]CoverageRecord, error) {
	return pg.coverageRecords(ctx, 0, 0)
}

// RulesAnswer is where one concept stands in one place (D9 step 2): a
// statement, a coverage record, or neither (a gap). Never "covered by the
// national page": for a state, a national line is not the state's answer.
type RulesAnswer struct {
	Concept Concept
	// Statement is the answer when one exists, with the page it sits on.
	Statement *CitedStatement
	PageTitle string
	TopicSlug string
	PageKind  string
	Status    string // "published" or "draft"
	// NoLaw is the coverage record when the law was searched and not found.
	NoLaw *CoverageRecord
}

// Gap reports a concept with neither an answer nor a record.
func (a RulesAnswer) Gap() bool { return a.Statement == nil && a.NoLaw == nil }

// RulesAnswers works out a rules page (D4 step 1): for every concept homed in
// the rules topic's situation topic, in order, the statement that answers it
// in this place, else its coverage record. A lookup by tag, never a
// judgement. With drafts true, draft pages count (a statement sitting in
// review is not a gap to draft twice); the public page passes false and
// shows only published answers. A situation page answers before the rules
// page itself, and a page on the home topic before any other.
func (pg *PG) RulesAnswers(ctx context.Context, jurisdictionID int64, rulesTopicSlug, language string, drafts bool) ([]RulesAnswer, error) {
	var homeID int64
	var kind string
	if err := pg.pool.QueryRow(ctx, `
		SELECT COALESCE(t.rules_for, 0), j.kind FROM topics t, jurisdictions j
		WHERE t.slug = $1 AND j.id = $2`, rulesTopicSlug, jurisdictionID).Scan(&homeID, &kind); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if homeID == 0 {
		return nil, fmt.Errorf("topic %q is not a rules topic", rulesTopicSlug)
	}
	crow, err := pg.pool.Query(ctx, `
		SELECT c.id, c.slug, c.name, c.topic_id, t.slug, c.question, c.position, c.definition
		FROM concepts c JOIN topics t ON t.id = c.topic_id
		WHERE c.topic_id = $1 AND (NOT c.national_only OR $2 = 'country')
		ORDER BY c.position, c.slug`, homeID, kind)
	if err != nil {
		return nil, err
	}
	var answers []RulesAnswer
	idx := map[int64]int{}
	for crow.Next() {
		var c Concept
		if err := crow.Scan(&c.ID, &c.Slug, &c.Name, &c.TopicID, &c.TopicSlug, &c.Question, &c.Position, &c.Definition); err != nil {
			crow.Close()
			return nil, err
		}
		idx[c.ID] = len(answers)
		answers = append(answers, RulesAnswer{Concept: c})
	}
	crow.Close()
	if err := crow.Err(); err != nil {
		return nil, err
	}

	statuses := []string{"published"}
	if drafts {
		statuses = append(statuses, "draft")
	}
	srow, err := pg.pool.Query(ctx, `
		SELECT DISTINCT ON (s.concept_id) s.concept_id, s.id, pb.title, t.slug, pb.page_kind, pb.status
		FROM statements s
		JOIN playbook_statements ps ON ps.statement_id = s.id
		JOIN playbooks pb ON pb.id = ps.playbook_id
		JOIN topics t ON t.id = pb.topic_id
		WHERE pb.jurisdiction_id = $1 AND pb.language = $2 AND pb.status = ANY($3)
		  AND s.concept_id IN (SELECT id FROM concepts WHERE topic_id = $4)
		ORDER BY s.concept_id, (pb.page_kind = 'rules'), (pb.topic_id <> $4),
		         (pb.status = 'published') DESC, ps.position`,
		jurisdictionID, language, statuses, homeID)
	if err != nil {
		return nil, err
	}
	stmtOf := map[int64]int{} // statement id -> answer index
	var ids []int64
	for srow.Next() {
		var conceptID, stmtID int64
		var title, topicSlug, pageKind, status string
		if err := srow.Scan(&conceptID, &stmtID, &title, &topicSlug, &pageKind, &status); err != nil {
			srow.Close()
			return nil, err
		}
		i, ok := idx[conceptID]
		if !ok {
			continue
		}
		answers[i].PageTitle, answers[i].TopicSlug, answers[i].PageKind, answers[i].Status = title, topicSlug, pageKind, status
		stmtOf[stmtID] = i
		ids = append(ids, stmtID)
	}
	srow.Close()
	if err := srow.Err(); err != nil {
		return nil, err
	}

	if len(ids) > 0 {
		rows, err := pg.pool.Query(ctx, `
			SELECT
				s.id, s.key::text, s.body_md, COALESCE(co.slug, ''), '', '', 0, '',
				c.source_id, c.locator, c.quote, c.manually_verified, c.checked_at, c.checked_by,
				src.url, src.publisher, src.kind,
				`+reviewedAtSQL+`, `+reviewedBySQL+`, `+undecidedSQL+`, `+proposalPendingSQL+`,
				COALESCE(`+sourceUnreadableSQL+`, false)
			FROM statements s
			JOIN statement_review_hash h ON h.statement_id = s.id
			LEFT JOIN concepts co ON co.id = s.concept_id
			JOIN citations c ON c.statement_id = s.id
			JOIN sources src ON src.id = c.source_id
			WHERE s.id = ANY($1)
			ORDER BY s.id, c.source_id, c.id`, ids)
		if err != nil {
			return nil, err
		}
		stmts := assembleStatements(rows)
		rows.Close()
		for k := range stmts {
			st := stmts[k]
			answers[stmtOf[st.ID]].Statement = &st
		}
	}

	records, err := pg.coverageRecords(ctx, jurisdictionID, 0)
	if err != nil {
		return nil, err
	}
	bySlug := map[string]int{}
	for i, a := range answers {
		bySlug[a.Concept.Slug] = i
	}
	for k := range records {
		r := records[k]
		if i, ok := bySlug[r.ConceptSlug]; ok && answers[i].Statement == nil {
			answers[i].NoLaw = &r
		}
	}
	return answers, nil
}

// PlaceGaps is the gap list for one place and rules topic: the input a
// rules-page drafter gets (D4 step 2), and nothing else.
type PlaceGaps struct {
	JurisdictionSlug string
	RulesTopicSlug   string
	Answers          []RulesAnswer
}

// Gaps returns the concepts still unanswered.
func (g PlaceGaps) Gaps() []Concept {
	var out []Concept
	for _, a := range g.Answers {
		if a.Gap() {
			out = append(out, a.Concept)
		}
	}
	return out
}

// RulesGaps works out the gaps for every rules topic in one place, drafts
// counted as answers (D9 step 2). National-only topics are skipped for
// states, and a rules topic whose situation is national only (rental
// application) is still listed: its rules vary by state.
func (pg *PG) RulesGaps(ctx context.Context, jurisdictionSlug string) ([]PlaceGaps, error) {
	j, err := pg.GetJurisdictionBySlug(ctx, jurisdictionSlug)
	if err != nil {
		return nil, err
	}
	topics, err := pg.queryTopics(ctx, `SELECT `+topicCols+` FROM topics WHERE rules_for IS NOT NULL ORDER BY slug`)
	if err != nil {
		return nil, err
	}
	var out []PlaceGaps
	for _, t := range topics {
		answers, err := pg.RulesAnswers(ctx, j.ID, t.Slug, "en", true)
		if err != nil {
			return nil, err
		}
		out = append(out, PlaceGaps{JurisdictionSlug: j.Slug, RulesTopicSlug: t.Slug, Answers: answers})
	}
	return out, nil
}

// ConceptAnsweredElsewhere reports the title of another page in the same
// place that already answers the concept (published or draft), or "". A
// rules page holds only the facts no situation page carries (D4), so a
// rules-page save refuses a concept a situation page already answers.
func (pg *PG) ConceptAnsweredElsewhere(ctx context.Context, jurisdictionID, rulesTopicID int64, conceptSlug, language string) (string, error) {
	var title string
	err := pg.pool.QueryRow(ctx, `
		SELECT pb.title
		FROM statements s
		JOIN concepts c ON c.id = s.concept_id
		JOIN playbook_statements ps ON ps.statement_id = s.id
		JOIN playbooks pb ON pb.id = ps.playbook_id
		WHERE pb.jurisdiction_id = $1 AND pb.topic_id <> $2 AND c.slug = $3
		  AND pb.language = $4 AND pb.status IN ('published', 'draft')
		ORDER BY (pb.status = 'published') DESC
		LIMIT 1`, jurisdictionID, rulesTopicID, conceptSlug, language).Scan(&title)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return title, err
}
