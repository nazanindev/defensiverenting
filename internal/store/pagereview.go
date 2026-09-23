package store

import (
	"context"
	"fmt"
)

// ConceptGaps lists the concept tags on the published page one level up
// (the state page for a city, the national page for a state), same topic
// and language, that this page carries on none of its statements (ADR-025
// D2). It is the one input to page review that looks outside the page, and
// it is computed here so the reviewer never has to read the other page: a
// gap is a slug for the reviewer to judge (a missing claim, a missing tag,
// or not relevant here), never text to copy.
func (pg *PG) ConceptGaps(ctx context.Context, playbookID int64) ([]string, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT DISTINCT co.slug
		FROM playbooks pb
		JOIN jurisdictions j ON j.id = pb.jurisdiction_id
		JOIN playbooks up ON up.jurisdiction_id = j.parent_id AND up.topic_id = pb.topic_id
		                 AND up.language = pb.language AND up.status = 'published'
		JOIN playbook_statements ups ON ups.playbook_id = up.id
		JOIN statements us ON us.id = ups.statement_id
		JOIN concepts co ON co.id = us.concept_id
		WHERE pb.id = $1
		  AND NOT EXISTS (
		    SELECT 1 FROM playbook_statements ps JOIN statements s ON s.id = ps.statement_id
		    WHERE ps.playbook_id = pb.id AND s.concept_id = us.concept_id)
		ORDER BY co.slug`, playbookID)
	if err != nil {
		return nil, fmt.Errorf("concept gaps: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
