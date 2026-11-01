package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Site copy pages: about, editorial standards, privacy, support. A row here
// overrides the built-in text in internal/sitecopy; no row means the page
// shows the built-in text. Every save keeps a history row.

// SitePage is one editable page of site copy.
type SitePage struct {
	Slug        string
	Title       string
	Description string
	Body        string // markdown
	UpdatedBy   string
	UpdatedAt   time.Time
}

// GetSitePage returns the saved copy for a slug, if any.
func (pg *PG) GetSitePage(ctx context.Context, slug string) (SitePage, bool, error) {
	var p SitePage
	err := pg.pool.QueryRow(ctx, `
		SELECT slug, title, description, body_md, updated_by, updated_at
		FROM site_pages WHERE slug = $1`, slug).
		Scan(&p.Slug, &p.Title, &p.Description, &p.Body, &p.UpdatedBy, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return SitePage{}, false, nil
	}
	if err != nil {
		return SitePage{}, false, err
	}
	return p, true, nil
}

// ListSitePages returns every saved page, keyed by slug.
func (pg *PG) ListSitePages(ctx context.Context) (map[string]SitePage, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT slug, title, description, body_md, updated_by, updated_at FROM site_pages`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]SitePage{}
	for rows.Next() {
		var p SitePage
		if err := rows.Scan(&p.Slug, &p.Title, &p.Description, &p.Body, &p.UpdatedBy, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out[p.Slug] = p
	}
	return out, rows.Err()
}

// SaveSitePage writes the page and a history row in one transaction. The
// caller has already checked the text; the store does not judge it.
func (pg *PG) SaveSitePage(ctx context.Context, p SitePage) error {
	return pgx.BeginTxFunc(ctx, pg.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO site_pages (slug, title, description, body_md, updated_by, updated_at)
			VALUES ($1, $2, $3, $4, $5, NOW())
			ON CONFLICT (slug) DO UPDATE SET
				title = EXCLUDED.title, description = EXCLUDED.description,
				body_md = EXCLUDED.body_md, updated_by = EXCLUDED.updated_by,
				updated_at = NOW()`,
			p.Slug, p.Title, p.Description, p.Body, p.UpdatedBy); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO site_page_history (slug, title, description, body_md, updated_by)
			VALUES ($1, $2, $3, $4, $5)`,
			p.Slug, p.Title, p.Description, p.Body, p.UpdatedBy)
		return err
	})
}
