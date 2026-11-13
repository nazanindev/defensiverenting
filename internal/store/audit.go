package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

// AuditRow is one place's row on a topic's audit grid (ADR-035 D2).
type AuditRow struct {
	Place     string `json:"place"`
	PlaceSlug string `json:"place_slug"`
	PlaceKind string `json:"place_kind"`
	// PageID is the page audited: the draft in the slot when there is one,
	// else the live page; 0 when the place has no page.
	PageID int64  `json:"page_id,omitempty"`
	Status string `json:"status,omitempty"`
	// LiveID is the live page a draft would replace, 0 when none.
	LiveID     int64 `json:"live_id,omitempty"`
	Statements int   `json:"statements,omitempty"`
	Words      int   `json:"words,omitempty"`
	// Gaps is the must-answer questions the page still has to answer: the input
	// of the research step.
	Gaps     []AuditConcept `json:"gaps,omitempty"`
	Findings []AuditFinding `json:"findings,omitempty"`
}

// Fails counts the row's failures in one group, hints left out.
func (r AuditRow) Fails(group string) int {
	n := 0
	for _, f := range r.Findings {
		if f.Group == group && !f.Hint {
			n++
		}
	}
	return n
}

// Hints counts the row's hints.
func (r AuditRow) Hints() int {
	n := 0
	for _, f := range r.Findings {
		if f.Hint {
			n++
		}
	}
	return n
}

// TopicAudit is a topic's grid.
type TopicAudit struct {
	Topic string     `json:"topic"`
	Rows  []AuditRow `json:"rows"`
}

