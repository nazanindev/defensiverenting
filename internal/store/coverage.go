package store

import "context"

// CoverageRow is one city's standing against every core topic.
//
// Status maps a topic slug to "published", "draft", or "" when the city has
// neither. The template reads it by slug so the column order is set once, by
// the caller's topic list, rather than depending on row order.
type CoverageRow struct {
	JurisdictionName string
	JurisdictionSlug string
	Status           map[string]string
}

// AuthorCoverage reports which core topics each city has and does not have.
//
// Core topics are the set every city is expected to carry (ADR-005 D5), so a
// missing one is a gap rather than a choice.
//
// This reports topics, not layouts. resource-directory is a topic — the subject
// "where to get help", shown as "Local Help" — and is not the same thing as the
// 'directory' page_kind, which is a layout any topic may use. Migration 000005
// put the directory layout on a cant-pay-rent page, so the two genuinely come
// apart, and a column here saying a city has Local Help says nothing about how
// that page is laid out.
//
// A draft is deliberately distinguished from a published page. A city with a
// directory sitting in review is not missing one, and counting it as missing
// would send an author to redraft a page that is already waiting for them.
//
// Every city is included, including those with no pages at all. Those rows look
// like noise but are the opposite: a city seeded with no content is exactly the
// one nothing else on this dashboard would surface.
//
// The country row (United States, first) is in the matrix for the same reason
// the cities are: national guides are the fallback every uncovered location
// resolves to (ADR-009), so a hole in the national row is a hole for everyone.
func (pg *PG) AuthorCoverage(ctx context.Context) ([]CoverageRow, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT j.name, j.slug, t.slug,
		       COALESCE(MAX(CASE p.status WHEN 'published' THEN 2 WHEN 'draft' THEN 1 ELSE 0 END), 0)
		FROM jurisdictions j
		CROSS JOIN topics t
		LEFT JOIN playbooks p
		       ON p.jurisdiction_id = j.id
		      AND p.topic_id = t.id
		      AND p.status IN ('published', 'draft')
		WHERE j.kind IN ('city', 'country') AND t.is_core
		GROUP BY j.kind, j.name, j.slug, t.slug
		ORDER BY (j.kind <> 'country'), j.name, t.slug`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CoverageRow
	bySlug := map[string]int{}
	for rows.Next() {
		var cityName, citySlug, topicSlug string
		var state int
		if err := rows.Scan(&cityName, &citySlug, &topicSlug, &state); err != nil {
			return nil, err
		}
		i, ok := bySlug[citySlug]
		if !ok {
			i = len(out)
			bySlug[citySlug] = i
			out = append(out, CoverageRow{
				JurisdictionName: cityName,
				JurisdictionSlug: citySlug,
				Status:           map[string]string{},
			})
		}
		switch state {
		case 2:
			out[i].Status[topicSlug] = "published"
		case 1:
			out[i].Status[topicSlug] = "draft"
		default:
			out[i].Status[topicSlug] = ""
		}
	}
	return out, rows.Err()
}
