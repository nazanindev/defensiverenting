package store

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nazanindev/defensiverenting/internal/discover"
)

// PG is the Postgres implementation of Store.
type PG struct {
	pool *pgxpool.Pool
}

// New opens a connection pool and returns a ready Store.
func New(ctx context.Context, dsn string) (*PG, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &PG{pool: pool}, nil
}

func (pg *PG) Close() {
	pg.pool.Close()
}

func (pg *PG) Ping(ctx context.Context) error {
	return pg.pool.Ping(ctx)
}

// Pool exposes the underlying pool (used by the db/migrate package).
func (pg *PG) Pool() *pgxpool.Pool {
	return pg.pool
}

// ---- Browse ----------------------------------------------------------------

// ListCityJurisdictions returns all city-level jurisdictions.
func (pg *PG) ListCityJurisdictions(ctx context.Context) ([]Jurisdiction, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT j.id, j.parent_id, j.kind, j.name, j.slug, COALESCE(p.slug, ''), COALESCE(p.name, '')
		FROM jurisdictions j
		LEFT JOIN jurisdictions p ON p.id = j.parent_id
		WHERE j.kind = 'city'
		ORDER BY j.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJurisdictions(rows)
}

// ListAuthorableJurisdictions returns every jurisdiction a page can be scoped
// to, not just cities. Tenant law layers — federal, then state, then local
// ordinances — and the content model has always allowed a playbook to hang off
// any level (Jurisdiction.Path renders all three). Only the authoring form's
// picker was city-only, which left the national "Renting Basics" page
// unsaveable: no option matched, so the required select submitted empty and the
// edit was rejected as "Unknown city selected".
//
// Ordered country first, then states, then cities grouped under their state, so
// the form can emit optgroups without a second pass.
func (pg *PG) ListAuthorableJurisdictions(ctx context.Context) ([]Jurisdiction, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT j.id, j.parent_id, j.kind, j.name, j.slug, COALESCE(p.slug, ''), COALESCE(p.name, '')
		FROM jurisdictions j
		LEFT JOIN jurisdictions p ON p.id = j.parent_id
		ORDER BY CASE j.kind WHEN 'country' THEN 0 WHEN 'state' THEN 1 ELSE 2 END,
		         COALESCE(p.name, ''), j.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJurisdictions(rows)
}

// ListPublishedCityJurisdictions returns city-level jurisdictions that have
// at least one published playbook. Used by the public site to avoid showing
// cities whose only playbooks have been deleted or were never published.
func (pg *PG) ListPublishedCityJurisdictions(ctx context.Context) ([]Jurisdiction, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT DISTINCT j.id, j.parent_id, j.kind, j.name, j.slug, COALESCE(pj.slug, ''), COALESCE(pj.name, '')
		FROM jurisdictions j
		JOIN playbooks p ON p.jurisdiction_id = j.id
		LEFT JOIN jurisdictions pj ON pj.id = j.parent_id
		WHERE j.kind = 'city' AND p.status = 'published'
		ORDER BY COALESCE(pj.name, ''), j.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJurisdictions(rows)
}

// ListPublishedStateJurisdictions returns state-level jurisdictions that have
// at least one published playbook of their own. The scope picker offers these
// as statewide choices: a renter in a town with no city page can scope to
// state law instead of settling for "Every location". A state whose only
// coverage is its cities is deliberately absent, matching the picker's
// long-standing rule that a choice must never silently exclude the renter's
// city.
func (pg *PG) ListPublishedStateJurisdictions(ctx context.Context) ([]Jurisdiction, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT DISTINCT j.id, j.parent_id, j.kind, j.name, j.slug, COALESCE(pj.slug, ''), COALESCE(pj.name, '')
		FROM jurisdictions j
		JOIN playbooks p ON p.jurisdiction_id = j.id
		LEFT JOIN jurisdictions pj ON pj.id = j.parent_id
		WHERE j.kind = 'state' AND p.status = 'published'
		ORDER BY j.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJurisdictions(rows)
}

// ListPublishedChildCities returns the cities under one parent (a state) that
// have at least one published playbook. A state hub had no way to list the
// cities beneath it, so /j/massachusetts rendered as a dead end even though
// Boston sat directly under it in the tree.
func (pg *PG) ListPublishedChildCities(ctx context.Context, parentID int64) ([]Jurisdiction, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT DISTINCT j.id, j.parent_id, j.kind, j.name, j.slug, COALESCE(pj.slug, ''), COALESCE(pj.name, '')
		FROM jurisdictions j
		JOIN playbooks p ON p.jurisdiction_id = j.id
		LEFT JOIN jurisdictions pj ON pj.id = j.parent_id
		WHERE j.parent_id = $1 AND j.kind = 'city' AND p.status = 'published'
		ORDER BY j.name`, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJurisdictions(rows)
}

// ListPublishedHubJurisdictions returns every jurisdiction — country, state,
// or city — that has at least one published playbook of its own, ordered
// country first, then states, then cities grouped under their state.
//
// This is the hub-page inventory: it feeds the sitemap and /locations, which
// used to list city hubs only, leaving the national and statewide hubs live
// but unreachable by crawl. A state that has covered cities but no statewide
// playbook is deliberately absent: its heading in the grouped city index
// already links to its hub.
func (pg *PG) ListPublishedHubJurisdictions(ctx context.Context) ([]Jurisdiction, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT j.id, j.parent_id, j.kind, j.name, j.slug, COALESCE(pj.slug, ''), COALESCE(pj.name, '')
		FROM jurisdictions j
		LEFT JOIN jurisdictions pj ON pj.id = j.parent_id
		WHERE EXISTS (
			SELECT 1 FROM playbooks p
			WHERE p.jurisdiction_id = j.id AND p.status = 'published')
		ORDER BY CASE j.kind WHEN 'country' THEN 0 WHEN 'state' THEN 1 ELSE 2 END,
		         COALESCE(pj.name, ''), j.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJurisdictions(rows)
}

// GetNearestTopicJurisdiction returns the closest jurisdiction, walking up from
// the given one through its ancestors, that has a published playbook for the
// topic in the language: the city's own page when it has one, else the state's,
// else the national guide. Same upward-only rule that scopes search. Returns
// ErrNotFound when nothing up the chain covers the topic.
func (pg *PG) GetNearestTopicJurisdiction(ctx context.Context, jurisdictionID, topicID int64, language string) (Jurisdiction, error) {
	row := pg.pool.QueryRow(ctx, `
		WITH RECURSIVE ancestors AS (
			SELECT id, parent_id, 0 AS depth FROM jurisdictions WHERE id = $1
			UNION ALL
			SELECT j.id, j.parent_id, a.depth + 1 FROM jurisdictions j
			JOIN ancestors a ON j.id = a.parent_id
		)
		SELECT j.id, j.parent_id, j.kind, j.name, j.slug, COALESCE(pj.slug, ''), COALESCE(pj.name, '')
		FROM ancestors a
		JOIN jurisdictions j ON j.id = a.id
		LEFT JOIN jurisdictions pj ON pj.id = j.parent_id
		WHERE EXISTS (
			SELECT 1 FROM playbooks p
			WHERE p.jurisdiction_id = j.id AND p.topic_id = $2
			  AND p.language = $3 AND p.status = 'published')
		ORDER BY a.depth
		LIMIT 1`, jurisdictionID, topicID, language)
	return scanJurisdiction(row)
}

// GetJurisdictionBySlug looks up one jurisdiction by its URL slug.
func (pg *PG) GetJurisdictionBySlug(ctx context.Context, slug string) (Jurisdiction, error) {
	row := pg.pool.QueryRow(ctx, `
		SELECT j.id, j.parent_id, j.kind, j.name, j.slug, COALESCE(p.slug, ''), COALESCE(p.name, '')
		FROM jurisdictions j
		LEFT JOIN jurisdictions p ON p.id = j.parent_id
		WHERE j.slug = $1`, slug)
	return scanJurisdiction(row)
}

// ListTopicsByJurisdiction returns topics that have a published playbook for the given jurisdiction + language.
func (pg *PG) ListTopicsByJurisdiction(ctx context.Context, jurisdictionID int64, language string) ([]Topic, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT t.id, t.slug, t.name
		FROM topics t
		JOIN playbooks p ON p.topic_id = t.id
		WHERE p.jurisdiction_id = $1 AND p.language = $2 AND p.status = 'published'
		ORDER BY t.name`, jurisdictionID, language)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTopics(rows)
}