// AuditTopic audits every page of one topic: a row for the country and every
// state whether or not it has a page, and a row for each city that has one.
// Each place is judged on its draft when one is open, else on its live page.
func (pg *PG) AuditTopic(ctx context.Context, topicSlug string) (TopicAudit, error) {
	out := TopicAudit{Topic: topicSlug}
	var topicID int64
	var stages []string
	var plain bool
	if err := pg.pool.QueryRow(ctx, `SELECT id, stages, plain_layout FROM topics WHERE slug = $1`, topicSlug).
		Scan(&topicID, &stages, &plain); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return out, fmt.Errorf("no topic %q", topicSlug)
		}
		return out, err
	}

	// The topic's shape (ADR-036 D4): its must-answer questions, caps, extras
	// and which tips it uses. A topic with no shape yet has no questions and
	// no caps, and the shape checks are silent.
	shapes, err := pg.TopicShapes(ctx, topicSlug)
	if err != nil {
		return out, err
	}
	shape := &shapes[0]
	var core []AuditConcept
	for _, q := range shape.Questions {
		core = append(core, AuditConcept{Slug: q.Slug, Question: q.Question})
	}
	noLaw := map[string]map[string]bool{} // place slug -> concept slug
	lrows, err := pg.pool.Query(ctx, `
		SELECT j.slug, c.slug FROM coverage_records r
		JOIN jurisdictions j ON j.id = r.jurisdiction_id
		JOIN concepts c ON c.id = r.concept_id
		JOIN topic_questions q ON q.concept_id = c.id AND q.topic_id = $1 AND q.role = 'must'`, topicID)
	if err != nil {
		return out, err
	}
	for lrows.Next() {
		var place, concept string
		if err := lrows.Scan(&place, &concept); err != nil {
			lrows.Close()
			return out, err
		}
		if noLaw[place] == nil {
			noLaw[place] = map[string]bool{}
		}
		noLaw[place][concept] = true
	}
	lrows.Close()
	if err := lrows.Err(); err != nil {
		return out, err
	}

	rows, err := pg.pool.Query(ctx, `
		SELECT j.name, j.slug, j.kind,
		       COALESCE(pb.id, 0), COALESCE(pb.status, ''), COALESCE(pb.title, ''),
		       COALESCE(pb.page_kind, ''), COALESCE(pb.language, '')
		FROM jurisdictions j
		LEFT JOIN playbooks pb ON pb.jurisdiction_id = j.id AND pb.topic_id = $1
		     AND pb.status IN ('draft', 'published') AND pb.language = ANY($2)
		WHERE j.kind IN ('country', 'state') OR pb.id IS NOT NULL
		ORDER BY (j.kind = 'country') DESC, (j.kind = 'state') DESC, j.name, (pb.status = 'draft') DESC`,
		topicID, ContentLanguages)
	if err != nil {
		return out, err
	}
	inputs := map[int64]*AuditInput{}
	at := map[string]int{} // place slug -> row
	var ids []int64
	for rows.Next() {
		var r AuditRow
		var title, kind, lang string
		if err := rows.Scan(&r.Place, &r.PlaceSlug, &r.PlaceKind, &r.PageID, &r.Status, &title, &kind, &lang); err != nil {
			rows.Close()
			return out, err
		}
		if i, seen := at[r.PlaceSlug]; seen {
			// The draft came first; this is the live page it would replace.
			if r.Status == "published" {
				out.Rows[i].LiveID = r.PageID
			}
			continue
		}
		at[r.PlaceSlug] = len(out.Rows)
		if r.PageID != 0 {
			place := r.Place
			if r.PlaceKind == "country" {
				place = ""
			}
			inputs[r.PageID] = &AuditInput{Language: lang, PageKind: kind, Place: place, City: r.PlaceKind != "country" && r.PlaceKind != "state", Title: title, Stages: stages, Plain: plain,
				Core: core, NoLaw: noLaw[r.PlaceSlug], Shape: shape}
			ids = append(ids, r.PageID)
		}
		out.Rows = append(out.Rows, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}
	if len(ids) == 0 {
		return out, nil
	}

	srows, err := pg.pool.Query(ctx, `
		SELECT ps.playbook_id, ps.position, s.key::text, s.body_md, ps.stage, COALESCE(co.slug, ''),
		       COALESCE(src.kind, ''), COALESCE(src.url, ''), COALESCE(c.locator, ''),
		       COALESCE(c.quote, ''),
		       COALESCE(c.checked_at IS NOT NULL OR c.manually_verified, false),
		       COALESCE(c.checked_via, '')
		FROM playbook_statements ps
		JOIN statements s ON s.id = ps.statement_id
		LEFT JOIN concepts co ON co.id = s.concept_id
		LEFT JOIN citations c ON c.statement_id = s.id
		LEFT JOIN sources src ON src.id = c.source_id
		WHERE ps.playbook_id = ANY($1)
		ORDER BY ps.playbook_id, ps.position, c.id`, ids)
	if err != nil {
		return out, err
	}
	lastPage, lastPos := int64(-1), -1
	for srows.Next() {
		var pid int64
		var pos int
		var st AuditStatement
		var c QualityCitation
		if err := srows.Scan(&pid, &pos, &st.Key, &st.Body, &st.Stage, &st.Concept, &c.SourceKind, &c.URL, &c.Locator, &c.Quote, &c.Checked, &c.CheckedVia); err != nil {
			srows.Close()
			return out, err
		}
		in := inputs[pid]
		if pid != lastPage || pos != lastPos {
			in.Statements = append(in.Statements, st)
			lastPage, lastPos = pid, pos
		}
		if c.SourceKind != "" {
			last := &in.Statements[len(in.Statements)-1]
			last.Citations = append(last.Citations, c)
		}
	}
	srows.Close()
	if err := srows.Err(); err != nil {
		return out, err
	}

	nrows, err := pg.pool.Query(ctx, `
		SELECT pa.playbook_id, a.slug, a.kind, COALESCE(a.warns, ''), COALESCE(pa.statement_key::text, '')
		FROM playbook_advice pa
		JOIN advice a ON a.id = pa.advice_id
		WHERE pa.playbook_id = ANY($1) AND a.retired_at IS NULL
		ORDER BY pa.playbook_id, pa.position, a.id`, ids)
	if err != nil {
		return out, err
	}
	for nrows.Next() {
		var pid int64
		var n AuditNote
		if err := nrows.Scan(&pid, &n.Slug, &n.Kind, &n.Warns, &n.StatementKey); err != nil {
			nrows.Close()
			return out, err
		}
		inputs[pid].Notes = append(inputs[pid].Notes, n)
	}
	nrows.Close()
	if err := nrows.Err(); err != nil {
		return out, err
	}

	// What the publish gate holds against each page: stamps, open items,
	// page flags, caps, quotes. One implementation, read here as a group.
	gate, err := collectIssues(ctx, pg.pool, "pb.id = ANY($1)", ids)
	if err != nil {
		return out, err
	}

	var lengths []int
	for i := range out.Rows {
		r := &out.Rows[i]
		in, ok := inputs[r.PageID]
		if !ok {
			continue
		}
		r.Statements = len(in.Statements)
		for _, s := range in.Statements {
			r.Words += len(strings.Fields(s.Body))
		}
		lengths = append(lengths, r.Words)
		r.Gaps = in.Gaps()
		r.Findings = PageAuditChecks(*in)
		for _, is := range gate[r.PageID] {
			key := ""
			if is.Stmt > 0 && is.Stmt <= len(in.Statements) {
				key = in.Statements[is.Stmt-1].Key
			}
			r.Findings = append(r.Findings, AuditFinding{Group: AuditFinished, Code: is.Code, Stmt: is.Stmt, Key: key, Detail: is.Detail})
		}
	}

	// Outliers: a page far from the topic's median length is a hint that a
	// state was over- or under-drafted.
	if len(lengths) >= 5 {
		sort.Ints(lengths)
		median := lengths[len(lengths)/2]
		for i := range out.Rows {
			r := &out.Rows[i]
			if r.PageID == 0 || median == 0 {
				continue
			}
			switch {
			case r.Words*2 < median:
				r.Findings = append(r.Findings, AuditFinding{Group: AuditShape, Code: "outlier-short", Hint: true,
					Detail: fmt.Sprintf("the page runs %d words; the topic's median is %d", r.Words, median)})
			case r.Words*4 > median*7:
				r.Findings = append(r.Findings, AuditFinding{Group: AuditShape, Code: "outlier-long", Hint: true,
					Detail: fmt.Sprintf("the page runs %d words; the topic's median is %d", r.Words, median)})
			}
		}
	}
	return out, nil
}
