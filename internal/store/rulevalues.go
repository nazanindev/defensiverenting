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

// Rule values (ADR-034): a number lives once, in a typed row.
//
// A statement tagged to a concept carries the number in prose. The row here
// carries the number itself, once per place, concept and condition, with
// its own verbatim quote so the weekly checker can re-read it (D2). Prose
// is checked against the rows, never generated from them (D4). "No law
// found" is a coverage record, never a row (D3).

// RuleValueKinds are the units a rule value can carry.
var RuleValueKinds = []string{"days", "dollars", "months_of_rent", "percent", "count"}

// RuleValue is one number for one place, concept and condition.
type RuleValue struct {
	ID               int64
	JurisdictionID   int64
	JurisdictionName string
	JurisdictionSlug string
	ConceptSlug      string
	Condition        string
	Kind             string
	Value            float64
	// ValueMax is set when the law gives a range. Nil otherwise.
	ValueMax  *float64
	SourceID  int64
	SourceURL string
	Publisher string
	Locator   string
	Quote     string
	Note      string
	CheckedBy string
	CheckedAt time.Time
	// SourceCheckedAt and DriftAt are the checker's stamps: when the quote
	// was last found at the live source, and when it was last missing.
	SourceCheckedAt *time.Time
	DriftAt         *time.Time
	DriftNote       string
}

// FileRuleValueParams files or refreshes a rule value. The source is named
// by URL and must already exist: a number cites a source a statement
// already cites, and a new source is a drafting decision, not a side effect
// of filing a number.
type FileRuleValueParams struct {
	JurisdictionSlug string
	ConceptSlug      string
	Condition        string
	Kind             string
	Value            float64
	ValueMax         *float64
	SourceURL        string
	Locator          string
	Quote            string
	Note             string
	By               string
}

