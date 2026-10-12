package store

import (
	"context"
	"strings"
)

// QualityPage is one page on the quality map (ADR-027 D5): where it sits,
// what it is, and every check it fails.
type QualityPage struct {
	ID               int64
	Title            string
	Status           string // draft or published
	PageKind         string
	JurisdictionName string
	JurisdictionSlug string
	JurisdictionKind string
	// TopicSlug is the core topic the page sits under on the map. A rules
	// page sits under the situation topic it answers (ADR-028 D4).
	TopicSlug string
	// Statements is how many statements the page has, so a statement-by-
	// check grid has a row for every one, passing or not.
	Statements int
	Checks     []QualityCheck
}

// Worst is the severity of the page's worst failing check, 0 when it passes.
func (p QualityPage) Worst() int {
	if len(p.Checks) == 0 {
		return 0
	}
	return p.Checks[0].Severity
}

// AuthorQuality runs the quality checks over every draft and published page
// in an active language and returns them, places in map order (the country
// first, then states, then cities, each by name) and topics by slug.
//
// Drafts are judged alongside live pages on purpose: a draft is where an
// agent can still fix what the map shows (Nazanin, 2026-10-11).
func (pg *PG) AuthorQuality(ctx context.Context) ([]QualityPage, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT pb.id, pb.title, pb.status, pb.page_kind, pb.language,
		       j.name, j.slug, j.kind,
		       COALESCE(rt.slug, t.slug)
		FROM playbooks pb
		JOIN jurisdictions j ON j.id = pb.jurisdiction_id
		JOIN topics t ON t.id = pb.topic_id
		LEFT JOIN topics rt ON rt.id = t.rules_for
		WHERE pb.status IN ('draft', 'published')
		  AND pb.language = ANY($1)
		ORDER BY (j.kind = 'country') DESC, (j.kind = 'state') DESC, j.name, COALESCE(rt.slug, t.slug), pb.status`,
		ContentLanguages)
	if err != nil {
		return nil, err
	}
	pages := []QualityPage{}
	index := map[int64]int{}
	inputs := map[int64]*QualityInput{}
	for rows.Next() {
		var p QualityPage
		var lang string
		if err := rows.Scan(&p.ID, &p.Title, &p.Status, &p.PageKind, &lang,
			&p.JurisdictionName, &p.JurisdictionSlug, &p.JurisdictionKind, &p.TopicSlug); err != nil {
			rows.Close()
			return nil, err
		}
		index[p.ID] = len(pages)
		pages = append(pages, p)
		inputs[p.ID] = &QualityInput{Language: lang, PageKind: p.PageKind, Warns: map[string]bool{}}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Statements in page order, each with its citations. One pass over the
	// join: a statement with no citation still arrives, with NULL source
	// columns, so the "no citation" tier is honest.
	srows, err := pg.pool.Query(ctx, `
		SELECT ps.playbook_id, ps.position, s.body_md,
		       COALESCE(src.kind, ''), COALESCE(src.url, ''), COALESCE(c.locator, ''),
		       COALESCE(c.quote, ''),
		       COALESCE(c.checked_at IS NOT NULL OR c.manually_verified, false),
		       COALESCE(c.checked_via, '')
		FROM playbook_statements ps
		JOIN playbooks pb ON pb.id = ps.playbook_id
		JOIN statements s ON s.id = ps.statement_id
		LEFT JOIN citations c ON c.statement_id = s.id
		LEFT JOIN sources src ON src.id = c.source_id
		WHERE pb.status IN ('draft', 'published') AND pb.language = ANY($1)
		ORDER BY ps.playbook_id, ps.position, c.id`,
		ContentLanguages)
	if err != nil {
		return nil, err
	}
	defer srows.Close()
	lastPage, lastPos := int64(-1), -1
	for srows.Next() {
		var pid int64
		var pos int
		var body string
		var c QualityCitation
		if err := srows.Scan(&pid, &pos, &body, &c.SourceKind, &c.URL, &c.Locator, &c.Quote, &c.Checked, &c.CheckedVia); err != nil {
			return nil, err
		}
		in, ok := inputs[pid]
		if !ok {
			continue
		}
		if pid != lastPage || pos != lastPos {
			in.Statements = append(in.Statements, QualityStatement{Body: body})
			lastPage, lastPos = pid, pos
		}
		if c.SourceKind != "" {
			st := &in.Statements[len(in.Statements)-1]
			st.Citations = append(st.Citations, c)
		}
	}
	if err := srows.Err(); err != nil {
		return nil, err
	}

	// The risk a page says once at the top (ADR-016 A3), so a statement
	// that relies on it is not flagged.
	wrows, err := pg.pool.Query(ctx, `
		SELECT pa.playbook_id, a.warns FROM playbook_advice pa
		JOIN advice a ON a.id = pa.advice_id
		WHERE a.warns IS NOT NULL AND a.retired_at IS NULL`)
	if err != nil {
		return nil, err
	}
	defer wrows.Close()
	for wrows.Next() {
		var pid int64
		var w string
		if err := wrows.Scan(&pid, &w); err != nil {
			return nil, err
		}
		if in, ok := inputs[pid]; ok {
			in.Warns[strings.TrimSpace(w)] = true
		}
	}
	if err := wrows.Err(); err != nil {
		return nil, err
	}

	for id, in := range inputs {
		pages[index[id]].Statements = len(in.Statements)
		pages[index[id]].Checks = PageQualityChecks(*in)
	}
	return pages, nil
}
