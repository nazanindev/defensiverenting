package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"

	"github.com/jackc/pgx/v5"
)

// "Did this page help?" (2026-10-04). Readers answer yes or no at the foot of
// a page, and each answer adds one to that page's count for the day. A page
// is its place, topic and language, so the count carries across republished
// revisions.
//
// The counts are only worth reading if one person cannot move them, so a
// reader counts once per page per day and at most HelpfulDailyCap times a day
// across all pages. A reader is a hash of their address and a salt made for
// the day. The day's salt and hashes are deleted when the next day starts,
// so nothing can tie an answer to an address after that. The address itself
// is never stored.

// HelpfulDailyCap is how many pages one reader can answer for in a day. A
// person reading closely may answer a dozen pages; a script answering
// hundreds is not a reader.
const HelpfulDailyCap = 20

// RecordHelpful adds one answer from client (the reader's IP address) to the
// published page whose playbook id is given. It reports false, and counts
// nothing, when no published page has that id, when this reader already
// answered for this page today, when they are over the daily cap, or when
// there is no client to tell readers apart by.
func (pg *PG) RecordHelpful(ctx context.Context, playbookID int64, helpful bool, client string) (bool, error) {
	if client == "" {
		return false, nil
	}
	yes, no := 0, 1
	if helpful {
		yes, no = 1, 0
	}
	newSalt := make([]byte, 32)
	if _, err := rand.Read(newSalt); err != nil {
		return false, err
	}

	counted := false
	err := pgx.BeginTxFunc(ctx, pg.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		// Forget yesterday before anything else, so no old hash survives a day.
		if _, err := tx.Exec(ctx, `DELETE FROM helpful_seen WHERE day < CURRENT_DATE`); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM helpful_salt WHERE day < CURRENT_DATE`); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO helpful_salt (day, salt) VALUES (CURRENT_DATE, $1) ON CONFLICT (day) DO NOTHING`, newSalt); err != nil {
			return err
		}
		var salt []byte
		if err := tx.QueryRow(ctx, `SELECT salt FROM helpful_salt WHERE day = CURRENT_DATE`).Scan(&salt); err != nil {
			return err
		}
		sum := sha256.Sum256(append(salt, client...))
		reader := sum[:]

		var jID, tID int64
		var lang string
		err := tx.QueryRow(ctx, `SELECT jurisdiction_id, topic_id, language FROM playbooks WHERE id = $1 AND status = 'published'`,
			playbookID).Scan(&jID, &tID, &lang)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}

		var today int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM helpful_seen WHERE day = CURRENT_DATE AND reader = $1`, reader).Scan(&today); err != nil {
			return err
		}
		if today >= HelpfulDailyCap {
			return nil
		}
		tag, err := tx.Exec(ctx, `
			INSERT INTO helpful_seen (day, reader, jurisdiction_id, topic_id, language)
			VALUES (CURRENT_DATE, $1, $2, $3, $4) ON CONFLICT DO NOTHING`, reader, jID, tID, lang)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return nil // already answered for this page today
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO page_helpful (jurisdiction_id, topic_id, language, yes, no)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (jurisdiction_id, topic_id, language, day)
			DO UPDATE SET yes = page_helpful.yes + EXCLUDED.yes, no = page_helpful.no + EXCLUDED.no`,
			jID, tID, lang, yes, no); err != nil {
			return err
		}
		counted = true
		return nil
	})
	return counted, err
}

// PageHelpful returns a page's yes and no totals over every day counted.
func (pg *PG) PageHelpful(ctx context.Context, jurisdictionID, topicID int64, language string) (yes, no int, err error) {
	err = pg.pool.QueryRow(ctx, `
		SELECT coalesce(sum(yes), 0), coalesce(sum(no), 0) FROM page_helpful
		WHERE jurisdiction_id = $1 AND topic_id = $2 AND language = $3`,
		jurisdictionID, topicID, language).Scan(&yes, &no)
	return yes, no, err
}
