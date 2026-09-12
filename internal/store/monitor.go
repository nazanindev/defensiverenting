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
// Only citations reachable from a playbook in an active content language
// count (ADR-015 D3): a deferred translation cites what its English page
// cites, so checking it would fetch the same sources for a page nobody can
// act on. Saving a playbook replaces its
// rows in playbook_statements but never deletes the statements themselves, so
// every re-save leaves its previous statements — and their citations — behind in
// the tables. Without the EXISTS filter the checker re-fetches sources that no
// page cites any more, and can flag a source on the strength of a quote that
// nothing published depends on.
func (pg *PG) ListCitationsForCheck(ctx context.Context) ([]CitationCheckRow, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT s.id, s.url, s.publisher, c.quote, st.key::text, c.locator,
		       c.checked_extractor, c.checked_hash, c.checked_context
		FROM citations c
		JOIN sources s ON s.id = c.source_id
		JOIN statements st ON st.id = c.statement_id
		WHERE s.kind <> 'editorial' AND btrim(c.quote) <> ''
		  AND EXISTS (SELECT 1 FROM playbook_statements ps JOIN playbooks pb ON pb.id = ps.playbook_id
		              WHERE ps.statement_id = c.statement_id AND pb.language = ANY($1)
		                AND pb.status IN ('draft', 'published'))
		ORDER BY s.id`, ContentLanguages)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CitationCheckRow
	for rows.Next() {
		var r CitationCheckRow
		if err := rows.Scan(&r.SourceID, &r.URL, &r.Publisher, &r.Quote, &r.StatementKey, &r.Locator,
			&r.CheckedExtractor, &r.CheckedHash, &r.CheckedContext); err != nil {
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
		  AND EXISTS (SELECT 1 FROM playbook_statements ps JOIN playbooks pb ON pb.id = ps.playbook_id
		              WHERE ps.statement_id = c.statement_id AND pb.language = ANY($1))`, ContentLanguages).Scan(&n)
	return n, err
}

// CitationQuoteExists reports whether this exact (source URL, quote) pair is
// already stored with a confirmation behind it.
//
// It answers "has this quote been verified before?" for the authoring form,
// which re-checks a pasted quote against the live source. Re-fetching every
// source on every save would make saving slow — including sources that block
// the fetcher permanently, such as the Massachusetts sanitary code PDF — so an
// unchanged, already-confirmed quote needs no second look; only new or edited
// text is fetched.
//
// The confirmation condition is load-bearing under ADR-013: a save now stores
// a quote whether or not it could be checked, so "it is stored" no longer
// implies "it was verified". Matching stored-but-unconfirmed text here would
// launder it — the first save records it unverified, and every later check
// would call it known-good. checked_at IS NOT NULL (a fetch found it, or an
// attestation stamped it) is what actually says someone looked; the
// manually_verified arm covers rows attested before checked_at existed
// (migration 000017), which were never stamped.
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
			  AND (c.checked_at IS NOT NULL OR c.manually_verified)
		)`, url, quote).Scan(&exists)
	return exists, err
}

// MarkSourceChecked stamps retrieved_at and last_checked_at, and records how
// the text was obtained. Only the checker calls this, so last_checked_at
// means exactly "the checker read this source and examined its quotes then"
// — unlike retrieved_at, which UpsertSource bumps on every save without
// fetching. A quote that went missing is not recorded here: it is filed as a
// source-drift proposal against the statement citing it (ADR-014 D4).
func (pg *PG) MarkSourceChecked(ctx context.Context, id int64, note string) error {
	_, err := pg.pool.Exec(ctx, `
		UPDATE sources
		SET retrieved_at    = NOW(),
		    last_checked_at = NOW(),
		    last_fetch_at   = NOW(),
		    last_fetch_note = $2
		WHERE id = $1`, id, note)
	return err
}

// MarkSourceUnreadable records a check that reached for the source and got
// nothing it could examine quotes against: a failed fetch, or a thin page.
// last_checked_at is left alone, because nothing was checked; the note says
// what happened so the issue list can tell the reviewer.
func (pg *PG) MarkSourceUnreadable(ctx context.Context, id int64, note string) error {
	_, err := pg.pool.Exec(ctx, `
		UPDATE sources
		SET last_fetch_at   = NOW(),
		    last_fetch_note = $2
		WHERE id = $1`, id, note)
	return err
}

// MarkQuotesChecked stamps checked_at and the confirmation receipt on every
// citation of this source whose quote the checker just found at the URL.
// Matching on (source_id, quote) rather than citation ids stamps every
// statement carrying the same confirmed text — including orphaned rows, whose
// stamps insertCitationSQL inherits when the same quote is saved again.
// Quotes that went missing are absent from the list and keep the stamp from
// the run that last actually saw them.
func (pg *PG) MarkQuotesChecked(ctx context.Context, sourceID int64, fetch CheckReceipt, quotes []QuoteConfirmation) error {
	for _, q := range quotes {
		if _, err := pg.pool.Exec(ctx, `
			UPDATE citations
			SET checked_at = NOW(), checked_by = $3,
			    checked_via = $4, checked_extractor = $5, checked_hash = $6, checked_context = $7
			WHERE source_id = $1 AND quote = $2`,
			sourceID, q.Quote, ActorSourceCheck, fetch.Via, fetch.Extractor, fetch.Hash, q.Context); err != nil {
			return err
		}
	}
	return nil
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
		SELECT id, url, publisher, jurisdiction_id, kind, retrieved_at, content_hash, last_checked_at
		FROM sources s
		WHERE `+unusedSourceSQL+`
		ORDER BY s.publisher, s.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSources(rows)
}

func scanSources(rows pgx.Rows) ([]Source, error) {
	var out []Source
	for rows.Next() {
		var s Source
		if err := rows.Scan(&s.ID, &s.URL, &s.Publisher, &s.JurisdictionID, &s.Kind,
			&s.RetrievedAt, &s.ContentHash, &s.LastCheckedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
