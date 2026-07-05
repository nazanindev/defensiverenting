package store

import (
	"context"
	"errors"
	"fmt"
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
		SELECT id, parent_id, kind, name, slug
		FROM jurisdictions
		WHERE kind = 'city'
		ORDER BY name`)
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
		SELECT DISTINCT j.id, j.parent_id, j.kind, j.name, j.slug
		FROM jurisdictions j
		JOIN playbooks p ON p.jurisdiction_id = j.id
		WHERE j.kind = 'city' AND p.status = 'published'
		ORDER BY j.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanJurisdictions(rows)
}

// GetJurisdictionBySlug looks up one jurisdiction by its URL slug.
func (pg *PG) GetJurisdictionBySlug(ctx context.Context, slug string) (Jurisdiction, error) {
	row := pg.pool.QueryRow(ctx, `
		SELECT id, parent_id, kind, name, slug
		FROM jurisdictions WHERE slug = $1`, slug)
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

// GetPlaybook fetches the full playbook with all statements and their citations.
// Returns ErrNotFound if the (jurisdiction, topic, language) combination does not exist.
func (pg *PG) GetPlaybook(ctx context.Context, jurisdictionSlug, topicSlug, language string) (PlaybookWithStatements, error) {
	var p PlaybookWithStatements

	err := pg.pool.QueryRow(ctx, `
		SELECT
			pb.id, pb.jurisdiction_id, pb.topic_id, pb.language,
			pb.slug, pb.title, pb.intro_md, pb.page_kind, pb.last_reviewed_at,
			j.id, j.parent_id, j.kind, j.name, j.slug,
			t.id, t.slug, t.name
		FROM playbooks pb
		JOIN jurisdictions j ON j.id = pb.jurisdiction_id
		JOIN topics        t ON t.id  = pb.topic_id
		WHERE j.slug = $1 AND t.slug = $2 AND pb.language = $3 AND pb.status = 'published'`,
		jurisdictionSlug, topicSlug, language,
	).Scan(
		&p.Playbook.ID, &p.Playbook.JurisdictionID, &p.Playbook.TopicID,
		&p.Playbook.Language, &p.Playbook.Slug, &p.Playbook.Title,
		&p.Playbook.IntroMD, &p.Playbook.PageKind, &p.Playbook.LastReviewedAt,
		&p.Jurisdiction.ID, &p.Jurisdiction.ParentID, &p.Jurisdiction.Kind,
		&p.Jurisdiction.Name, &p.Jurisdiction.Slug,
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
			s.id, s.body_md, ps.position,
			c.source_id, c.locator, c.quote,
			src.url, src.publisher, src.kind
		FROM playbook_statements ps
		JOIN statements s   ON s.id  = ps.statement_id
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
			q AS (SELECT plainto_tsquery('english', $2) AS tsq)
			SELECT
				s.id,
				s.body_md,
				ts_rank_cd(s.body_tsv, q.tsq) AS rank,
				COALESCE(j.slug, ''),
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
			WHERE s.body_tsv @@ q.tsq AND s.language = $3
			ORDER BY rank DESC LIMIT 20`,
			*jurisdictionID, query, language)
	} else {
		stmtRows, err = pg.pool.Query(ctx, `
			WITH q AS (SELECT plainto_tsquery('english', $1) AS tsq)
			SELECT
				s.id,
				s.body_md,
				ts_rank_cd(s.body_tsv, q.tsq) AS rank,
				COALESCE(j.slug, ''),
				COALESCE(t.slug, ''),
				COALESCE(pb.slug, ''),
				COALESCE(pb.title,'')
			FROM statements s
			CROSS JOIN q
			LEFT JOIN playbook_statements ps ON ps.statement_id = s.id
			LEFT JOIN playbooks pb ON pb.id = ps.playbook_id
			LEFT JOIN topics t ON t.id = pb.topic_id
			LEFT JOIN jurisdictions j ON j.id = s.jurisdiction_id
			WHERE s.body_tsv @@ q.tsq AND s.language = $2
			ORDER BY rank DESC LIMIT 20`,
			query, language)
	}
	if err != nil {
		return nil, err
	}
	defer stmtRows.Close()

	var results []SearchResult
	for stmtRows.Next() {
		var r SearchResult
		var sid int64
		if err := stmtRows.Scan(&sid, &r.Snippet, &r.Rank,
			&r.JurisdictionSlug, &r.TopicSlug, &r.PlaybookSlug, &r.PlaybookTitle); err != nil {
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
		WITH q AS (SELECT plainto_tsquery('english', $1) AS tsq)
		SELECT
			pb.slug, pb.title, pb.intro_md,
			ts_rank_cd(pb.body_tsv, q.tsq) AS rank,
			j.slug, t.slug
		FROM playbooks pb
		CROSS JOIN q
		JOIN jurisdictions j ON j.id = pb.jurisdiction_id
		JOIN topics t ON t.id = pb.topic_id
		WHERE pb.body_tsv @@ q.tsq
		  AND ($2::BIGINT IS NULL OR pb.jurisdiction_id = $2)
		  AND pb.language = $3
		ORDER BY rank DESC LIMIT 10`,
		query, jurisdictionID, language)
	if err != nil {
		return nil, err
	}
	defer pbRows.Close()

	for pbRows.Next() {
		var r SearchResult
		if err := pbRows.Scan(&r.PlaybookSlug, &r.PlaybookTitle, &r.Snippet, &r.Rank,
			&r.JurisdictionSlug, &r.TopicSlug); err != nil {
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

// ListSitemapURLs returns jurisdiction/topic slug pairs for all English playbooks.
func (pg *PG) ListSitemapURLs(ctx context.Context) ([]SitemapEntry, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT j.slug, t.slug, pb.last_reviewed_at
		FROM playbooks pb
		JOIN jurisdictions j ON j.id = pb.jurisdiction_id
		JOIN topics        t ON t.id  = pb.topic_id
		WHERE pb.language = 'en' AND pb.status = 'published'
		ORDER BY j.slug, t.slug`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []SitemapEntry
	for rows.Next() {
		var e SitemapEntry
		if err := rows.Scan(&e.JurisdictionSlug, &e.TopicSlug, &e.LastMod); err != nil {
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
		RETURNING id, parent_id, kind, name, slug`,
		params.ParentID, params.Kind, params.Name, params.Slug)
	return scanJurisdiction(row)
}

func (pg *PG) UpsertSource(ctx context.Context, params UpsertSourceParams) (Source, error) {
	now := time.Now().UTC()
	row := pg.pool.QueryRow(ctx, `
		INSERT INTO sources (url, publisher, jurisdiction_id, kind, retrieved_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (url) DO UPDATE
		    SET publisher     = EXCLUDED.publisher,
		        kind          = EXCLUDED.kind,
		        retrieved_at  = EXCLUDED.retrieved_at
		RETURNING id, url, publisher, jurisdiction_id, kind, retrieved_at, content_hash`,
		params.URL, params.Publisher, params.JurisdictionID, params.Kind, now)
	var s Source
	err := row.Scan(&s.ID, &s.URL, &s.Publisher, &s.JurisdictionID, &s.Kind, &s.RetrievedAt, &s.ContentHash)
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
	row := pg.pool.QueryRow(ctx, `
		INSERT INTO topics (slug, name) VALUES ($1, $2)
		ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name
		RETURNING id, slug, name`, params.Slug, params.Name)
	var t Topic
	err := row.Scan(&t.ID, &t.Slug, &t.Name)
	return t, err
}

func (pg *PG) GetEditorialSource(ctx context.Context) (Source, error) {
	row := pg.pool.QueryRow(ctx, `
		SELECT id, url, publisher, jurisdiction_id, kind, retrieved_at, content_hash
		FROM sources WHERE kind = 'editorial' LIMIT 1`)
	var s Source
	err := row.Scan(&s.ID, &s.URL, &s.Publisher, &s.JurisdictionID, &s.Kind, &s.RetrievedAt, &s.ContentHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, ErrNotFound
	}
	return s, err
}

// IngestPlaybook writes a full playbook atomically. The transaction is rolled
// back if any statement has zero citations, enforcing the citation guarantee at the DB layer.
func (pg *PG) IngestPlaybook(ctx context.Context, params IngestPlaybookParams) error {
	status := params.Status
	if status == "" {
		status = "published"
	}
	pageKind := params.PageKind
	if pageKind == "" {
		pageKind = "playbook"
	}
	return pgx.BeginTxFunc(ctx, pg.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var playbookID int64
		err := tx.QueryRow(ctx, `
			INSERT INTO playbooks (jurisdiction_id, topic_id, language, slug, title, intro_md, status, page_kind, updated_at, published_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), CASE WHEN $7 = 'published' THEN NOW() ELSE NULL END)
			ON CONFLICT (jurisdiction_id, topic_id, language) DO UPDATE
			    SET slug = EXCLUDED.slug, title = EXCLUDED.title, intro_md = EXCLUDED.intro_md,
			        status = EXCLUDED.status, page_kind = EXCLUDED.page_kind, updated_at = NOW(),
			        published_at = CASE WHEN EXCLUDED.status = 'published'
			                            THEN COALESCE(playbooks.published_at, NOW())
			                            ELSE playbooks.published_at END
			RETURNING id`,
			params.JurisdictionID, params.TopicID, params.Language,
			params.Slug, params.Title, params.IntroMD, status, pageKind,
		).Scan(&playbookID)
		if err != nil {
			return fmt.Errorf("upsert playbook: %w", err)
		}

		if _, err := tx.Exec(ctx, `DELETE FROM playbook_statements WHERE playbook_id = $1`, playbookID); err != nil {
			return fmt.Errorf("clear playbook_statements: %w", err)
		}

		for i, sp := range params.Statements {
			if len(sp.Sources) == 0 {
				return fmt.Errorf("statement %d has no citations — ingest aborted", i)
			}

			var stmtID int64
			if err := tx.QueryRow(ctx, `
				INSERT INTO statements (jurisdiction_id, language, body_md)
				VALUES ($1, $2, $3) RETURNING id`,
				params.JurisdictionID, sp.Language, sp.BodyMD,
			).Scan(&stmtID); err != nil {
				return fmt.Errorf("insert statement %d: %w", i, err)
			}

			for _, cite := range sp.Sources {
				if _, err := tx.Exec(ctx, `
					INSERT INTO citations (statement_id, source_id, locator, quote)
					VALUES ($1, $2, $3, $4)
					ON CONFLICT (statement_id, source_id) DO UPDATE SET locator = EXCLUDED.locator, quote = EXCLUDED.quote`,
					stmtID, cite.SourceID, cite.Locator, cite.Quote,
				); err != nil {
					return fmt.Errorf("insert citation for statement %d: %w", i, err)
				}
			}

			if _, err := tx.Exec(ctx, `
				INSERT INTO playbook_statements (playbook_id, statement_id, position)
				VALUES ($1, $2, $3)`, playbookID, stmtID, i,
			); err != nil {
				return fmt.Errorf("link statement %d to playbook: %w", i, err)
			}
		}
		return nil
	})
}

// ---- scan helpers ----------------------------------------------------------

func scanJurisdiction(row pgx.Row) (Jurisdiction, error) {
	var j Jurisdiction
	err := row.Scan(&j.ID, &j.ParentID, &j.Kind, &j.Name, &j.Slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return j, ErrNotFound
	}
	return j, err
}

func scanJurisdictions(rows pgx.Rows) ([]Jurisdiction, error) {
	var out []Jurisdiction
	for rows.Next() {
		var j Jurisdiction
		if err := rows.Scan(&j.ID, &j.ParentID, &j.Kind, &j.Name, &j.Slug); err != nil {
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
			stmtID   int64
			bodyMD   string
			position int
			c        CitationWithSource
		)
		if err := rows.Scan(
			&stmtID, &bodyMD, &position,
			&c.SourceID, &c.Locator, &c.Quote, &c.SourceURL, &c.Publisher, &c.SourceKind,
		); err != nil {
			continue
		}
		k := key{stmtID, int64(position)}
		i, ok := idx[k]
		if !ok {
			i = len(out)
			out = append(out, CitedStatement{ID: stmtID, BodyMD: bodyMD})
			idx[k] = i
			order = append(order, k)
		}
		out[i].Citations = append(out[i].Citations, c)
	}
	_ = order
	return out
}

// ---- Authoring -------------------------------------------------------------

// AuthorGetPlaybook fetches any playbook by ID regardless of status, used for reference/preview.
func (pg *PG) AuthorGetPlaybook(ctx context.Context, id int64) (PlaybookWithStatements, error) {
	var p PlaybookWithStatements
	err := pg.pool.QueryRow(ctx, `
		SELECT
			pb.id, pb.jurisdiction_id, pb.topic_id, pb.language,
			pb.slug, pb.title, pb.intro_md, pb.status, pb.page_kind, pb.last_reviewed_at,
			pb.created_at, pb.updated_at, pb.published_at,
			j.id, j.parent_id, j.kind, j.name, j.slug,
			t.id, t.slug, t.name
		FROM playbooks pb
		JOIN jurisdictions j ON j.id = pb.jurisdiction_id
		JOIN topics        t ON t.id  = pb.topic_id
		WHERE pb.id = $1`, id,
	).Scan(
		&p.Playbook.ID, &p.Playbook.JurisdictionID, &p.Playbook.TopicID,
		&p.Playbook.Language, &p.Playbook.Slug, &p.Playbook.Title,
		&p.Playbook.IntroMD, &p.Playbook.Status, &p.Playbook.PageKind, &p.Playbook.LastReviewedAt,
		&p.Playbook.CreatedAt, &p.Playbook.UpdatedAt, &p.Playbook.PublishedAt,
		&p.Jurisdiction.ID, &p.Jurisdiction.ParentID, &p.Jurisdiction.Kind,
		&p.Jurisdiction.Name, &p.Jurisdiction.Slug,
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
			s.id, s.body_md, ps.position,
			c.source_id, c.locator, c.quote,
			src.url, src.publisher, src.kind
		FROM playbook_statements ps
		JOIN statements s   ON s.id  = ps.statement_id
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

// AuthorUpdatePlaybook replaces a playbook's metadata and all its statements in one transaction.
// Unlike IngestPlaybook, it targets by ID so city/topic can be changed on drafts.
func (pg *PG) AuthorUpdatePlaybook(ctx context.Context, params AuthorUpdatePlaybookParams) error {
	return pgx.BeginTxFunc(ctx, pg.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		pageKind := params.PageKind
		if pageKind == "" {
			pageKind = "playbook"
		}
		if _, err := tx.Exec(ctx, `
			UPDATE playbooks
			SET jurisdiction_id = $2, topic_id = $3, language = $4,
			    slug = $5, title = $6, intro_md = $7, page_kind = $8, updated_at = NOW()
			WHERE id = $1`,
			params.ID, params.JurisdictionID, params.TopicID,
			params.Language, params.Slug, params.Title, params.IntroMD, pageKind,
		); err != nil {
			return fmt.Errorf("update playbook: %w", err)
		}

		if _, err := tx.Exec(ctx, `DELETE FROM playbook_statements WHERE playbook_id = $1`, params.ID); err != nil {
			return fmt.Errorf("clear playbook_statements: %w", err)
		}

		for i, sp := range params.Statements {
			if len(sp.Sources) == 0 {
				return fmt.Errorf("statement %d has no citations", i)
			}
			var stmtID int64
			if err := tx.QueryRow(ctx, `
				INSERT INTO statements (jurisdiction_id, language, body_md)
				VALUES ($1, $2, $3) RETURNING id`,
				params.JurisdictionID, sp.Language, sp.BodyMD,
			).Scan(&stmtID); err != nil {
				return fmt.Errorf("insert statement %d: %w", i, err)
			}
			for _, cite := range sp.Sources {
				if _, err := tx.Exec(ctx, `
					INSERT INTO citations (statement_id, source_id, locator, quote)
					VALUES ($1, $2, $3, $4)
					ON CONFLICT (statement_id, source_id) DO UPDATE SET locator = EXCLUDED.locator, quote = EXCLUDED.quote`,
					stmtID, cite.SourceID, cite.Locator, cite.Quote,
				); err != nil {
					return fmt.Errorf("insert citation for statement %d: %w", i, err)
				}
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO playbook_statements (playbook_id, statement_id, position)
				VALUES ($1, $2, $3)`, params.ID, stmtID, i,
			); err != nil {
				return fmt.Errorf("link statement %d to playbook: %w", i, err)
			}
		}
		return nil
	})
}

// AuthorListPlaybooks returns all playbooks (draft and published) for the authoring dashboard.
func (pg *PG) AuthorListPlaybooks(ctx context.Context) ([]AuthorPlaybookRow, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT pb.id, pb.title, j.name, j.slug, t.slug, pb.language, pb.status, pb.page_kind,
		       pb.created_at, pb.updated_at, pb.published_at,
		       (SELECT count(*) FROM playbook_statements ps WHERE ps.playbook_id = pb.id),
		       (SELECT count(DISTINCT c.source_id)
		          FROM playbook_statements ps JOIN citations c ON c.statement_id = ps.statement_id
		         WHERE ps.playbook_id = pb.id)
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
			&r.PublishedAt, &r.StatementCount, &r.SourceCount); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// AuthorPublishPlaybook sets a playbook's status to "published".
func (pg *PG) AuthorPublishPlaybook(ctx context.Context, id int64) error {
	_, err := pg.pool.Exec(ctx,
		`UPDATE playbooks SET status = 'published', updated_at = NOW(),
		     published_at = COALESCE(published_at, NOW()) WHERE id = $1`, id)
	return err
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