// ListTopicsByJurisdictionRecursive returns topics that have a published
// playbook for the jurisdiction or any of its ancestors, in the language. This
// is the coverage set a location actually resolves to under the upward-only
// rule: the topics /t/{topic}?j={slug} would land on a real guide for, which
// is what the homepage's situation list filters against once a location is
// chosen.
func (pg *PG) ListTopicsByJurisdictionRecursive(ctx context.Context, jurisdictionID int64, language string) ([]Topic, error) {
	rows, err := pg.pool.Query(ctx, `
		WITH RECURSIVE ancestors AS (
			SELECT id, parent_id FROM jurisdictions WHERE id = $1
			UNION ALL
			SELECT j.id, j.parent_id FROM jurisdictions j
			JOIN ancestors a ON j.id = a.parent_id
		)
		SELECT DISTINCT t.id, t.slug, t.name
		FROM topics t
		JOIN playbooks p ON p.topic_id = t.id
		JOIN ancestors a ON a.id = p.jurisdiction_id
		WHERE p.language = $2 AND p.status = 'published'
		ORDER BY t.name`, jurisdictionID, language)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTopics(rows)
}

func scanTopics(rows pgx.Rows) ([]Topic, error) {
	var topics []Topic
	for rows.Next() {
		var t Topic
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name); err != nil {
			return nil, err
		}
		topics = append(topics, t)
	}
	return topics, rows.Err()
}

// ListTopicRegistry returns every topic that exists, ordered core-first then by
// name. This is the registry: the authoring dropdown and the drafting agent's
// list_topics both read it, so there is one vocabulary rather than a hardcoded
// list per caller. See docs/ADRs/ADR-005 D5.
func (pg *PG) ListTopicRegistry(ctx context.Context) ([]Topic, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT id, slug, name, is_core FROM topics ORDER BY is_core DESC, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Topic
	for rows.Next() {
		var t Topic
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name, &t.IsCore); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ListCoreTopics returns the topics every new city is seeded with.
func (pg *PG) ListCoreTopics(ctx context.Context) ([]Topic, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT id, slug, name, is_core FROM topics WHERE is_core ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Topic
	for rows.Next() {
		var t Topic
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name, &t.IsCore); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetJurisdictionByID looks up one jurisdiction by id. Used to walk the
// parent chain when copying jurisdictions between databases, where ids are
// not comparable and the hierarchy has to be rebuilt by slug.
func (pg *PG) GetJurisdictionByID(ctx context.Context, id int64) (Jurisdiction, error) {
	var j Jurisdiction
	err := pg.pool.QueryRow(ctx, `
		SELECT j.id, j.parent_id, j.kind, j.name, j.slug, COALESCE(p.slug, '')
		FROM jurisdictions j
		LEFT JOIN jurisdictions p ON p.id = j.parent_id
		WHERE j.id = $1`, id,
	).Scan(&j.ID, &j.ParentID, &j.Kind, &j.Name, &j.Slug, &j.ParentSlug)
	if errors.Is(err, pgx.ErrNoRows) {
		return j, ErrNotFound
	}
	return j, err
}

// GetTopicBySlug looks up one topic by its URL slug.
func (pg *PG) GetTopicBySlug(ctx context.Context, slug string) (Topic, error) {
	var t Topic
	err := pg.pool.QueryRow(ctx, `
		SELECT id, slug, name FROM topics WHERE slug = $1`, slug).
		Scan(&t.ID, &t.Slug, &t.Name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return t, ErrNotFound
		}
		return t, err
	}
	return t, nil
}

// ListPublishedTopics returns topics that have at least one published playbook.
func (pg *PG) ListPublishedTopics(ctx context.Context, language string) ([]Topic, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT DISTINCT t.id, t.slug, t.name
		FROM topics t
		JOIN playbooks p ON p.topic_id = t.id
		WHERE p.language = $1 AND p.status = 'published'
		ORDER BY t.name`, language)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var topics []Topic
	for rows.Next() {
		var t Topic
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name); err != nil {
			return nil, err
		}
		topics = append(topics, t)
	}
	return topics, rows.Err()
}

// ListJurisdictionsByTopic returns city jurisdictions with a published playbook
// for the given topic + language.
func (pg *PG) ListJurisdictionsByTopic(ctx context.Context, topicID int64, language string) ([]Jurisdiction, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT DISTINCT j.id, j.parent_id, j.kind, j.name, j.slug, COALESCE(pj.slug, ''), COALESCE(pj.name, '')
		FROM jurisdictions j
		JOIN playbooks p ON p.jurisdiction_id = j.id
		LEFT JOIN jurisdictions pj ON pj.id = j.parent_id
		WHERE p.topic_id = $1 AND p.language = $2 AND p.status = 'published'
		ORDER BY COALESCE(pj.name, ''), j.name`, topicID, language)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJurisdictions(rows)
}

// GetPlaybook fetches the full playbook with all statements and their citations.
// Returns ErrNotFound if the (jurisdiction, topic, language) combination does not exist.
func (pg *PG) GetPlaybook(ctx context.Context, jurisdictionSlug, topicSlug, language string) (PlaybookWithStatements, error) {
	var p PlaybookWithStatements

	err := pg.pool.QueryRow(ctx, `
		SELECT
			pb.id, pb.jurisdiction_id, pb.topic_id, pb.language,
			pb.slug, pb.title, pb.intro_md, pb.page_kind, pb.last_reviewed_at, pb.updated_by,
			pb.published_at, pb.updated_at,
			j.id, j.parent_id, j.kind, j.name, j.slug, COALESCE(pj.slug, ''),
			t.id, t.slug, t.name
		FROM playbooks pb
		JOIN jurisdictions j ON j.id = pb.jurisdiction_id
		LEFT JOIN jurisdictions pj ON pj.id = j.parent_id
		JOIN topics        t ON t.id  = pb.topic_id
		WHERE j.slug = $1 AND t.slug = $2 AND pb.language = $3 AND pb.status = 'published'`,
		jurisdictionSlug, topicSlug, language,
	).Scan(
		&p.Playbook.ID, &p.Playbook.JurisdictionID, &p.Playbook.TopicID,
		&p.Playbook.Language, &p.Playbook.Slug, &p.Playbook.Title,
		&p.Playbook.IntroMD, &p.Playbook.PageKind, &p.Playbook.LastReviewedAt, &p.Playbook.UpdatedBy,
		&p.Playbook.PublishedAt, &p.Playbook.UpdatedAt,
		&p.Jurisdiction.ID, &p.Jurisdiction.ParentID, &p.Jurisdiction.Kind,
		&p.Jurisdiction.Name, &p.Jurisdiction.Slug, &p.Jurisdiction.ParentSlug,
		&p.Topic.ID, &p.Topic.Slug, &p.Topic.Name,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return p, ErrNotFound
		}
		return p, err
	}

	// Fetch statement rows; each row is one citation, so multiple rows per statement
	statementRows, err := pg.pool.Query(ctx, `
		SELECT
			s.id, s.key::text, s.body_md, COALESCE(co.slug, ''), COALESCE(tr.slug, ''), COALESCE(tr.name, ''), ps.position,
			c.source_id, c.locator, c.quote, c.manually_verified, c.checked_at, c.checked_by,
			src.url, src.publisher, src.kind,
			`+reviewedAtSQL+`, `+reviewedBySQL+`, `+undecidedSQL+`
		FROM playbook_statements ps
		JOIN statements s   ON s.id  = ps.statement_id
		JOIN statement_review_hash h ON h.statement_id = s.id
		LEFT JOIN concepts co ON co.id = s.concept_id
		LEFT JOIN topics   tr ON tr.id = s.topic_ref
		JOIN citations  c   ON c.statement_id = s.id
		JOIN sources    src ON src.id = c.source_id
		WHERE ps.playbook_id = $1
		ORDER BY ps.position, c.source_id`, p.Playbook.ID)
	if err != nil {
		return p, err
	}
	defer statementRows.Close()

	p.Statements = assembleStatements(statementRows)
	return p, statementRows.Err()
}

// ---- Search ----------------------------------------------------------------

