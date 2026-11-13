package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// TopicShape is what a page of a topic holds (ADR-036 D4), read from the
// topic's rows. It is the same in every state. Its four parts are stored
// apart, so a change to one touches nothing in the others.
type TopicShape struct {
	Slug   string   `json:"topic"`
	Name   string   `json:"name"`
	Stages []string `json:"stages"`

	Scope     ShapeScope      `json:"scope"`
	Advice    []ShapeAdvice   `json:"tips_and_notes"`
	Size      ShapeSize       `json:"size_and_order"`
	Questions []ShapeQuestion `json:"must_answer"`
}

// ShapeScope is part 1: title and scope.
type ShapeScope struct {
	// TitlePattern holds {Place} where the place goes.
	TitlePattern string   `json:"title_pattern"`
	Reader       string   `json:"reader"`
	IntroOpens   string   `json:"intro_opens,omitempty"`
	Covers       string   `json:"covers"`
	NotHere      []string `json:"not_here"`
}

// ShapeAdvice is one row of part 2: a tip or warning note the topic uses.
type ShapeAdvice struct {
	Slug string `json:"advice"`
	Kind string `json:"kind"`
	// Use is every_page, where_it_fits or never.
	Use  string `json:"use"`
	Body string `json:"body"`
}

// ShapeSize is part 3: size and order.
type ShapeSize struct {
	MaxStatements     int `json:"max_statements"`
	MaxWords          int `json:"max_words"`
	MaxStatementWords int `json:"max_statement_words"`
	MaxExtras         int `json:"max_extras"`
	// CityMore is how many more statements, and how many more extras, a
	// city page may hold than a state page.
	CityMore   int             `json:"city_more,omitempty"`
	OrderRules []string        `json:"order_rules"`
	Example    string          `json:"example_statement"`
	Extras     []ShapeQuestion `json:"extras"`
}

// ShapeQuestion is one question of a topic: a must-answer question of
// part 4, or an extra of part 3.
type ShapeQuestion struct {
	Slug     string `json:"concept"`
	Question string `json:"question"`
	// Stage is the section a must answer sits under; "" on an extra.
	Stage string `json:"stage,omitempty"`
	// Answer is days (every place has a number), days_where_set (a number
	// only where the law sets one), list (one statement per protection), or
	// "" for words only (ADR-036 D7).
	Answer string `json:"answer,omitempty"`
	// Situations is the fixed list a stored number's condition comes from.
	// Empty means the base case only.
	Situations []ShapeSituation `json:"situations,omitempty"`
	NeverCut   bool             `json:"never_cut,omitempty"`
}

// ShapeSituation is one named case of a question.
type ShapeSituation struct {
	Name string `json:"name"`
	Base bool   `json:"base,omitempty"`
}

// Title is the shape's title for a place; the nationwide page (place "")
// drops " in {Place}".
func (s TopicShape) Title(place string) string {
	if place == "" {
		return strings.Replace(s.Scope.TitlePattern, " in {Place}", "", 1)
	}
	return strings.Replace(s.Scope.TitlePattern, "{Place}", place, 1)
}

// Missing names the parts a topic has not filled, in plain words. A topic
// with all four parts filled returns nothing.
func (s TopicShape) Missing() []string {
	var out []string
	if s.Scope.TitlePattern == "" || s.Scope.Reader == "" || s.Scope.Covers == "" {
		out = append(out, "title and scope")
	}
	if len(s.Advice) == 0 {
		out = append(out, "tips and warning notes")
	}
	if s.Size.MaxStatements == 0 || s.Size.MaxWords == 0 {
		out = append(out, "size and order")
	}
	if len(s.Questions) == 0 {
		out = append(out, "must-answer questions")
	}
	return out
}

