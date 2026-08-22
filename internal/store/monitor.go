package store

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
)

// ListCitationsForCheck returns every (source, verbatim quote) pair cited from a
// non-editorial source, for the checker to confirm each quote still appears at
// the URL. Rows are ordered by source id so callers can group by source.
//
// Only citations reachable from a playbook count. Saving a playbook replaces its
// rows in playbook_statements but never deletes the statements themselves, so
// every re-save leaves its previous statements — and their citations — behind in
// the tables. Without the EXISTS filter the checker re-fetches sources that no
// page cites any more, and can flag a source on the strength of a quote that
// nothing published depends on.
func (pg *PG) ListCitationsForCheck(ctx context.Context) ([]CitationCheckRow, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT s.id, s.url, s.publisher, c.quote
		FROM citations c
		JOIN sources s ON s.id = c.source_id
		WHERE s.kind <> 'editorial' AND btrim(c.quote) <> ''
		  AND EXISTS (SELECT 1 FROM playbook_statements ps WHERE ps.statement_id = c.statement_id)
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

// CountUncheckableCitations returns how many citations ListCitationsForCheck
// silently drops because they carry no verbatim quote.
//
// Citations written before the quote column existed (migration 000008) default
// to the empty string, and a check for the empty string matches any page, so
// the filter in ListCitationsForCheck excludes them rather than passing them
// falsely. The exclusion is correct; being quiet about it was not. A run that
// examined nothing reported the same "0 flagged" as a run that examined
// everything, which is how 17 of 19 published pages went a month without ever
// being checked.
//
// Editorial sources are excluded here as they are there: they cite no external
// text by design (ADR-003), so they are out of scope rather than missing.
//
// The EXISTS filter matches ListCitationsForCheck for the reason given there:
// orphaned statements accumulate on every save, and a count that included them
// would overstate the gap on live pages — reporting a number that does not mean
// what it says, which is the failure this count exists to end.
func (pg *PG) CountUncheckableCitations(ctx context.Context) (int, error) {
	var n int
	err := pg.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM citations c
		JOIN sources s ON s.id = c.source_id
		WHERE s.kind <> 'editorial' AND btrim(c.quote) = ''
		  AND EXISTS (SELECT 1 FROM playbook_statements ps WHERE ps.statement_id = c.statement_id)`).Scan(&n)
	return n, err
}

// CitationQuoteExists reports whether this exact (source URL, quote) pair is
// already stored.
//
// It answers "has this quote been verified before?" for the authoring form,
// which re-checks a pasted quote against the live source. Re-fetching every
// source on every save would make saving slow, and would fail a save outright
// when a source is temporarily unreachable — including sources that block the
// fetcher permanently, such as the Massachusetts sanitary code PDF. A quote
// that is already stored got there through a path that verified it, so an
// unchanged quote needs no second look; only new or edited text is fetched.
//
// Matching on the pair rather than on a row id keeps this correct when
// statements are reordered, added, or removed between edits.
func (pg *PG) CitationQuoteExists(ctx context.Context, url, quote string) (bool, error) {
	if strings.TrimSpace(quote) == "" {
		return false, nil
	}
	var exists bool
	err := pg.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM citations c
			JOIN sources s ON s.id = c.source_id
			WHERE s.url = $1 AND c.quote = $2
		)`, url, quote).Scan(&exists)
	return exists, err
}

// MarkSourceReviewed stamps retrieved_at and last_checked_at and, when a cited
// quote went missing, sets flagged_at. An existing flag is left intact when
// changed is false — only a dismiss clears it. Only the checker calls this, so
// last_checked_at means exactly "the checker fetched this source and examined
// its quotes then" — unlike retrieved_at, which UpsertSource bumps on every
// save without fetching.
func (pg *PG) MarkSourceReviewed(ctx context.Context, id int64, changed bool) error {
	_, err := pg.pool.Exec(ctx, `
		UPDATE sources
		SET retrieved_at    = NOW(),
		    last_checked_at = NOW(),
		    flagged_at      = CASE WHEN $2 THEN NOW() ELSE flagged_at END
		WHERE id = $1`, id, changed)
	return err
}

// MarkQuotesChecked stamps checked_at on every citation of this source whose
// quote the checker just confirmed at the URL. Matching on (source_id, quote)
// rather than citation ids stamps every statement carrying the same confirmed
// text — including orphaned rows, whose stamps insertCitationSQL inherits when
// the same quote is saved again. Quotes that went missing are absent from the
// list and keep the stamp from the run that last actually saw them.
func (pg *PG) MarkQuotesChecked(ctx context.Context, sourceID int64, quotes []string) error {
	if len(quotes) == 0 {
		return nil
	}
	_, err := pg.pool.Exec(ctx, `
		UPDATE citations SET checked_at = NOW(), checked_by = $3
		WHERE source_id = $1 AND quote = ANY($2)`, sourceID, quotes, ActorSourceCheck)
	return err
}

// ListFlaggedSources returns sources the checker flagged (a cited quote no longer
// appears), newest first.
func (pg *PG) ListFlaggedSources(ctx context.Context) ([]Source, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT id, url, publisher, jurisdiction_id, kind, retrieved_at, content_hash, flagged_at, last_checked_at
		FROM sources
		WHERE flagged_at IS NOT NULL
		ORDER BY flagged_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSources(rows)
}

// ListUnusedSources returns sources no page on the site cites: no citation
// from any statement still linked to a playbook. Saves upsert a source row the
// moment a URL is typed and never remove one, and re-saves orphan statements
// (see ListCitationsForCheck), so rows drift into disuse silently. An unused
// source costs a re-fetch on every check run and clutters the import picker,
// and nothing surfaced them until this. The site's own editorial source is
// permanent plumbing, not clutter, so it is excluded.
func (pg *PG) ListUnusedSources(ctx context.Context) ([]Source, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT id, url, publisher, jurisdiction_id, kind, retrieved_at, content_hash, flagged_at, last_checked_at
		FROM sources s
		WHERE s.url <> '/editorial'
		  AND NOT EXISTS (
			SELECT 1 FROM citations c
			JOIN playbook_statements ps ON ps.statement_id = c.statement_id
			WHERE c.source_id = s.id)
		ORDER BY s.publisher, s.id`)
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
			&s.RetrievedAt, &s.ContentHash, &s.FlaggedAt, &s.LastCheckedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
