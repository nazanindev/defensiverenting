package store

import "context"

// "Did this page help?" (2026-10-04). Readers answer yes or no at the foot of
// a page, and each answer adds one to that page's count for the day. Nothing
// about the reader is kept. A page is its place, topic and language, so the
// count carries across republished revisions.

// RecordHelpful adds one answer to the published page whose playbook id is
// given. It reports false, and counts nothing, when no published page has
// that id: the id arrives from the reader's browser and may be anything.
func (pg *PG) RecordHelpful(ctx context.Context, playbookID int64, helpful bool) (bool, error) {
	yes, no := 0, 1
	if helpful {
		yes, no = 1, 0
	}
	tag, err := pg.pool.Exec(ctx, `
		INSERT INTO page_helpful (jurisdiction_id, topic_id, language, yes, no)
		SELECT jurisdiction_id, topic_id, language, $2, $3
		FROM playbooks WHERE id = $1 AND status = 'published'
		ON CONFLICT (jurisdiction_id, topic_id, language, day)
		DO UPDATE SET yes = page_helpful.yes + EXCLUDED.yes, no = page_helpful.no + EXCLUDED.no`,
		playbookID, yes, no)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// PageHelpful returns a page's yes and no totals over every day counted.
func (pg *PG) PageHelpful(ctx context.Context, jurisdictionID, topicID int64, language string) (yes, no int, err error) {
	err = pg.pool.QueryRow(ctx, `
		SELECT coalesce(sum(yes), 0), coalesce(sum(no), 0) FROM page_helpful
		WHERE jurisdiction_id = $1 AND topic_id = $2 AND language = $3`,
		jurisdictionID, topicID, language).Scan(&yes, &no)
	return yes, no, err
}