// FileRuleValue writes one row. Filing again for the same place, concept
// and condition replaces the value and citation and refreshes the stamp.
// Invariants: a known kind, a non-negative value, a range that does not run
// backwards, a verbatim quote, a known non-editorial source, a named filer,
// and a condition from the question's list of situations when it has one
// (ADR-036 D2).
func (pg *PG) FileRuleValue(ctx context.Context, p FileRuleValueParams) error {
	if !validRuleValueKind(p.Kind) {
		return fmt.Errorf("kind %q is not one of %s", p.Kind, strings.Join(RuleValueKinds, ", "))
	}
	if p.Value < 0 {
		return errors.New("a rule value is not negative")
	}
	if p.ValueMax != nil && *p.ValueMax < p.Value {
		return errors.New("the top of a range is not below its bottom")
	}
	if strings.TrimSpace(p.Quote) == "" {
		return errors.New("a rule value carries the verbatim quote that states it")
	}
	if strings.TrimSpace(p.By) == "" {
		return errors.New("a rule value names who filed it")
	}
	if strings.TrimSpace(p.SourceURL) == "" {
		return errors.New("a rule value names the source URL it cites")
	}
	return pgx.BeginTxFunc(ctx, pg.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var jID, cID int64
		err := tx.QueryRow(ctx, `
			SELECT j.id, c.id FROM jurisdictions j, concepts c
			WHERE j.slug = $1 AND c.slug = $2`, p.JurisdictionSlug, p.ConceptSlug).Scan(&jID, &cID)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("no place %q or no concept %q: %w", p.JurisdictionSlug, p.ConceptSlug, ErrNotFound)
		}
		if err != nil {
			return err
		}
		if err := situationAllowed(ctx, tx, cID, strings.TrimSpace(p.Condition)); err != nil {
			return err
		}
		var sID int64
		var kind string
		err = tx.QueryRow(ctx, `SELECT id, kind FROM sources WHERE url = $1`, strings.TrimSpace(p.SourceURL)).Scan(&sID, &kind)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("no source with URL %q; a rule value cites a source a statement already cites: %w", p.SourceURL, ErrNotFound)
		}
		if err != nil {
			return err
		}
		if kind == "editorial" {
			return errors.New("a rule value cites law or official guidance, not editorial guidance")
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO rule_values (jurisdiction_id, concept_id, condition, kind, value, value_max,
			                         source_id, locator, quote, note, checked_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			ON CONFLICT (jurisdiction_id, concept_id, condition) DO UPDATE
			   SET kind = EXCLUDED.kind, value = EXCLUDED.value, value_max = EXCLUDED.value_max,
			       source_id = EXCLUDED.source_id, locator = EXCLUDED.locator, quote = EXCLUDED.quote,
			       note = EXCLUDED.note, checked_by = EXCLUDED.checked_by, checked_at = NOW(),
			       source_checked_at = NULL, drift_at = NULL, drift_note = ''`,
			jID, cID, strings.TrimSpace(p.Condition), p.Kind, p.Value, p.ValueMax,
			sID, strings.TrimSpace(p.Locator), strings.TrimSpace(p.Quote), strings.TrimSpace(p.Note), strings.TrimSpace(p.By))
		return err
	})
}

func validRuleValueKind(k string) bool {
	for _, v := range RuleValueKinds {
		if v == k {
			return true
		}
	}
	return false
}

// RemoveRuleValue takes a row back. Reports whether one existed.
func (pg *PG) RemoveRuleValue(ctx context.Context, jurisdictionSlug, conceptSlug, condition string) (bool, error) {
	tag, err := pg.pool.Exec(ctx, `
		DELETE FROM rule_values r
		USING jurisdictions j, concepts c
		WHERE r.jurisdiction_id = j.id AND r.concept_id = c.id
		  AND j.slug = $1 AND c.slug = $2 AND r.condition = $3`,
		jurisdictionSlug, conceptSlug, strings.TrimSpace(condition))
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ListRuleValues lists rows, filtered by place slug or concept slug. Either
// may be empty for "any". Ordered by place, then concept, then condition,
// which is the order a comparison table reads in.
func (pg *PG) ListRuleValues(ctx context.Context, jurisdictionSlug, conceptSlug string) ([]RuleValue, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT r.id, j.id, j.name, j.slug, c.slug, r.condition, r.kind, r.value, r.value_max,
		       s.id, s.url, s.publisher, r.locator, r.quote, r.note, r.checked_by, r.checked_at,
		       r.source_checked_at, r.drift_at, r.drift_note
		FROM rule_values r
		JOIN jurisdictions j ON j.id = r.jurisdiction_id
		JOIN concepts c ON c.id = r.concept_id
		JOIN sources s ON s.id = r.source_id
		WHERE ($1 = '' OR j.slug = $1) AND ($2 = '' OR c.slug = $2)
		ORDER BY (j.kind = 'country') DESC, (j.kind = 'state') DESC, j.name, c.position, c.slug, r.condition`,
		jurisdictionSlug, conceptSlug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RuleValue
	for rows.Next() {
		var r RuleValue
		if err := rows.Scan(&r.ID, &r.JurisdictionID, &r.JurisdictionName, &r.JurisdictionSlug, &r.ConceptSlug,
			&r.Condition, &r.Kind, &r.Value, &r.ValueMax,
			&r.SourceID, &r.SourceURL, &r.Publisher, &r.Locator, &r.Quote, &r.Note, &r.CheckedBy, &r.CheckedAt,
			&r.SourceCheckedAt, &r.DriftAt, &r.DriftNote); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RuleValueCheckRow is what the source checker needs about one row.
type RuleValueCheckRow struct {
	ID        int64
	Place     string
	Concept   string
	Condition string
	SourceID  int64
	URL       string
	Quote     string
}

// ListRuleValueCitationsForCheck returns every rule value's quote, grouped
// by source URL so a run fetches each page once.
func (pg *PG) ListRuleValueCitationsForCheck(ctx context.Context) ([]RuleValueCheckRow, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT r.id, j.slug, c.slug, r.condition, s.id, s.url, r.quote
		FROM rule_values r
		JOIN jurisdictions j ON j.id = r.jurisdiction_id
		JOIN concepts c ON c.id = r.concept_id
		JOIN sources s ON s.id = r.source_id
		ORDER BY s.url, r.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RuleValueCheckRow
	for rows.Next() {
		var r RuleValueCheckRow
		if err := rows.Scan(&r.ID, &r.Place, &r.Concept, &r.Condition, &r.SourceID, &r.URL, &r.Quote); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// MarkRuleValueCitation records one check of one rule value's quote: found
// clears any drift and stamps the time; missing records the drift once.
func (pg *PG) MarkRuleValueCitation(ctx context.Context, id int64, found bool, note string) error {
	if found {
		_, err := pg.pool.Exec(ctx, `
			UPDATE rule_values SET source_checked_at = now(), drift_at = NULL, drift_note = '' WHERE id = $1`, id)
		return err
	}
	_, err := pg.pool.Exec(ctx, `
		UPDATE rule_values SET drift_at = COALESCE(drift_at, now()), drift_note = $2 WHERE id = $1`, id, note)
	return err
}

// ConceptStatement is one law-backed statement tagged to a concept, with
// its citations, in the shape a backfill agent reads to write rule values.
type ConceptStatement struct {
	Key              string
	JurisdictionSlug string
	JurisdictionName string
	PageTitle        string
	Status           string
	BodyMD           string
	Citations        []ProposedCitation
}

// StatementsForConcept lists every law-backed statement carrying the
// concept on a draft or published English page, published before draft,
// states and the country before cities. This is the backfill's input
// (ADR-034 D7): the agent reads the prose and the quotes and files the
// numbers it finds.
func (pg *PG) StatementsForConcept(ctx context.Context, conceptSlug string) ([]ConceptStatement, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT s.key::text, j.slug, j.name, pb.title, pb.status, s.body_md,
		       COALESCE((SELECT json_agg(json_build_object(
		                 'url', src.url, 'publisher', src.publisher, 'kind', src.kind,
		                 'locator', c.locator, 'quote', c.quote) ORDER BY c.id)
		          FROM citations c JOIN sources src ON src.id = c.source_id
		         WHERE c.statement_id = s.id), '[]'::json)
		FROM statements s
		JOIN concepts co ON co.id = s.concept_id
		JOIN playbook_statements ps ON ps.statement_id = s.id
		JOIN playbooks pb ON pb.id = ps.playbook_id
		JOIN jurisdictions j ON j.id = pb.jurisdiction_id
		WHERE co.slug = $1 AND pb.status IN ('published', 'draft') AND pb.language = 'en'
		  AND `+lawBackedSQL+`
		ORDER BY (j.kind = 'country') DESC, (j.kind = 'state') DESC, j.name,
		         (pb.status = 'published') DESC, pb.id, ps.position`, conceptSlug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ConceptStatement
	for rows.Next() {
		var r ConceptStatement
		var citations []byte
		if err := rows.Scan(&r.Key, &r.JurisdictionSlug, &r.JurisdictionName, &r.PageTitle, &r.Status, &r.BodyMD, &citations); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(citations, &r.Citations); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
