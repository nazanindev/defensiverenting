package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Reader accounts (ADR-017). Every method takes a token *hash*; hashing the
// secret is the handler's job, and this layer never sees a raw token.

// Account is what a valid session resolves to: who, and what they have told
// us. Location is the chosen jurisdiction, or a zero Jurisdiction when none
// is set.
type Account struct {
	UserID   int64
	Email    string
	Location Jurisdiction
}

// CreateLoginToken records a sign-in link for email, expiring at expires.
func (pg *PG) CreateLoginToken(ctx context.Context, tokenHash, email string, expires time.Time) error {
	_, err := pg.pool.Exec(ctx, `
		INSERT INTO login_tokens (token_hash, email, expires_at) VALUES ($1, $2, $3)`,
		tokenHash, email, expires)
	return err
}

// CountRecentLoginTokens reports how many links have been issued to email
// since the given time, so the handler can stop mailing an address that is
// being hammered.
func (pg *PG) CountRecentLoginTokens(ctx context.Context, email string, since time.Time) (int, error) {
	var n int
	err := pg.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM login_tokens WHERE email = $1 AND created_at >= $2`,
		email, since).Scan(&n)
	return n, err
}

// ConsumeLoginToken marks a link used and returns the user it signs in,
// creating the user row on first use. ErrNotFound covers every way a link can
// be bad: unknown, expired, or already used. The handler shows one message
// for all three, so the store need not tell them apart.
//
// The update and the user upsert run in one transaction so a link used twice
// at the same instant signs in exactly once.
func (pg *PG) ConsumeLoginToken(ctx context.Context, tokenHash string, now time.Time) (int64, error) {
	tx, err := pg.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var email string
	err = tx.QueryRow(ctx, `
		UPDATE login_tokens SET used_at = $2
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > $2
		RETURNING email`, tokenHash, now).Scan(&email)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("consume token: %w", err)
	}

	var userID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO users (email) VALUES ($1)
		ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
		RETURNING id`, email).Scan(&userID); err != nil {
		return 0, fmt.Errorf("upsert user: %w", err)
	}

	// Housekeeping on the same trip: dead links have no further use, and this
	// runs rarely enough that a scheduled job would be more machinery than it
	// is worth.
	if _, err := tx.Exec(ctx, `
		DELETE FROM login_tokens WHERE expires_at < $1::timestamptz - INTERVAL '1 day'`, now); err != nil {
		return 0, fmt.Errorf("sweep tokens: %w", err)
	}
	return userID, tx.Commit(ctx)
}

// CreateSession records a signed-in browser for the user.
func (pg *PG) CreateSession(ctx context.Context, tokenHash string, userID int64, expires time.Time) error {
	_, err := pg.pool.Exec(ctx, `
		INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`,
		tokenHash, userID, expires)
	return err
}

// GetAccountBySession resolves a session cookie to the account behind it.
// ErrNotFound for an unknown or expired session.
func (pg *PG) GetAccountBySession(ctx context.Context, tokenHash string, now time.Time) (Account, error) {
	var a Account
	var locID *int64
	err := pg.pool.QueryRow(ctx, `
		SELECT u.id, u.email, p.location_id
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		LEFT JOIN user_prefs p ON p.user_id = u.id
		WHERE s.token_hash = $1 AND s.expires_at > $2`, tokenHash, now).
		Scan(&a.UserID, &a.Email, &locID)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, ErrNotFound
	}
	if err != nil {
		return a, err
	}
	if locID != nil {
		row := pg.pool.QueryRow(ctx, `
			SELECT j.id, j.parent_id, j.kind, j.name, j.slug, COALESCE(p.slug, ''), COALESCE(p.name, '')
			FROM jurisdictions j
			LEFT JOIN jurisdictions p ON p.id = j.parent_id
			WHERE j.id = $1`, *locID)
		loc, err := scanJurisdiction(row)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return a, err
		}
		a.Location = loc
	}
	return a, nil
}

// DeleteSession signs one browser out. Deleting a session that is already
// gone is not an error: the cookie is cleared either way.
func (pg *PG) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := pg.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	return err
}

// SetUserLocation records the reader's chosen place by slug, or clears it
// when slug is empty. ErrNotFound when the slug names no jurisdiction, so a
// made-up value from the client never lands in the table.
func (pg *PG) SetUserLocation(ctx context.Context, userID int64, slug string) error {
	var locID *int64
	if slug != "" {
		j, err := pg.GetJurisdictionBySlug(ctx, slug)
		if err != nil {
			return err
		}
		locID = &j.ID
	}
	_, err := pg.pool.Exec(ctx, `
		INSERT INTO user_prefs (user_id, location_id, updated_at) VALUES ($1, $2, NOW())
		ON CONFLICT (user_id) DO UPDATE SET location_id = EXCLUDED.location_id, updated_at = NOW()`,
		userID, locID)
	return err
}
