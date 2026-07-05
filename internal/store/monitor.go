package store

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// ListCitationsForCheck returns every (source, verbatim quote) pair cited from a
// non-editorial source, for the checker to confirm each quote still appears at
// the URL. Rows are ordered by source id so callers can group by source.
func (pg *PG) ListCitationsForCheck(ctx context.Context) ([]CitationCheckRow, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT s.id, s.url, s.publisher, c.quote
		FROM citations c
		JOIN sources s ON s.id = c.source_id
		WHERE s.kind <> 'editorial' AND btrim(c.quote) <> ''
		ORDER BY s.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CitationCheckRow
	for rows.Next() {
		var r CitationCheckRow
		if err := rows.Scan(&r.SourceID, &r.URL, &r.Publisher, &r.Quote); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// MarkSourceReviewed stamps retrieved_at and, when a cited quote went missing,
// sets flagged_at. An existing flag is left intact when changed is false — only
// a dismiss clears it.
func (pg *PG) MarkSourceReviewed(ctx context.Context, id int64, changed bool) error {
	_, err := pg.pool.Exec(ctx, `
		UPDATE sources
		SET retrieved_at = NOW(),
		    flagged_at   = CASE WHEN $2 THEN NOW() ELSE flagged_at END
		WHERE id = $1`, id, changed)
	return err
}

// ListFlaggedSources returns sources the checker flagged (a cited quote no longer
// appears), newest first.
func (pg *PG) ListFlaggedSources(ctx context.Context) ([]Source, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT id, url, publisher, jurisdiction_id, kind, retrieved_at, content_hash, flagged_at
		FROM sources
		WHERE flagged_at IS NOT NULL
		ORDER BY flagged_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSources(rows)
}

// DismissSourceFlag clears a source's change flag after the author reviews it.
func (pg *PG) DismissSourceFlag(ctx context.Context, id int64) error {
	_, err := pg.pool.Exec(ctx, `UPDATE sources SET flagged_at = NULL WHERE id = $1`, id)
	return err
}

func scanSources(rows pgx.Rows) ([]Source, error) {
	var out []Source
	for rows.Next() {
		var s Source
		if err := rows.Scan(&s.ID, &s.URL, &s.Publisher, &s.JurisdictionID, &s.Kind,
			&s.RetrievedAt, &s.ContentHash, &s.FlaggedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
