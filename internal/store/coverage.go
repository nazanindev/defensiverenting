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
	Published        int
	Draft            int
	Missing          int
}

// AuthorCoverage reports which core topics each city has and does not have.
//
// Core topics are the set every city is expected to carry (ADR-005 D5), so a
// missing one is a gap rather than a choice. resource-directory is among them,
// which makes "which cities still have no directory" a read of one column here
// rather than a question needing its own query.
//
// A draft is deliberately distinguished from a published page. A city with a
// directory sitting in review is not missing one, and counting it as missing
// would send an author to redraft a page that is already waiting for them.
//
// Every city is included, including those with no pages at all. Those rows look
// like noise but are the opposite: a city seeded with no content is exactly the
// one nothing else on this dashboard would surface.
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
		WHERE j.kind = 'city' AND t.is_core
		GROUP BY j.name, j.slug, t.slug
		ORDER BY j.name, t.slug`)
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
			out[i].Published++
		case 1:
			out[i].Status[topicSlug] = "draft"
			out[i].Draft++
		default:
			out[i].Status[topicSlug] = ""
			out[i].Missing++
		}
	}
	return out, rows.Err()
}
