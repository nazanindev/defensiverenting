package store

import (
	"context"
	"sort"
)

// Concepts — ADR-011. The registry itself is written only by migration; these
// queries read it and answer the two questions it exists for: "who localized
// this claim" (D4's links) and "who hasn't yet" (D3's coverage).

func (pg *PG) ListConcepts(ctx context.Context) ([]Concept, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT c.id, c.slug, c.name, c.topic_id, t.slug
		FROM concepts c
		JOIN topics t ON t.id = c.topic_id
		ORDER BY t.slug, c.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Concept
	for rows.Next() {
		var c Concept
		if err := rows.Scan(&c.ID, &c.Slug, &c.Name, &c.TopicID, &c.TopicSlug); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ConceptCoverage computes the resolution-aware matrix (ADR-011 D3): for each
// concept, which covered places have their own tagged statement (localized),
// which are covered only because the national page states the claim (generic
// — the work queue), and which have no statement anywhere up the chain
// (missing). Drafts count as coverage for the same reason they do in the
// topic matrix: a statement sitting in review is not a gap to re-research.
// English only — translations mirror English content (ADR-007), so counting
// both would double every place.
func (pg *PG) ConceptCoverage(ctx context.Context) ([]ConceptCoverageRow, error) {
	concepts, err := pg.ListConcepts(ctx)
	if err != nil {
		return nil, err
	}

	// Every (place, concept) pair with a tagged statement on a live page.
	tagRows, err := pg.pool.Query(ctx, `
		SELECT DISTINCT pb.jurisdiction_id, j.kind, s.concept_id
		FROM playbooks pb
		JOIN playbook_statements ps ON ps.playbook_id = pb.id
		JOIN statements s ON s.id = ps.statement_id
		JOIN jurisdictions j ON j.id = pb.jurisdiction_id
		WHERE s.concept_id IS NOT NULL
		  AND pb.status IN ('published', 'draft') AND pb.language = 'en'`)
	if err != nil {
		return nil, err
	}
	defer tagRows.Close()
	type jc struct {
		j int64
		c int64
	}
	tagged := map[jc]bool{}
	nationalHas := map[int64]bool{}
	for tagRows.Next() {
		var jID, cID int64
		var kind string
		if err := tagRows.Scan(&jID, &kind, &cID); err != nil {
			return nil, err
		}
		if kind == "country" {
			nationalHas[cID] = true
		} else {
			tagged[jc{jID, cID}] = true
		}
	}
	if err := tagRows.Err(); err != nil {
		return nil, err
	}

	// Every non-national place with a live page, and per topic.
	pageRows, err := pg.pool.Query(ctx, `
		SELECT DISTINCT pb.jurisdiction_id, j.name, pb.topic_id
		FROM playbooks pb
		JOIN jurisdictions j ON j.id = pb.jurisdiction_id
		WHERE pb.status IN ('published', 'draft') AND pb.language = 'en'
		  AND j.kind IN ('city', 'state')`)
	if err != nil {
		return nil, err
	}
	defer pageRows.Close()
	nameOf := map[int64]string{}
	byTopic := map[int64]map[int64]bool{} // topic id -> place ids
	anyPage := map[int64]bool{}
	for pageRows.Next() {
		var jID, tID int64
		var name string
		if err := pageRows.Scan(&jID, &name, &tID); err != nil {
			return nil, err
		}
		nameOf[jID] = name
		anyPage[jID] = true
		if byTopic[tID] == nil {
			byTopic[tID] = map[int64]bool{}
		}
		byTopic[tID][jID] = true
	}
	if err := pageRows.Err(); err != nil {
		return nil, err
	}

	out := make([]ConceptCoverageRow, 0, len(concepts))
	for _, c := range concepts {
		row := ConceptCoverageRow{Concept: c, National: nationalHas[c.ID]}
		// Cross-cutting concepts can be localized on any of a place's pages,
		// so every covered place is in scope; a topic-owned concept is only
		// expected where its topic's page exists.
		places := byTopic[c.TopicID]
		if c.TopicSlug == "renting-fundamentals" {
			places = anyPage
		}
		for jID := range places {
			switch {
			case tagged[jc{jID, c.ID}]:
				row.Localized = append(row.Localized, nameOf[jID])
			case row.National:
				row.GenericOnly = append(row.GenericOnly, nameOf[jID])
			default:
				row.Missing = append(row.Missing, nameOf[jID])
			}
		}
		sort.Strings(row.Localized)
		sort.Strings(row.GenericOnly)
		sort.Strings(row.Missing)
		out = append(out, row)
	}
	return out, nil
}

// ListSourceUsage returns each source's citation structure across live pages
// (ADR-011 D6): distinct statements, distinct pages, and the distinct locators
// — so three sections of one statute page read as one source cited three ways,
// not as apparent duplicates.
func (pg *PG) ListSourceUsage(ctx context.Context) ([]SourceUsage, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT c.source_id,
		       count(DISTINCT c.statement_id),
		       count(DISTINCT ps.playbook_id),
		       COALESCE(array_agg(DISTINCT c.locator) FILTER (WHERE btrim(c.locator) <> ''), '{}')
		FROM citations c
		JOIN playbook_statements ps ON ps.statement_id = c.statement_id
		GROUP BY c.source_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SourceUsage
	for rows.Next() {
		var u SourceUsage
		if err := rows.Scan(&u.SourceID, &u.Statements, &u.Pages, &u.Locators); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// ConceptHubTopics maps each concept to the topic hub that answers "where is
// this claim localized": the topic whose published, non-national pages carry
// tagged statements for it, in that language. Usually that is the concept's
// own topic, but a cross-cutting concept tagged on a national fundamentals
// page has its local instances on other topics' pages entirely — retaliation
// lives on eviction and repairs pages — and linking the fundamentals reader
// to a fundamentals hub no city will ever have would be a link to nowhere.
// When instances span topics the one covering the most places wins, with the
// topic slug as a deterministic tie-break. A concept with no published local
// instance is absent from the map, and its statements get no link — the
// original D4 rule, kept: never render an empty shell of the feature.
func (pg *PG) ConceptHubTopics(ctx context.Context, language string) (map[string]string, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT co.slug, t.slug, count(DISTINCT pb.jurisdiction_id) AS places
		FROM statements s
		JOIN concepts co ON co.id = s.concept_id
		JOIN playbook_statements ps ON ps.statement_id = s.id
		JOIN playbooks pb ON pb.id = ps.playbook_id
		JOIN topics t ON t.id = pb.topic_id
		JOIN jurisdictions j ON j.id = pb.jurisdiction_id
		WHERE pb.status = 'published' AND pb.language = $1 AND j.kind <> 'country'
		GROUP BY co.slug, t.slug
		ORDER BY co.slug, places DESC, t.slug`, language)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var conceptSlug, topicSlug string
		var places int
		if err := rows.Scan(&conceptSlug, &topicSlug, &places); err != nil {
			return nil, err
		}
		if _, seen := out[conceptSlug]; !seen {
			out[conceptSlug] = topicSlug
		}
	}
	return out, rows.Err()
}