// Search performs full-text search scoped to a jurisdiction's ancestor chain.
func (pg *PG) Search(ctx context.Context, query string, jurisdictionID *int64, language string) ([]SearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}

	var stmtRows pgx.Rows
	var err error
	if jurisdictionID != nil {
		stmtRows, err = pg.pool.Query(ctx, `
			WITH RECURSIVE ancestors AS (
				SELECT id, parent_id FROM jurisdictions WHERE id = $1
				UNION ALL
				SELECT j.id, j.parent_id FROM jurisdictions j
				JOIN ancestors a ON j.id = a.parent_id
			),
			q AS (SELECT plainto_tsquery(lang_regconfig($3), $2) AS tsq)
			SELECT
				s.id,
				s.body_md,
				ts_rank_cd(s.body_tsv, q.tsq) AS rank,
				COALESCE(j.slug, ''),
				COALESCE(pj.slug, ''),
				COALESCE(t.slug, ''),
				COALESCE(pb.slug, ''),
				COALESCE(pb.title,'')
			FROM statements s
			CROSS JOIN q
			JOIN ancestors a ON a.id = s.jurisdiction_id
			LEFT JOIN playbook_statements ps ON ps.statement_id = s.id
			LEFT JOIN playbooks pb ON pb.id = ps.playbook_id
			LEFT JOIN topics t ON t.id = pb.topic_id
			LEFT JOIN jurisdictions j ON j.id = s.jurisdiction_id
			LEFT JOIN jurisdictions pj ON pj.id = j.parent_id
			WHERE s.body_tsv @@ q.tsq AND s.language = $3
			  -- Only statements on a published page are public. Without this,
			  -- search leaks the text of drafts no human has reviewed yet.
			  AND pb.status = 'published'
			ORDER BY rank DESC LIMIT 20`,
			*jurisdictionID, query, language)
	} else {
		stmtRows, err = pg.pool.Query(ctx, `
			WITH q AS (SELECT plainto_tsquery(lang_regconfig($2), $1) AS tsq)
			SELECT
				s.id,
				s.body_md,
				ts_rank_cd(s.body_tsv, q.tsq) AS rank,
				COALESCE(j.slug, ''),
				COALESCE(pj.slug, ''),
				COALESCE(t.slug, ''),
				COALESCE(pb.slug, ''),
				COALESCE(pb.title,'')
			FROM statements s
			CROSS JOIN q
			LEFT JOIN playbook_statements ps ON ps.statement_id = s.id
			LEFT JOIN playbooks pb ON pb.id = ps.playbook_id
			LEFT JOIN topics t ON t.id = pb.topic_id
			LEFT JOIN jurisdictions j ON j.id = s.jurisdiction_id
			LEFT JOIN jurisdictions pj ON pj.id = j.parent_id
			WHERE s.body_tsv @@ q.tsq AND s.language = $2
			  AND pb.status = 'published'
			ORDER BY rank DESC LIMIT 20`,
			query, language)
	}
	if err != nil {
		return nil, err
	}
	defer stmtRows.Close()

	var results []SearchResult

	// Registry terms first (ADR-012 D4): a query naming a concept routes to
	// the reference page that defines it and lists every jurisdiction's
	// answer, above whichever statement's prose happened to rank. The
	// registry is a few dozen rows, so a substring match suffices; gated on
	// the page existing (some published tagged statement carries it).
	termRows, terr := pg.pool.Query(ctx, `
		SELECT co.slug, co.name FROM concepts co
		WHERE (co.name ILIKE '%' || $1 || '%' OR replace(co.slug, '-', ' ') ILIKE '%' || $1 || '%')
		  AND EXISTS (
			SELECT 1 FROM statements s
			JOIN playbook_statements ps ON ps.statement_id = s.id
			JOIN playbooks pb ON pb.id = ps.playbook_id
			WHERE s.concept_id = co.id AND pb.status = 'published' AND pb.language = $2)
		ORDER BY co.name LIMIT 3`, strings.TrimSpace(query), language)
	if terr != nil {
		return nil, terr
	}
	defer termRows.Close()
	for termRows.Next() {
		var r SearchResult
		if err := termRows.Scan(&r.TermSlug, &r.TermName); err != nil {
			return nil, err
		}
		r.Type = "term"
		results = append(results, r)
	}
	if err := termRows.Err(); err != nil {
		return nil, err
	}

	for stmtRows.Next() {
		var r SearchResult
		var sid int64
		if err := stmtRows.Scan(&sid, &r.Snippet, &r.Rank,
			&r.JurisdictionSlug, &r.JurisdictionParentSlug, &r.TopicSlug,
			&r.PlaybookSlug, &r.PlaybookTitle); err != nil {
			return nil, err
		}
		r.Type = "statement"
		r.StatementID = &sid
		if len(r.Snippet) > 200 {
			r.Snippet = r.Snippet[:200] + "…"
		}
		results = append(results, r)
	}
	if err := stmtRows.Err(); err != nil {
		return nil, err
	}

	pbRows, err := pg.pool.Query(ctx, `
		WITH q AS (SELECT plainto_tsquery(lang_regconfig($3), $1) AS tsq)
		SELECT
			pb.slug, pb.title, pb.intro_md,
			ts_rank_cd(pb.body_tsv, q.tsq) AS rank,
			j.slug, COALESCE(pj.slug, ''), t.slug
		FROM playbooks pb
		CROSS JOIN q
		JOIN jurisdictions j ON j.id = pb.jurisdiction_id
		LEFT JOIN jurisdictions pj ON pj.id = j.parent_id
		JOIN topics t ON t.id = pb.topic_id
		WHERE pb.body_tsv @@ q.tsq
		  AND ($2::BIGINT IS NULL OR pb.jurisdiction_id = $2)
		  AND pb.language = $3
		  AND pb.status = 'published'
		ORDER BY rank DESC LIMIT 10`,
		query, jurisdictionID, language)
	if err != nil {
		return nil, err
	}
	defer pbRows.Close()

	for pbRows.Next() {
		var r SearchResult
		if err := pbRows.Scan(&r.PlaybookSlug, &r.PlaybookTitle, &r.Snippet, &r.Rank,
			&r.JurisdictionSlug, &r.JurisdictionParentSlug, &r.TopicSlug); err != nil {
			return nil, err
		}
		r.Type = "playbook"
		if len(r.Snippet) > 200 {
			r.Snippet = r.Snippet[:200] + "…"
		}
		results = append(results, r)
	}
	return results, pbRows.Err()
}

// ---- SEO -------------------------------------------------------------------