// TopicShapes returns the shape of one topic, or of every topic when slug
// is "".
func (pg *PG) TopicShapes(ctx context.Context, slug string) ([]TopicShape, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT t.id, t.slug, t.name, t.stages,
		       t.title_pattern, t.reader, t.intro_opens, t.covers, t.not_here,
		       t.max_statements, t.max_words, t.max_statement_words, t.max_extras, t.city_more,
		       t.order_rules, t.example_statement
		FROM topics t
		WHERE $1 = '' OR t.slug = $1
		ORDER BY (t.rules_for IS NOT NULL), t.national_only, t.is_core DESC, t.slug`, slug)
	if err != nil {
		return nil, err
	}
	var out []TopicShape
	at := map[int64]int{}
	for rows.Next() {
		var id int64
		var s TopicShape
		if err := rows.Scan(&id, &s.Slug, &s.Name, &s.Stages,
			&s.Scope.TitlePattern, &s.Scope.Reader, &s.Scope.IntroOpens, &s.Scope.Covers, &s.Scope.NotHere,
			&s.Size.MaxStatements, &s.Size.MaxWords, &s.Size.MaxStatementWords, &s.Size.MaxExtras, &s.Size.CityMore,
			&s.Size.OrderRules, &s.Size.Example); err != nil {
			rows.Close()
			return nil, err
		}
		at[id] = len(out)
		out = append(out, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if slug != "" && len(out) == 0 {
		return nil, fmt.Errorf("no topic %q: %w", slug, ErrNotFound)
	}

	// The situations of every question, by concept slug.
	situations := map[string][]ShapeSituation{}
	srows, err := pg.pool.Query(ctx, `
		SELECT c.slug, s.name, s.is_base
		FROM concept_situations s JOIN concepts c ON c.id = s.concept_id
		ORDER BY c.slug, s.position, s.name`)
	if err != nil {
		return nil, err
	}
	for srows.Next() {
		var c string
		var s ShapeSituation
		if err := srows.Scan(&c, &s.Name, &s.Base); err != nil {
			srows.Close()
			return nil, err
		}
		situations[c] = append(situations[c], s)
	}
	srows.Close()
	if err := srows.Err(); err != nil {
		return nil, err
	}

	qrows, err := pg.pool.Query(ctx, `
		SELECT q.topic_id, c.slug, c.question, q.role, q.stage, q.answer, q.never_cut
		FROM topic_questions q JOIN concepts c ON c.id = q.concept_id
		ORDER BY q.topic_id, q.position, c.slug`)
	if err != nil {
		return nil, err
	}
	for qrows.Next() {
		var id int64
		var role string
		var q ShapeQuestion
		if err := qrows.Scan(&id, &q.Slug, &q.Question, &role, &q.Stage, &q.Answer, &q.NeverCut); err != nil {
			qrows.Close()
			return nil, err
		}
		i, ok := at[id]
		if !ok {
			continue
		}
		q.Situations = situations[q.Slug]
		if role == "extra" {
			out[i].Size.Extras = append(out[i].Size.Extras, q)
		} else {
			out[i].Questions = append(out[i].Questions, q)
		}
	}
	qrows.Close()
	if err := qrows.Err(); err != nil {
		return nil, err
	}

	arows, err := pg.pool.Query(ctx, `
		SELECT ta.topic_id, a.slug, a.kind, ta.use, a.body_md
		FROM topic_advice ta JOIN advice a ON a.id = ta.advice_id
		WHERE a.retired_at IS NULL
		ORDER BY ta.topic_id, (ta.use = 'every_page') DESC, (ta.use = 'never'), a.kind, a.slug`)
	if err != nil {
		return nil, err
	}
	defer arows.Close()
	for arows.Next() {
		var id int64
		var a ShapeAdvice
		if err := arows.Scan(&id, &a.Slug, &a.Kind, &a.Use, &a.Body); err != nil {
			return nil, err
		}
		if i, ok := at[id]; ok {
			out[i].Advice = append(out[i].Advice, a)
		}
	}
	return out, arows.Err()
}

// A cell of a topic table is in one of these states (ADR-036 D3, D7).
const (
	CellNumber     = "number"      // a stored number, with its quote
	CellWords      = "words"       // the page answers the question; no number is stored
	CellNoLaw      = "no_law"      // searched, no law found
	CellNotChecked = "not_checked" // nothing on the page and no record
)

// TopicTable is a topic's must-answer questions that are answered with
// a number of days, across every place that has a page of the topic.
type TopicTable struct {
	Topic     string          `json:"topic"`
	Questions []ShapeQuestion `json:"questions"`
	Rows      []TableRow      `json:"rows"`
}

// TableRow is one place. Cells runs parallel to TopicTable.Questions.
type TableRow struct {
	Place string      `json:"place"`
	Name  string      `json:"name"`
	Kind  string      `json:"kind"`
	Cells []TableCell `json:"cells"`
}

// TableCell is one place and one question.
type TableCell struct {
	State  string        `json:"state"`
	Values []TableNumber `json:"values,omitempty"`
}

// TableNumber is one stored number of a cell.
type TableNumber struct {
	Situation string   `json:"situation,omitempty"`
	Kind      string   `json:"kind"`
	Value     float64  `json:"value"`
	ValueMax  *float64 `json:"value_max,omitempty"`
	Drifted   bool     `json:"drifted,omitempty"`
}

// Gap reports whether a cell is a hole to research: a days question with
// no stored number, or any question nobody has checked. A days_where_set
// question answered in words is not a gap.
func (c TableCell) Gap(q ShapeQuestion) bool {
	switch c.State {
	case CellNotChecked:
		return true
	case CellWords:
		return q.Answer == "days"
	}
	return false
}

// TopicTable builds the table of a topic from its shape and the stored
// rule values. It reads; it writes nothing.
func (pg *PG) TopicTable(ctx context.Context, slug string) (TopicTable, error) {
	out := TopicTable{Topic: slug}
	shapes, err := pg.TopicShapes(ctx, slug)
	if err != nil {
		return out, err
	}
	col := map[string]int{}
	for _, q := range shapes[0].Questions {
		if q.Answer != "days" && q.Answer != "days_where_set" {
			continue
		}
		col[q.Slug] = len(out.Questions)
		out.Questions = append(out.Questions, q)
	}
	if len(out.Questions) == 0 {
		return out, fmt.Errorf("topic %q has no must-answer question answered in days", slug)
	}

	// Every state and city with a page of the topic, draft or live.
	row := map[string]int{}
	prows, err := pg.pool.Query(ctx, `
		SELECT DISTINCT j.slug, j.name, j.kind
		FROM playbooks p
		JOIN topics t ON t.id = p.topic_id
		JOIN jurisdictions j ON j.id = p.jurisdiction_id
		WHERE t.slug = $1 AND p.language = 'en' AND p.status IN ('draft', 'published')
		  AND j.kind IN ('state', 'city')
		ORDER BY j.name, j.slug`, slug)
	if err != nil {
		return out, err
	}
	for prows.Next() {
		r := TableRow{Cells: make([]TableCell, len(out.Questions))}
		if err := prows.Scan(&r.Place, &r.Name, &r.Kind); err != nil {
			prows.Close()
			return out, err
		}
		for i := range r.Cells {
			r.Cells[i].State = CellNotChecked
		}
		row[r.Place] = len(out.Rows)
		out.Rows = append(out.Rows, r)
	}
	prows.Close()
	if err := prows.Err(); err != nil {
		return out, err
	}

	// cell finds the cell of a place and a concept, if the table has one.
	cell := func(place, concept string) *TableCell {
		i, ok := row[place]
		j, ok2 := col[concept]
		if !ok || !ok2 {
			return nil
		}
		return &out.Rows[i].Cells[j]
	}
	each := func(sql string, scan func(pgx.Rows) error, args ...any) error {
		r, err := pg.pool.Query(ctx, sql, args...)
		if err != nil {
			return err
		}
		defer r.Close()
		for r.Next() {
			if err := scan(r); err != nil {
				return err
			}
		}
		return r.Err()
	}
	// Weakest state first, so a stronger one overwrites it.
	err = errors.Join(
		each(`
			SELECT j.slug, c.slug FROM coverage_records r
			JOIN jurisdictions j ON j.id = r.jurisdiction_id
			JOIN concepts c ON c.id = r.concept_id`,
			func(r pgx.Rows) error {
				var place, concept string
				if err := r.Scan(&place, &concept); err != nil {
					return err
				}
				if c := cell(place, concept); c != nil {
					c.State = CellNoLaw
				}
				return nil
			}),
		each(`
			SELECT DISTINCT j.slug, c.slug
			FROM playbooks p
			JOIN topics t ON t.id = p.topic_id
			JOIN jurisdictions j ON j.id = p.jurisdiction_id
			JOIN playbook_statements ps ON ps.playbook_id = p.id
			JOIN statements s ON s.id = ps.statement_id
			JOIN concepts c ON c.id = s.concept_id
			WHERE t.slug = $1 AND p.language = 'en' AND p.status IN ('draft', 'published')`,
			func(r pgx.Rows) error {
				var place, concept string
				if err := r.Scan(&place, &concept); err != nil {
					return err
				}
				if c := cell(place, concept); c != nil {
					c.State = CellWords
				}
				return nil
			}, slug),
		each(`
			SELECT j.slug, c.slug, rv.condition, rv.kind, rv.value::float8, rv.value_max::float8, rv.drift_at IS NOT NULL
			FROM rule_values rv
			JOIN jurisdictions j ON j.id = rv.jurisdiction_id
			JOIN concepts c ON c.id = rv.concept_id
			LEFT JOIN concept_situations s ON s.concept_id = rv.concept_id AND s.name = rv.condition
			ORDER BY j.slug, c.slug, s.position NULLS FIRST, rv.condition`,
			func(r pgx.Rows) error {
				var place, concept string
				var n TableNumber
				if err := r.Scan(&place, &concept, &n.Situation, &n.Kind, &n.Value, &n.ValueMax, &n.Drifted); err != nil {
					return err
				}
				if c := cell(place, concept); c != nil {
					c.State = CellNumber
					c.Values = append(c.Values, n)
				}
				return nil
			}),
	)
	return out, err
}

// situationAllowed reports whether a rule value's condition is in its
// question's fixed list (ADR-036 D2). A question with no list takes any
// condition: its list has not been set yet.
func situationAllowed(ctx context.Context, tx pgx.Tx, conceptID int64, condition string) error {
	rows, err := tx.Query(ctx, `SELECT name FROM concept_situations WHERE concept_id = $1 ORDER BY position, name`, conceptID)
	if err != nil {
		return err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return err
		}
		if n == condition {
			return nil
		}
		names = append(names, n)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(names) == 0 {
		return nil
	}
	return fmt.Errorf("condition %q is not one of this question's situations: %s", condition, strings.Join(names, "; "))
}
