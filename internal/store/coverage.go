package store

import "context"

// CoverageCell is one place's standing on one core topic.
//
// State is "published", "draft", or "" when the place has nothing. A draft
// wins over a published page: a place with a draft waiting has work to look
// at, and the map is read for where work is (Nazanin, 2026-10-11). Pages is
// how many pages sit in the cell, the situation page and its rules page
// (ADR-028 D4) together. PageID is set when there is exactly one, so a cell
// can link straight to it.
type CoverageCell struct {
	State  string
	Pages  int
	PageID int64
}

// CoverageRow is one place's standing against every core topic. Status is
// keyed by topic slug so the column order is set once, by the caller's
// topic list, rather than depending on row order.
type CoverageRow struct {
	JurisdictionName string
	JurisdictionSlug string
	JurisdictionKind string // country, state or city
	Status           map[string]CoverageCell
}

// AuthorCoverage reports which core topics each place has and does not have.
//
// Core topics are the standard set every state is expected to carry (ADR-028
// D2), so a missing one is a gap rather than a choice, or a thin-page skip.
// A rules page counts under the situation topic it answers, so one cell
// says whether the topic is done in that place, not whether one page is.
//
// Every state is included, including those with no pages at all: a state with
// no content is exactly the one nothing else on this dashboard would surface.
// The country row (United States, first) is in the matrix because national
// guides are the fallback every uncovered location resolves to (ADR-009), so a
// hole in the national row is a hole for everyone.
//
// Cities come last. A city gets a topic only where city law adds to its
// state's, so an empty city cell is not a gap; the rows are there to show
// what the cities do have.
//
// This reports topics, not layouts. resource-directory is a topic — the
// subject "where to get help", shown as "Local Help" — and is not the same
// thing as the 'directory' page_kind, which is a layout any topic may use.
func (pg *PG) AuthorCoverage(ctx context.Context) ([]CoverageRow, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT j.name, j.slug, j.kind, t.slug,
		       COALESCE(MAX(CASE p.status WHEN 'draft' THEN 2 WHEN 'published' THEN 1 ELSE 0 END), 0),
		       COUNT(p.id), MIN(p.id)
		FROM jurisdictions j
		CROSS JOIN topics t
		LEFT JOIN topics rt ON rt.rules_for = t.id
		LEFT JOIN playbooks p
		       ON p.jurisdiction_id = j.id
		      AND (p.topic_id = t.id OR p.topic_id = rt.id)
		      AND p.status IN ('published', 'draft')
		      AND p.language = ANY($1)
		WHERE j.kind IN ('country', 'state', 'city') AND t.is_core
		GROUP BY j.kind, j.name, j.slug, t.slug
		ORDER BY (j.kind = 'country') DESC, (j.kind = 'state') DESC, j.name, t.slug`,
		ContentLanguages)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CoverageRow
	bySlug := map[string]int{}
	for rows.Next() {
		var name, slug, kind, topicSlug string
		var state, pages int
		var minID *int64
		if err := rows.Scan(&name, &slug, &kind, &topicSlug, &state, &pages, &minID); err != nil {
			return nil, err
		}
		i, ok := bySlug[slug]
		if !ok {
			i = len(out)
			bySlug[slug] = i
			out = append(out, CoverageRow{
				JurisdictionName: name,
				JurisdictionSlug: slug,
				JurisdictionKind: kind,
				Status:           map[string]CoverageCell{},
			})
		}
		cell := CoverageCell{Pages: pages}
		switch state {
		case 2:
			cell.State = "draft"
		case 1:
			cell.State = "published"
		}
		if pages == 1 && minID != nil {
			cell.PageID = *minID
		}
		out[i].Status[topicSlug] = cell
	}
	return out, rows.Err()
}