// ListSitemapURLs returns jurisdiction/topic/language rows for every
// published playbook, in whichever language(s) it exists — see ADR-007 D7.
func (pg *PG) ListSitemapURLs(ctx context.Context) ([]SitemapEntry, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT j.slug, COALESCE(pj.slug, ''), j.kind, t.slug, pb.language,
		       COALESCE(pb.last_reviewed_at, pb.published_at, pb.updated_at)
		FROM playbooks pb
		JOIN jurisdictions j ON j.id = pb.jurisdiction_id
		LEFT JOIN jurisdictions pj ON pj.id = j.parent_id
		JOIN topics        t ON t.id  = pb.topic_id
		WHERE pb.status = 'published'
		ORDER BY j.slug, t.slug, pb.language`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []SitemapEntry
	for rows.Next() {
		var e SitemapEntry
		if err := rows.Scan(&e.JurisdictionSlug, &e.JurisdictionParentSlug,
			&e.JurisdictionKind, &e.TopicSlug, &e.Language, &e.LastMod); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// ---- Ingest ----------------------------------------------------------------

func (pg *PG) UpsertJurisdiction(ctx context.Context, params UpsertJurisdictionParams) (Jurisdiction, error) {
	row := pg.pool.QueryRow(ctx, `
		INSERT INTO jurisdictions (parent_id, kind, name, slug)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (slug) DO UPDATE
		    SET name = EXCLUDED.name, kind = EXCLUDED.kind, parent_id = EXCLUDED.parent_id
		RETURNING id, parent_id, kind, name, slug,
		    COALESCE((SELECT p.slug FROM jurisdictions p WHERE p.id = jurisdictions.parent_id), ''),
		    COALESCE((SELECT p.name FROM jurisdictions p WHERE p.id = jurisdictions.parent_id), '')`,
		params.ParentID, params.Kind, params.Name, params.Slug)
	return scanJurisdiction(row)
}

func (pg *PG) UpsertSource(ctx context.Context, params UpsertSourceParams) (Source, error) {
	// Reference-only domains never become source rows, from any path — the
	// agent, the authoring form, or ingest. This is the deepest chokepoint,
	// so a new caller cannot recreate the lawyer-blog-as-source incident.
	if discover.ReferenceOnly(params.URL) {
		return Source{}, fmt.Errorf("%s is reference-only (lawyer marketing or content mill) and can never be a source. Read it to orient, then cite the primary law it summarizes", params.URL)
	}
	now := time.Now().UTC()
	// Sources are shared across pages, and a save may now arrive with a
	// publisher not yet filled in (ADR-013). An empty publisher must not wipe
	// the name every other page shows for the same source.
	row := pg.pool.QueryRow(ctx, `
		INSERT INTO sources (url, publisher, jurisdiction_id, kind, retrieved_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (url) DO UPDATE
		    SET publisher     = CASE WHEN EXCLUDED.publisher = '' THEN sources.publisher
		                             ELSE EXCLUDED.publisher END,
		        kind          = EXCLUDED.kind,
		        retrieved_at  = EXCLUDED.retrieved_at
		RETURNING id, url, publisher, jurisdiction_id, kind, retrieved_at, content_hash, last_checked_at`,
		params.URL, params.Publisher, params.JurisdictionID, params.Kind, now)
	var s Source
	err := row.Scan(&s.ID, &s.URL, &s.Publisher, &s.JurisdictionID, &s.Kind, &s.RetrievedAt, &s.ContentHash, &s.LastCheckedAt)
	return s, err
}

// ---- source discovery -------------------------------------------------------

// InsertCandidates bulk-inserts discovered candidates for a jurisdiction.
// Re-running discovery is idempotent: an existing (jurisdiction, url) row is left
// untouched so its triage status is preserved. Returns the count newly added.
func (pg *PG) InsertCandidates(ctx context.Context, jurisdictionID int64, cands []discover.Candidate) (int, error) {
	if len(cands) == 0 {
		return 0, nil
	}
	tx, err := pg.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after a successful commit

	added := 0
	for _, c := range cands {
		tag, err := tx.Exec(ctx, `
			INSERT INTO source_candidates
				(jurisdiction_id, url, publisher, title, kind_guess, rationale, confidence, discovered_via)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (jurisdiction_id, url) DO NOTHING`,
			jurisdictionID, c.URL, c.Publisher, c.Title, c.KindGuess, c.Rationale, c.Confidence, c.Via)
		if err != nil {
			return 0, fmt.Errorf("insert candidate %s: %w", c.URL, err)
		}
		added += int(tag.RowsAffected())
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return added, nil
}

// ListCandidates returns a jurisdiction's candidates with the given status,
// ranked by confidence (highest first).
func (pg *PG) ListCandidates(ctx context.Context, jurisdictionID int64, status string) ([]SourceCandidate, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT id, jurisdiction_id, url, publisher, title, kind_guess, rationale,
		       confidence, discovered_via, status, source_id, created_at, reviewed_at
		FROM source_candidates
		WHERE jurisdiction_id = $1 AND status = $2
		ORDER BY confidence DESC, id`, jurisdictionID, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCandidates(rows)
}

// GetCandidate looks up a single candidate by id.
func (pg *PG) GetCandidate(ctx context.Context, id int64) (SourceCandidate, error) {
	row := pg.pool.QueryRow(ctx, `
		SELECT id, jurisdiction_id, url, publisher, title, kind_guess, rationale,
		       confidence, discovered_via, status, source_id, created_at, reviewed_at
		FROM source_candidates WHERE id = $1`, id)
	return scanCandidate(row)
}

// SetCandidateStatus records a triage decision. sourceID is non-nil only when a
// candidate is approved (linking it to the sources row it created).
func (pg *PG) SetCandidateStatus(ctx context.Context, id int64, status string, sourceID *int64) error {
	_, err := pg.pool.Exec(ctx, `
		UPDATE source_candidates
		SET status = $2, source_id = $3, reviewed_at = NOW()
		WHERE id = $1`, id, status, sourceID)
	return err
}

// CandidateCounts returns the number of pending candidates per city, for the
// authoring dashboard's "sources to review" badges.
func (pg *PG) CandidateCounts(ctx context.Context) ([]CandidateCountRow, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT j.id, j.name, j.slug, COUNT(*)
		FROM source_candidates c
		JOIN jurisdictions j ON j.id = c.jurisdiction_id
		WHERE c.status = 'pending'
		GROUP BY j.id, j.name, j.slug
		ORDER BY j.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CandidateCountRow
	for rows.Next() {
		var r CandidateCountRow
		if err := rows.Scan(&r.JurisdictionID, &r.JurisdictionName, &r.JurisdictionSlug, &r.PendingCount); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func scanCandidate(row pgx.Row) (SourceCandidate, error) {
	c, err := scanCandidateRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}

func scanCandidates(rows pgx.Rows) ([]SourceCandidate, error) {
	var out []SourceCandidate
	for rows.Next() {
		c, err := scanCandidateRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// scanCandidateRow scans the source_candidates columns (in select order) from a single row.
func scanCandidateRow(row pgx.Row) (SourceCandidate, error) {
	var c SourceCandidate
	err := row.Scan(&c.ID, &c.JurisdictionID, &c.URL, &c.Publisher, &c.Title,
		&c.KindGuess, &c.Rationale, &c.Confidence, &c.DiscoveredVia, &c.Status,
		&c.SourceID, &c.CreatedAt, &c.ReviewedAt)
	return c, err
}

func (pg *PG) UpsertTopic(ctx context.Context, params UpsertTopicParams) (Topic, error) {
	// Topics are shared across every city, so the name is set once at creation
	// and never overwritten here: a draft saved with a different display name
	// would otherwise rename the topic on every city page site-wide. Renaming a
	// topic is an explicit editorial action, not a side effect of saving.
	// The no-op SET keeps RETURNING working on conflict. See docs/ADRs/ADR-005.
	row := pg.pool.QueryRow(ctx, `
		INSERT INTO topics (slug, name) VALUES ($1, $2)
		ON CONFLICT (slug) DO UPDATE SET name = topics.name
		RETURNING id, slug, name`, params.Slug, params.Name)
	var t Topic
	err := row.Scan(&t.ID, &t.Slug, &t.Name)
	return t, err
}

// GetEditorialSource returns the site's own editorial-guidance source — the
// row migration 000001 seeded at url '/editorial'. It must select by URL, not
// by kind: third-party commentary (law firm blogs, Nolo) was historically
// filed under kind='editorial' too, and `WHERE kind = 'editorial' LIMIT 1`
// with no ORDER BY returned an arbitrary row. Every statement saved with the
// "editorial guidance" checkbox cites the row returned here, so the arbitrary
// pick published statements citing a lawyer's marketing blog as if it were
// our own guidance (found live on three pages, 2026-08-21). url is UNIQUE, so
// this is deterministic.
func (pg *PG) GetEditorialSource(ctx context.Context) (Source, error) {
	row := pg.pool.QueryRow(ctx, `
		SELECT id, url, publisher, jurisdiction_id, kind, retrieved_at, content_hash, last_checked_at
		FROM sources WHERE url = '/editorial' LIMIT 1`)
	var s Source
	err := row.Scan(&s.ID, &s.URL, &s.Publisher, &s.JurisdictionID, &s.Kind, &s.RetrievedAt, &s.ContentHash, &s.LastCheckedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, ErrNotFound
	}
	return s, err
}

// insertCitationSQL is the one way a citation row is written, shared by
// IngestPlaybook and AuthorUpdatePlaybook so checked_at cannot drift between
// the two save paths.
//
// checked_at records when the quote was last confirmed at the live source. An
// empty quote was never confirmed, so it stays NULL. When the caller verified
// the quote during this save ($6), the stamp is now. Otherwise the row inherits
// the newest stamp stored for the same (source, quote): the save skipped the
// fetch precisely because that pair was verified before (CitationQuoteExists),
// and the identical text carries its confirmation with it. Rows from before
// migration 000017 have no stamp to inherit and stay NULL — claiming a time we
// did not record would be the same lie retrieved_at already tells.
//
// checked_by and the receipt columns (checked_via, checked_extractor,
// checked_hash, checked_context; migration 000040) follow checked_at through
// the same three arms: nothing for a never-confirmed quote, the current
// actor's values ($7..$11) for a confirmation made by this save, and
// otherwise whatever the newest stamp being inherited recorded — the
// inherited time, name and receipt must describe the same confirmation.
const insertCitationSQL = `
	INSERT INTO citations (statement_id, source_id, locator, quote, manually_verified,
	                       checked_at, checked_by, checked_via, checked_extractor, checked_hash, checked_context)
	SELECT $1::bigint, $2::bigint, $3::text, $4::text, $5::boolean,
		CASE WHEN btrim($4::text) = '' THEN NULL WHEN $6::boolean THEN NOW() ELSE prior.checked_at END,
		CASE WHEN btrim($4::text) = '' THEN '' WHEN $6::boolean THEN $7::text  ELSE COALESCE(prior.checked_by, '') END,
		CASE WHEN btrim($4::text) = '' THEN '' WHEN $6::boolean THEN $8::text  ELSE COALESCE(prior.checked_via, '') END,
		CASE WHEN btrim($4::text) = '' THEN '' WHEN $6::boolean THEN $9::text  ELSE COALESCE(prior.checked_extractor, '') END,
		CASE WHEN btrim($4::text) = '' THEN '' WHEN $6::boolean THEN $10::text ELSE COALESCE(prior.checked_hash, '') END,
		CASE WHEN btrim($4::text) = '' THEN '' WHEN $6::boolean THEN $11::text ELSE COALESCE(prior.checked_context, '') END
	FROM (SELECT 1) AS one
	LEFT JOIN LATERAL (
		SELECT c2.checked_at, c2.checked_by, c2.checked_via, c2.checked_extractor, c2.checked_hash, c2.checked_context
		FROM citations c2
		WHERE c2.source_id = $2::bigint AND c2.quote = $4::text AND c2.checked_at IS NOT NULL
		ORDER BY c2.checked_at DESC LIMIT 1
	) AS prior ON true
	ON CONFLICT (statement_id, source_id) DO UPDATE SET
		locator = EXCLUDED.locator, quote = EXCLUDED.quote,
		manually_verified = EXCLUDED.manually_verified,
		checked_at = EXCLUDED.checked_at, checked_by = EXCLUDED.checked_by,
		checked_via = EXCLUDED.checked_via, checked_extractor = EXCLUDED.checked_extractor,
		checked_hash = EXCLUDED.checked_hash, checked_context = EXCLUDED.checked_context`

// citationArgs is the argument list insertCitationSQL expects for one row.
func citationArgs(stmtID int64, cite IngestCitationParams) []any {
	return []any{stmtID, cite.SourceID, cite.Locator, cite.Quote, cite.ManuallyVerified, cite.CheckedNow, cite.CheckedBy,
		cite.Checked.Via, cite.Checked.Extractor, cite.Checked.Hash, cite.Checked.Context}
}

// insertStatement writes one statement row, resolving its concept slug against
// the registry. An unknown slug is an error, not a silently dropped tag: the
// registry is closed (ADR-011 D1), both tagging paths validate before save,
// and a tag that vanished quietly would surface later as a coverage lie.
func insertStatement(ctx context.Context, tx pgx.Tx, jurisdictionID int64, sp IngestStatementParams, key string) (int64, error) {
	if sp.ConceptSlug != "" && sp.TopicRefSlug != "" {
		return 0, fmt.Errorf("statement carries both concept %q and topic reference %q: a statement is one claim or one summary, never both (ADR-011 D7)", sp.ConceptSlug, sp.TopicRefSlug)
	}
	var conceptID *int64
	if sp.ConceptSlug != "" {
		var id int64
		err := tx.QueryRow(ctx, `SELECT id FROM concepts WHERE slug = $1`, sp.ConceptSlug).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, fmt.Errorf("unknown concept slug %q: concepts are a closed registry (ADR-011), added by migration", sp.ConceptSlug)
		}
		if err != nil {
			return 0, err
		}
		conceptID = &id
	}
	var topicRefID *int64
	if sp.TopicRefSlug != "" {
		var id int64
		err := tx.QueryRow(ctx, `SELECT id FROM topics WHERE slug = $1`, sp.TopicRefSlug).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, fmt.Errorf("unknown topic reference %q: topics are a closed registry (ADR-005 D5)", sp.TopicRefSlug)
		}
		if err != nil {
			return 0, err
		}
		topicRefID = &id
	}
	// The review stamp (ADR-018 D2) rides along from the most recent row
	// with the same key, the way a citation inherits its confirmation. The
	// stamp is only honoured while its hash matches the content, so carrying
	// it onto an edited statement is harmless: the mismatch reads as
	// unreviewed. The prior row still exists here because saves detach and
	// delete replaced rows last.
	var stmtID int64
	err := tx.QueryRow(ctx, `
		INSERT INTO statements (jurisdiction_id, language, body_md, concept_id, topic_ref, key,
		                        last_reviewed_at, reviewed_by, reviewed_hash)
		SELECT $1, $2, $3, $4, $5, k.key,
		       prior.last_reviewed_at, COALESCE(prior.reviewed_by, ''), COALESCE(prior.reviewed_hash, '')
		FROM (SELECT COALESCE(NULLIF($6, '')::uuid, gen_random_uuid()) AS key) k
		LEFT JOIN LATERAL (
			SELECT p.last_reviewed_at, p.reviewed_by, p.reviewed_hash
			FROM statements p WHERE p.key = k.key ORDER BY p.id DESC LIMIT 1
		) prior ON true
		RETURNING id`,
		jurisdictionID, sp.Language, sp.BodyMD, conceptID, topicRefID, key,
	).Scan(&stmtID)
	return stmtID, err
}

// uuidRE accepts the textual form statements.key is read back in. An explicit
// key that is not one is a caller bug, refused by name rather than as a cast
// error from inside the insert.
var uuidRE = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// slotKeys returns the keys of every statement currently linked to a page in
// this playbook's slot (same jurisdiction, topic, and language — that is, the
// page itself and, for a draft revision, the live page beside it), indexed by
// body text. Called before the save unlinks the page's old rows, it is what
// lets a save that carries no explicit keys keep the keys of every statement
// whose body did not change.
//
// The page's own rows win over the sibling's, and a published sibling over a
// draft one, so the key stays with the version the author is looking at.
func slotKeys(ctx context.Context, tx pgx.Tx, playbookID int64) (map[string]string, error) {
	rows, err := tx.Query(ctx, `
		SELECT s.body_md, s.key::text
		FROM playbook_statements ps
		JOIN statements s ON s.id = ps.statement_id
		JOIN playbooks pb ON pb.id = ps.playbook_id
		JOIN playbooks me ON me.id = $1
		WHERE pb.jurisdiction_id = me.jurisdiction_id
		  AND pb.topic_id = me.topic_id
		  AND pb.language = me.language
		ORDER BY (pb.id = $1) DESC, (pb.status = 'published') DESC, ps.position`, playbookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var body, key string
		if err := rows.Scan(&body, &key); err != nil {
			return nil, err
		}
		if _, seen := out[body]; !seen {
			out[body] = key
		}
	}
	return out, rows.Err()
}

// keyChooser hands each statement in one save its key (ADR-014 D1): the
// explicit key when the caller passed one, else the key of the statement in
// this slot with the same body, else "" so the insert mints one. A key is
// handed out once per save — two statements sharing a key would make the
// identity ambiguous, so the second identical body gets a fresh one.
type keyChooser struct {
	byBody map[string]string
	used   map[string]bool
}

func newKeyChooser(ctx context.Context, tx pgx.Tx, playbookID int64) (*keyChooser, error) {
	byBody, err := slotKeys(ctx, tx, playbookID)
	if err != nil {
		return nil, fmt.Errorf("read statement keys: %w", err)
	}
	return &keyChooser{byBody: byBody, used: map[string]bool{}}, nil
}

func (kc *keyChooser) choose(i int, sp IngestStatementParams) (string, error) {
	key := strings.ToLower(strings.TrimSpace(sp.Key))
	if key != "" && !uuidRE.MatchString(key) {
		return "", fmt.Errorf("statement %d: key %q is not a statement key", i, sp.Key)
	}
	if key == "" {
		key = kc.byBody[sp.BodyMD]
	}
	if key == "" || kc.used[key] {
		return "", nil
	}
	kc.used[key] = true
	return key, nil
}

// IngestPlaybook writes a full playbook atomically. The transaction is rolled
// back if any statement has zero citations, enforcing the citation guarantee at
// the DB layer — unless AllowIncomplete relaxes it for a human's draft
// (ADR-013), in which case the gaps surface as page issues instead.
func (pg *PG) IngestPlaybook(ctx context.Context, params IngestPlaybookParams) error {
	status := params.Status
	if status == "" {
		status = "published"
	}
	// AllowIncomplete captures a person's part-finished work. The drafting
	// agent and the seeding tools have no such excuse, and anything published
	// must be whole — so an incomplete non-draft is refused outright.
	if params.AllowIncomplete && status != "draft" {
		return fmt.Errorf("AllowIncomplete only applies to drafts, not status %q", status)
	}
	pageKind := params.PageKind
	if pageKind == "" {
		pageKind = "playbook"
	}
	return pgx.BeginTxFunc(ctx, pg.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if !params.AllowIncomplete {
			if err := validateStatuteLocators(ctx, tx, params.Statements); err != nil {
				return err
			}
		}
		// A slot now holds at most one row per status, so the row to replace is
		// the one with the SAME status. Re-drafting a page that is already
		// published updates the draft beside it and leaves the live page alone.
		// (ON CONFLICT cannot express this: the uniqueness is enforced by two
		// partial indexes, and the applicable one depends on status.)
		var playbookID int64
		err := tx.QueryRow(ctx, `
			SELECT id FROM playbooks
			 WHERE jurisdiction_id = $1 AND topic_id = $2 AND language = $3 AND status = $4`,
			params.JurisdictionID, params.TopicID, params.Language, status,
		).Scan(&playbookID)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			err = tx.QueryRow(ctx, `
				INSERT INTO playbooks (jurisdiction_id, topic_id, language, slug, title, intro_md, status, page_kind, updated_by, updated_at, published_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(),
				        CASE WHEN $7 = 'published' THEN NOW() ELSE NULL END)
				RETURNING id`,
				params.JurisdictionID, params.TopicID, params.Language,
				params.Slug, params.Title, params.IntroMD, status, pageKind, params.UpdatedBy,
			).Scan(&playbookID)
		case err != nil:
			return fmt.Errorf("find playbook slot: %w", err)
		default:
			_, err = tx.Exec(ctx, `
				UPDATE playbooks
				   SET slug = $2, title = $3, intro_md = $4, page_kind = $5, updated_by = $6, updated_at = NOW(),
				       published_at = CASE WHEN status = 'published'
				                           THEN COALESCE(published_at, NOW()) ELSE published_at END
				 WHERE id = $1`,
				playbookID, params.Slug, params.Title, params.IntroMD, pageKind, params.UpdatedBy)
		}
		if err != nil {
			return fmt.Errorf("upsert playbook: %w", err)
		}

		keys, err := newKeyChooser(ctx, tx, playbookID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM playbook_statements WHERE playbook_id = $1`, playbookID); err != nil {
			return fmt.Errorf("clear playbook_statements: %w", err)
		}

		for i, sp := range params.Statements {
			if len(sp.Sources) == 0 && !params.AllowIncomplete {
				return fmt.Errorf("statement %d has no citations — ingest aborted", i)
			}

			if err := writeStatement(ctx, tx, keys, params.JurisdictionID, playbookID, i, sp, params.UpdatedBy); err != nil {
				return err
			}
		}
		// Ingesting straight to published is publishing: the person running
		// the tool reviewed the page as a page (ADR-018 D3).
		if status == "published" {
			if err := stampUnreviewed(ctx, tx, playbookID, params.UpdatedBy); err != nil {
				return err
			}
		}
		return nil
	})
}

// writeStatement is the one place a statement lands on a page: the row, its
// citations, its position link, and any reviewer note filed as a work item
// (ADR-018 D1). Shared by IngestPlaybook and AuthorUpdatePlaybook so the
// two save paths cannot drift.
func writeStatement(ctx context.Context, tx pgx.Tx, keys *keyChooser, jurisdictionID, playbookID int64, i int, sp IngestStatementParams, by string) error {
	key, err := keys.choose(i, sp)
	if err != nil {
		return err
	}
	stmtID, err := insertStatement(ctx, tx, jurisdictionID, sp, key)
	if err != nil {
		return fmt.Errorf("insert statement %d: %w", i, err)
	}
	for _, cite := range sp.Sources {
		if _, err := tx.Exec(ctx, insertCitationSQL, citationArgs(stmtID, cite)...); err != nil {
			return fmt.Errorf("insert citation for statement %d: %w", i, err)
		}
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO playbook_statements (playbook_id, statement_id, position)
		VALUES ($1, $2, $3)`, playbookID, stmtID, i,
	); err != nil {
		return fmt.Errorf("link statement %d to playbook: %w", i, err)
	}
	if err := fileReviewerFlag(ctx, tx, key, playbookID, sp.ReviewerNote, by); err != nil {
		return fmt.Errorf("file reviewer note on statement %d: %w", i, err)
	}
	if isReviewer(by) {
		if err := stampIfWritten(ctx, tx, stmtID, by); err != nil {
			return fmt.Errorf("stamp statement %d: %w", i, err)
		}
	}
	return nil
}

// ---- scan helpers ----------------------------------------------------------

func scanJurisdiction(row pgx.Row) (Jurisdiction, error) {
	var j Jurisdiction
	err := row.Scan(&j.ID, &j.ParentID, &j.Kind, &j.Name, &j.Slug, &j.ParentSlug, &j.ParentName)
	if errors.Is(err, pgx.ErrNoRows) {
		return j, ErrNotFound
	}
	return j, err
}

func scanJurisdictions(rows pgx.Rows) ([]Jurisdiction, error) {
	var out []Jurisdiction
	for rows.Next() {
		var j Jurisdiction
		if err := rows.Scan(&j.ID, &j.ParentID, &j.Kind, &j.Name, &j.Slug, &j.ParentSlug, &j.ParentName); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// assembleStatements collapses multiple citation rows (one per citation) into
// CitedStatement values. Statements are returned in position order.
func assembleStatements(rows pgx.Rows) []CitedStatement {
	type key struct{ id, position int64 }
	var order []key
	idx := map[key]int{}
	var out []CitedStatement

	for rows.Next() {
		var (
			stmtID       int64
			stmtKey      string
			bodyMD       string
			conceptSlug  string
			topicRefSlug string
			topicRefName string
			position     int
			c            CitationWithSource
			// The citation columns are nullable: the authoring query LEFT
			// JOINs citations so a draft's uncited statement (legal since
			// ADR-013) still comes back, with no chips.
			sourceID         *int64
			locator, quote   *string
			manuallyVerified *bool
			checkedBy        *string
			url, pub, kind   *string
		)
		var (
			reviewedAt *time.Time
			reviewedBy string
			undecided  bool
		)
		if err := rows.Scan(
			&stmtID, &stmtKey, &bodyMD, &conceptSlug, &topicRefSlug, &topicRefName, &position,
			&sourceID, &locator, &quote, &manuallyVerified, &c.CheckedAt, &checkedBy,
			&url, &pub, &kind,
			&reviewedAt, &reviewedBy, &undecided,
		); err != nil {
			continue
		}
		k := key{stmtID, int64(position)}
		i, ok := idx[k]
		if !ok {
			i = len(out)
			out = append(out, CitedStatement{
				ID: stmtID, Key: stmtKey, BodyMD: bodyMD, ConceptSlug: conceptSlug,
				TopicRefSlug: topicRefSlug, TopicRefName: topicRefName,
				ReviewedAt: reviewedAt, ReviewedBy: reviewedBy, Undecided: undecided,
			})
			idx[k] = i
			order = append(order, k)
		}
		if sourceID == nil {
			continue
		}
		c.SourceID = *sourceID
		c.Locator, c.Quote = deref(locator), deref(quote)
		c.ManuallyVerified = manuallyVerified != nil && *manuallyVerified
		c.CheckedBy = deref(checkedBy)
		c.SourceURL, c.Publisher, c.SourceKind = deref(url), deref(pub), deref(kind)
		out[i].Citations = append(out[i].Citations, c)
	}
	_ = order
	return out
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ---- Authoring -------------------------------------------------------------

// AuthorGetPlaybook fetches any playbook by ID regardless of status, used for reference/preview.
func (pg *PG) AuthorGetPlaybook(ctx context.Context, id int64) (PlaybookWithStatements, error) {
	var p PlaybookWithStatements
	err := pg.pool.QueryRow(ctx, `
		SELECT
			pb.id, pb.jurisdiction_id, pb.topic_id, pb.language,
			pb.slug, pb.title, pb.intro_md, pb.status, pb.page_kind, pb.author_notes, pb.last_reviewed_at,
			pb.created_at, pb.updated_at, pb.published_at, pb.updated_by,
			j.id, j.parent_id, j.kind, j.name, j.slug, COALESCE(pj.slug, ''),
			t.id, t.slug, t.name
		FROM playbooks pb
		JOIN jurisdictions j ON j.id = pb.jurisdiction_id
		LEFT JOIN jurisdictions pj ON pj.id = j.parent_id
		JOIN topics        t ON t.id  = pb.topic_id
		WHERE pb.id = $1`, id,
	).Scan(
		&p.Playbook.ID, &p.Playbook.JurisdictionID, &p.Playbook.TopicID,
		&p.Playbook.Language, &p.Playbook.Slug, &p.Playbook.Title,
		&p.Playbook.IntroMD, &p.Playbook.Status, &p.Playbook.PageKind, &p.Playbook.AuthorNotes, &p.Playbook.LastReviewedAt,
		&p.Playbook.CreatedAt, &p.Playbook.UpdatedAt, &p.Playbook.PublishedAt, &p.Playbook.UpdatedBy,
		&p.Jurisdiction.ID, &p.Jurisdiction.ParentID, &p.Jurisdiction.Kind,
		&p.Jurisdiction.Name, &p.Jurisdiction.Slug, &p.Jurisdiction.ParentSlug,
		&p.Topic.ID, &p.Topic.Slug, &p.Topic.Name,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return p, ErrNotFound
		}
		return p, err
	}
	statementRows, err := pg.pool.Query(ctx, `
		SELECT
			s.id, s.key::text, s.body_md, COALESCE(co.slug, ''), COALESCE(tr.slug, ''), COALESCE(tr.name, ''), ps.position,
			c.source_id, c.locator, c.quote, c.manually_verified, c.checked_at, c.checked_by,
			src.url, src.publisher, src.kind,
			`+reviewedAtSQL+`, `+reviewedBySQL+`, `+undecidedSQL+`
		FROM playbook_statements ps
		JOIN statements s   ON s.id  = ps.statement_id
		JOIN statement_review_hash h ON h.statement_id = s.id
		LEFT JOIN concepts co ON co.id = s.concept_id
		LEFT JOIN topics   tr ON tr.id = s.topic_ref
		LEFT JOIN citations  c   ON c.statement_id = s.id
		LEFT JOIN sources    src ON src.id = c.source_id
		WHERE ps.playbook_id = $1
		ORDER BY ps.position, c.source_id`, p.Playbook.ID)
	if err != nil {
		return p, err
	}
	defer statementRows.Close()
	p.Statements = assembleStatements(statementRows)
	if err := statementRows.Err(); err != nil {
		return p, err
	}
	statementRows.Close()
	return p, pg.attachNotes(ctx, &p)
}

// attachNotes loads the pending reviewer notes (ADR-018 D1) for every
// statement on the page, so the editor and the view can show the doubt
// beside the claim rather than pointing at the queue.
func (pg *PG) attachNotes(ctx context.Context, p *PlaybookWithStatements) error {
	rows, err := pg.pool.Query(ctx, `
		SELECT s.key::text, sp.id, COALESCE(sp.evidence->>'note', ''), sp.proposed_by, sp.created_at
		FROM playbook_statements ps
		JOIN statements s ON s.id = ps.statement_id
		JOIN statement_proposals sp ON sp.statement_key = s.key
		WHERE ps.playbook_id = $1 AND sp.reason = $2 AND sp.status = 'pending'
		ORDER BY sp.id`, p.Playbook.ID, ReasonReviewerFlag)
	if err != nil {
		return fmt.Errorf("load reviewer notes: %w", err)
	}
	defer rows.Close()
	byKey := map[string][]StatementNote{}
	for rows.Next() {
		var key string
		var n StatementNote
		if err := rows.Scan(&key, &n.ID, &n.Note, &n.By, &n.At); err != nil {
			return err
		}
		byKey[key] = append(byKey[key], n)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range p.Statements {
		p.Statements[i].Notes = byKey[p.Statements[i].Key]
	}
	return nil
}

// AuthorUpdatePlaybook replaces a playbook's metadata and all its statements in one transaction.
// Unlike IngestPlaybook, it targets by ID so city/topic can be changed on drafts.
// AuthorUpdatePlaybook saves whatever the author has (ADR-013). A draft may
// be incomplete in any way — uncited statements, missing quotes, no title —
// and still saves: the issues surface on the dashboard and block publishing,
// not saving. A page that is already published is different: this save
// rewrites what renters are reading right now, so it must pass the same gate
// publishing does, and a save that would break the live page is refused with
// a NotPublishableError and writes nothing.
func (pg *PG) AuthorUpdatePlaybook(ctx context.Context, params AuthorUpdatePlaybookParams) error {
	return pgx.BeginTxFunc(ctx, pg.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return authorUpdatePlaybookTx(ctx, tx, params)
	})
}

// authorUpdatePlaybookTx is AuthorUpdatePlaybook inside a caller's
// transaction. Approving a statement proposal (ADR-014 D3) is exactly this
// save plus a status change on the proposal, and the two must land or fail
// together.
func authorUpdatePlaybookTx(ctx context.Context, tx pgx.Tx, params AuthorUpdatePlaybookParams) error {
	{
		var status string
		err := tx.QueryRow(ctx, `SELECT status FROM playbooks WHERE id = $1`, params.ID).Scan(&status)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("read playbook %d: %w", params.ID, err)
		}
		pageKind := params.PageKind
		if pageKind == "" {
			pageKind = "playbook"
		}
		if _, err := tx.Exec(ctx, `
			UPDATE playbooks
			SET jurisdiction_id = $2, topic_id = $3, language = $4,
			    slug = $5, title = $6, intro_md = $7, page_kind = $8,
			    author_notes = $9, updated_by = $10, updated_at = NOW()
			WHERE id = $1`,
			params.ID, params.JurisdictionID, params.TopicID,
			params.Language, params.Slug, params.Title, params.IntroMD, pageKind,
			params.AuthorNotes, params.UpdatedBy,
		); err != nil {
			return fmt.Errorf("update playbook: %w", err)
		}

		keys, err := newKeyChooser(ctx, tx, params.ID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM playbook_statements WHERE playbook_id = $1`, params.ID); err != nil {
			return fmt.Errorf("clear playbook_statements: %w", err)
		}

		for i, sp := range params.Statements {
			if err := writeStatement(ctx, tx, keys, params.JurisdictionID, params.ID, i, sp, params.UpdatedBy); err != nil {
				return err
			}
		}

		// The gate runs after the write so it judges exactly what was saved,
		// and inside the tx so a refusal rolls the whole save back.
		if status == "published" {
			if err := validatePublishable(ctx, tx, params.ID); err != nil {
				return err
			}
		}

		// Re-saving detaches the previous statement rows; with autosave doing
		// this every minute they would pile up forever. Deleted last so the
		// citation inserts above could still inherit checked_at/checked_by
		// from the rows being replaced (see insertCitationSQL) — the new rows
		// carry those stamps forward, so nothing is lost by dropping the old.
		if _, err := tx.Exec(ctx, `
			DELETE FROM statements s
			WHERE NOT EXISTS (SELECT 1 FROM playbook_statements ps WHERE ps.statement_id = s.id)`,
		); err != nil {
			return fmt.Errorf("clean up detached statements: %w", err)
		}
		return nil
	}
}

// AuthorListPlaybooks returns all playbooks (draft and published) for the authoring dashboard.
func (pg *PG) AuthorListPlaybooks(ctx context.Context) ([]AuthorPlaybookRow, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT pb.id, pb.title, j.name, j.slug, t.slug, pb.language, pb.status, pb.page_kind,
		       pb.created_at, pb.updated_at, pb.published_at, pb.updated_by,
		       (SELECT count(*) FROM playbook_statements ps WHERE ps.playbook_id = pb.id),
		       (SELECT count(DISTINCT c.source_id)
		          FROM playbook_statements ps JOIN citations c ON c.statement_id = ps.statement_id
		         WHERE ps.playbook_id = pb.id),
		       -- A draft sitting in a slot that already has a live page is a
		       -- proposed revision: publishing it replaces that page rather
		       -- than adding one, so the dashboard has to say so.
		       EXISTS (SELECT 1 FROM playbooks live
		                WHERE live.jurisdiction_id = pb.jurisdiction_id
		                  AND live.topic_id = pb.topic_id
		                  AND live.language = pb.language
		                  AND live.status = 'published' AND live.id <> pb.id)
		FROM playbooks pb
		JOIN jurisdictions j ON j.id = pb.jurisdiction_id
		JOIN topics        t ON t.id  = pb.topic_id
		ORDER BY pb.status DESC, j.name, t.slug`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuthorPlaybookRow
	for rows.Next() {
		var r AuthorPlaybookRow
		if err := rows.Scan(&r.ID, &r.Title, &r.JurisdictionName, &r.JurisdictionSlug,
			&r.TopicSlug, &r.Language, &r.Status, &r.PageKind, &r.CreatedAt, &r.UpdatedAt,
			&r.PublishedAt, &r.UpdatedBy, &r.StatementCount, &r.SourceCount, &r.RevisesPublished); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// AuthorFindDraft returns the id of the draft occupying a slot, or ErrNotFound.
// The one-draft-per-slot index makes this at most one row. The authoring
// portal uses it to land on the page a save just created, and to refuse an
// autosave that would silently overwrite a draft someone else already has in
// the slot.
func (pg *PG) AuthorFindDraft(ctx context.Context, jurisdictionID, topicID int64, language string) (int64, error) {
	var id int64
	err := pg.pool.QueryRow(ctx, `
		SELECT id FROM playbooks
		 WHERE jurisdiction_id = $1 AND topic_id = $2 AND language = $3 AND status = 'draft'`,
		jurisdictionID, topicID, language).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return id, err
}

// AuthorPublishPlaybook sets a playbook's status to "published", recording
// actor as the person who signed it off.
func (pg *PG) AuthorPublishPlaybook(ctx context.Context, id int64, actor string) error {
	// Publishing is the review sign-off: nothing reaches the public site
	// without a human clicking this. Stamping last_reviewed_at here is what
	// backs the reviewedBy claim the page emits in its JSON-LD, and what the
	// sitemap reports as lastmod. Re-publishing an edited page re-stamps it,
	// which is correct — the human looked at it again.
	//
	// When this playbook is a revision of a live page, publishing swaps them:
	// the page being replaced is retired to 'superseded', not deleted, so what
	// it used to say survives. Retiring must happen first or the one-published-
	// per-slot index rejects the swap.
	return pgx.BeginTxFunc(ctx, pg.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if err := validatePublishable(ctx, tx, id); err != nil {
			return err
		}
		var jurisdictionID, topicID int64
		var language string
		err := tx.QueryRow(ctx,
			`SELECT jurisdiction_id, topic_id, language FROM playbooks WHERE id = $1`, id,
		).Scan(&jurisdictionID, &topicID, &language)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("read playbook %d: %w", id, err)
		}

		if _, err := tx.Exec(ctx, `
			UPDATE playbooks SET status = 'superseded', updated_by = $5, updated_at = NOW()
			 WHERE jurisdiction_id = $1 AND topic_id = $2 AND language = $3
			   AND status = 'published' AND id <> $4`,
			jurisdictionID, topicID, language, id, actor); err != nil {
			return fmt.Errorf("retire the page being replaced: %w", err)
		}

		if _, err := tx.Exec(ctx, `
			UPDATE playbooks SET status = 'published', updated_by = $2, updated_at = NOW(),
			     published_at = COALESCE(published_at, NOW()), last_reviewed_at = NOW()
			 WHERE id = $1`, id, actor); err != nil {
			return fmt.Errorf("publish playbook %d: %w", id, err)
		}
		// The gate already required every statement reviewed, except on a
		// directory page, which is reviewed as a page (ADR-018 D3): its
		// statements are stamped by the publisher here.
		return stampUnreviewed(ctx, tx, id, actor)
	})
}

// AuthorDeletePlaybook deletes a playbook, its orphaned statements, and any
// resulting orphaned jurisdiction/topic rows so they don't surface on the site.
func (pg *PG) AuthorDeletePlaybook(ctx context.Context, id int64) error {
	_, err := pg.pool.Exec(ctx, `
		WITH deleted AS (
			DELETE FROM playbooks WHERE id = $1
		)
		DELETE FROM statements
		WHERE id NOT IN (SELECT DISTINCT statement_id FROM playbook_statements)`, id)
	return err
}

// ErrNotFound is returned when a requested row does not exist.
var ErrNotFound = errors.New("not found")

// ErrDraftExists reports that the slot already holds a draft, so taking this
// page down needs a decision first. Only one draft may exist per (jurisdiction,
// topic, language) — see migration 000015 — so the waiting draft has to be
// retired for the live page to become the draft.
//
// It is a question, not a refusal. Call AuthorUnpublishPlaybook again with
// retireExistingDraft once the reviewer has been shown what will happen.
var ErrDraftExists = errors.New("a draft already exists for this page")

// ErrNotPublished is returned when unpublishing something that is not live.
var ErrNotPublished = errors.New("playbook is not published")

// AuthorUnpublishPlaybook takes a live page down and keeps its content as the
// slot's draft, so it can be corrected and published again.
//
// Nothing is deleted and nothing is overwritten. A page that is wrong in front
// of renters needs to come down faster than a correction can be researched, and
// until now the only ways off the live site were deleting the page — which
// destroys the text and 404s a URL with search traffic — or publishing a
// replacement, which requires having one ready.
//
// When the slot already holds a draft, the first call returns ErrDraftExists so
// the caller can show what is about to happen. Calling again with
// retireExistingDraft moves that draft to 'superseded' and proceeds: the draft
// leaves the slot but keeps its statements, citations and quotes, and can be
// read back from the Replaced tab. Nothing is deleted, and the reviewer chose.
//
// The alternative — a hard refusal — was worse: correcting a live page is a
// normal act, and a stale draft in the way should not stop it.
//
// published_at is deliberately left set. It records that this page was live
// once, which is what the sitemap's lastmod and the "Published" line in the
// dashboard read; clearing it would erase that history to no benefit.
func (pg *PG) AuthorUnpublishPlaybook(ctx context.Context, id int64, retireExistingDraft bool, actor string) error {
	return pgx.BeginTxFunc(ctx, pg.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var jurisdictionID, topicID int64
		var language, status string
		err := tx.QueryRow(ctx,
			`SELECT jurisdiction_id, topic_id, language, status FROM playbooks WHERE id = $1`, id,
		).Scan(&jurisdictionID, &topicID, &language, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("read playbook %d: %w", id, err)
		}
		if status != "published" {
			return ErrNotPublished
		}

		// Checked inside the transaction so a draft created concurrently cannot
		// slip in between the check and the update. The partial index would
		// reject that write anyway; this turns it into an explainable answer
		// rather than a constraint-violation error.
		var otherDraft bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM playbooks
				 WHERE jurisdiction_id = $1 AND topic_id = $2 AND language = $3
				   AND status = 'draft' AND id <> $4
			)`, jurisdictionID, topicID, language, id).Scan(&otherDraft); err != nil {
			return fmt.Errorf("check for an existing draft: %w", err)
		}
		if otherDraft {
			if !retireExistingDraft {
				return ErrDraftExists
			}
			// Retired, not deleted: the draft keeps its statements, citations
			// and quotes and stays readable under Replaced. Superseded rows
			// carry no unique index, so several may accumulate in a slot.
			if _, err := tx.Exec(ctx, `
				UPDATE playbooks SET status = 'superseded', updated_by = $5, updated_at = NOW()
				 WHERE jurisdiction_id = $1 AND topic_id = $2 AND language = $3
				   AND status = 'draft' AND id <> $4`,
				jurisdictionID, topicID, language, id, actor); err != nil {
				return fmt.Errorf("retire the waiting draft: %w", err)
			}
		}

		if _, err := tx.Exec(ctx, `
			UPDATE playbooks SET status = 'draft', updated_by = $2, updated_at = NOW() WHERE id = $1`, id, actor); err != nil {
			return fmt.Errorf("unpublish playbook %d: %w", id, err)
		}
		return nil
	})
}

// ErrAliasShadowsLiveSlug is returned when an alias would be created for a slug
// that is still live. Lookups try live slugs first, so such an alias could never
// be reached — it would be dead weight that misleads whoever reads the table.
var ErrAliasShadowsLiveSlug = errors.New("alias would shadow a live slug")
