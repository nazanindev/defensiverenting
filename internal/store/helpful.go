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
	counted := false
	err := pgx.BeginTxFunc(ctx, pg.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		reader, err := todaysReader(ctx, tx, client)
		if err != nil {
			return err
		}

		var jID, tID int64
		var lang string
		err = tx.QueryRow(ctx, `SELECT jurisdiction_id, topic_id, language FROM playbooks WHERE id = $1 AND status = 'published'`,
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

// todaysReader returns the reader's hash for today: their address with
// today's salt. It first deletes every earlier day's salt and hashes, for
// both the helpful answers and the click counts, so no hash outlives its day.
func todaysReader(ctx context.Context, tx pgx.Tx, client string) ([]byte, error) {
	for _, purge := range []string{
		`DELETE FROM helpful_seen WHERE day < CURRENT_DATE`,
		`DELETE FROM click_seen WHERE day < CURRENT_DATE`,
		`DELETE FROM helpful_salt WHERE day < CURRENT_DATE`,
	} {
		if _, err := tx.Exec(ctx, purge); err != nil {
			return nil, err
		}
	}
	newSalt := make([]byte, 32)
	if _, err := rand.Read(newSalt); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO helpful_salt (day, salt) VALUES (CURRENT_DATE, $1) ON CONFLICT (day) DO NOTHING`, newSalt); err != nil {
		return nil, err
	}
	var salt []byte
	if err := tx.QueryRow(ctx, `SELECT salt FROM helpful_salt WHERE day = CURRENT_DATE`).Scan(&salt); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(append(salt, client...))
	return sum[:], nil
}

// ClickDailyCap is how many sources one reader's clicks count for in a day.
const ClickDailyCap = 50

// RecordClick counts one click or phone tap on a source cited by a published
// page (ADR-029 D6). Like RecordHelpful it reports false, and counts nothing,
// for an unknown source, a repeat from the same reader today, a reader over
// the daily cap, or no client.
func (pg *PG) RecordClick(ctx context.Context, sourceID int64, client string) (bool, error) {
	if client == "" {
		return false, nil
	}
	counted := false
	err := pgx.BeginTxFunc(ctx, pg.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		reader, err := todaysReader(ctx, tx, client)
		if err != nil {
			return err
		}
		var live bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM citations c
			               JOIN playbook_statements ps ON ps.statement_id = c.statement_id
			               JOIN playbooks pb ON pb.id = ps.playbook_id
			               WHERE c.source_id = $1 AND pb.status = 'published')`, sourceID).Scan(&live); err != nil {
			return err
		}
		if !live {
			return nil
		}
		var today int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM click_seen WHERE day = CURRENT_DATE AND reader = $1`, reader).Scan(&today); err != nil {
			return err
		}
		if today >= ClickDailyCap {
			return nil
		}
		tag, err := tx.Exec(ctx, `
			INSERT INTO click_seen (day, reader, source_id) VALUES (CURRENT_DATE, $1, $2)
			ON CONFLICT DO NOTHING`, reader, sourceID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO source_clicks (source_id, clicks) VALUES ($1, 1)
			ON CONFLICT (source_id, day) DO UPDATE SET clicks = source_clicks.clicks + 1`, sourceID); err != nil {
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
