package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Slug aliases make renames survivable.
//
// A renamed slug leaves every indexed link, bookmark, and saved agent prompt
// pointing at an address that no longer exists. Recording the old slug here lets
// browse handlers answer 301 instead of 404, so a rename costs one redirect
// generation rather than the accumulated search equity of the old URL.
//
// Lookups always try the live slug first and consult this table only on a miss,
// which is what keeps a stale alias from shadowing a real page.

// ResolveJurisdictionAlias returns the jurisdiction a retired slug now points
// at, or ErrNotFound if the slug was never in use.
func (pg *PG) ResolveJurisdictionAlias(ctx context.Context, alias string) (Jurisdiction, error) {
	var j Jurisdiction
	err := pg.pool.QueryRow(ctx, `
		SELECT j.id, j.parent_id, j.kind, j.name, j.slug, COALESCE(pj.slug, ''), COALESCE(pj.name, '')
		FROM slug_aliases a
		JOIN jurisdictions j ON j.id = a.jurisdiction_id
		LEFT JOIN jurisdictions pj ON pj.id = j.parent_id
		WHERE a.namespace = 'jurisdiction' AND a.alias = $1`, alias,
	).Scan(&j.ID, &j.ParentID, &j.Kind, &j.Name, &j.Slug, &j.ParentSlug, &j.ParentName)
	if errors.Is(err, pgx.ErrNoRows) {
		return j, ErrNotFound
	}
	return j, err
}

// ResolveTopicAlias returns the topic a retired slug now points at, or
// ErrNotFound if the slug was never in use.
func (pg *PG) ResolveTopicAlias(ctx context.Context, alias string) (Topic, error) {
	var t Topic
	err := pg.pool.QueryRow(ctx, `
		SELECT t.id, t.slug, t.name
		FROM slug_aliases a
		JOIN topics t ON t.id = a.topic_id
		WHERE a.namespace = 'topic' AND a.alias = $1`, alias,
	).Scan(&t.ID, &t.Slug, &t.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, ErrNotFound
	}
	return t, err
}

// AddJurisdictionAlias records a retired jurisdiction slug against the row that
// replaced it. It refuses an alias that shadows a live slug
// (ErrAliasShadowsLiveSlug), since lookups try live slugs first and such an
// alias could never be reached.
//
// Re-pointing an existing alias is allowed, so an old URL can be redirected at
// a different row later without having to delete and re-add it.
func (pg *PG) AddJurisdictionAlias(ctx context.Context, alias string, targetID int64) error {
	var live bool
	if err := pg.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM jurisdictions WHERE slug = $1)`, alias,
	).Scan(&live); err != nil {
		return fmt.Errorf("check live jurisdiction slug: %w", err)
	}
	if live {
		return fmt.Errorf("%w: jurisdiction %q", ErrAliasShadowsLiveSlug, alias)
	}
	_, err := pg.pool.Exec(ctx, `
		INSERT INTO slug_aliases (alias, namespace, jurisdiction_id)
		VALUES ($1, 'jurisdiction', $2)
		ON CONFLICT (namespace, alias) DO UPDATE SET jurisdiction_id = EXCLUDED.jurisdiction_id`,
		alias, targetID)
	if err != nil {
		return fmt.Errorf("insert jurisdiction alias %q: %w", alias, err)
	}
	return nil
}

// AddTopicAlias records a retired topic slug against the row that replaced it.
// Same rules as AddJurisdictionAlias.
func (pg *PG) AddTopicAlias(ctx context.Context, alias string, targetID int64) error {
	var live bool
	if err := pg.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM topics WHERE slug = $1)`, alias,
	).Scan(&live); err != nil {
		return fmt.Errorf("check live topic slug: %w", err)
	}
	if live {
		return fmt.Errorf("%w: topic %q", ErrAliasShadowsLiveSlug, alias)
	}
	_, err := pg.pool.Exec(ctx, `
		INSERT INTO slug_aliases (alias, namespace, topic_id)
		VALUES ($1, 'topic', $2)
		ON CONFLICT (namespace, alias) DO UPDATE SET topic_id = EXCLUDED.topic_id`,
		alias, targetID)
	if err != nil {
		return fmt.Errorf("insert topic alias %q: %w", alias, err)
	}
	return nil
}

// RenameJurisdiction changes a jurisdiction's slug and records the old one as an
// alias, in one transaction.
//
// Repeated renames need no extra bookkeeping: aliases store the target's id, not
// its slug, so every alias ever recorded for this row keeps resolving to it and
// each lookup stays a single hop.
func (pg *PG) RenameJurisdiction(ctx context.Context, oldSlug, newSlug string) error {
	return pgx.BeginTxFunc(ctx, pg.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var id int64
		err := tx.QueryRow(ctx,
			`UPDATE jurisdictions SET slug = $2 WHERE slug = $1 RETURNING id`, oldSlug, newSlug,
		).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("rename jurisdiction %q: %w", oldSlug, ErrNotFound)
		}
		if err != nil {
			return fmt.Errorf("rename jurisdiction %q: %w", oldSlug, err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO slug_aliases (alias, namespace, jurisdiction_id)
			VALUES ($1, 'jurisdiction', $2)
			ON CONFLICT (namespace, alias) DO UPDATE SET jurisdiction_id = EXCLUDED.jurisdiction_id`,
			oldSlug, id); err != nil {
			return fmt.Errorf("record alias %q: %w", oldSlug, err)
		}
		// The new slug may itself have been an alias for something else; it is
		// live now, so that alias must go or it would shadow this row.
		if _, err := tx.Exec(ctx,
			`DELETE FROM slug_aliases WHERE namespace = 'jurisdiction' AND alias = $1`, newSlug,
		); err != nil {
			return fmt.Errorf("clear alias %q: %w", newSlug, err)
		}
		return nil
	})
}

// RenameTopic changes a topic's slug and records the old one as an alias.
// Same transactional guarantees as RenameJurisdiction.
func (pg *PG) RenameTopic(ctx context.Context, oldSlug, newSlug string) error {
	return pgx.BeginTxFunc(ctx, pg.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var id int64
		err := tx.QueryRow(ctx,
			`UPDATE topics SET slug = $2 WHERE slug = $1 RETURNING id`, oldSlug, newSlug,
		).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("rename topic %q: %w", oldSlug, ErrNotFound)
		}
		if err != nil {
			return fmt.Errorf("rename topic %q: %w", oldSlug, err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO slug_aliases (alias, namespace, topic_id)
			VALUES ($1, 'topic', $2)
			ON CONFLICT (namespace, alias) DO UPDATE SET topic_id = EXCLUDED.topic_id`,
			oldSlug, id); err != nil {
			return fmt.Errorf("record alias %q: %w", oldSlug, err)
		}
		if _, err := tx.Exec(ctx,
			`DELETE FROM slug_aliases WHERE namespace = 'topic' AND alias = $1`, newSlug,
		); err != nil {
			return fmt.Errorf("clear alias %q: %w", newSlug, err)
		}
		return nil
	})
}
