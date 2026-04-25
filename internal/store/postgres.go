package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

// GetJurisdictionBySlug looks up one jurisdiction by its URL slug.
func (pg *PG) GetJurisdictionBySlug(ctx context.Context, slug string) (Jurisdiction, error) {
	row := pg.pool.QueryRow(ctx, `
		SELECT id, parent_id, kind, name, slug
		FROM jurisdictions WHERE slug = $1`, slug)
	return scanJurisdiction(row)
}

// ListTopicsByJurisdiction returns topics that have a playbook for the given jurisdiction + language.
func (pg *PG) ListTopicsByJurisdiction(ctx context.Context, jurisdictionID int64, language string) ([]Topic, error) {
	rows, err := pg.pool.Query(ctx, `
		SELECT t.id, t.slug, t.name
		FROM topics t
		JOIN playbooks p ON p.topic_id = t.id
		WHERE p.jurisdiction_id = $1 AND p.language = $2
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
			pb.slug, pb.title, pb.intro_md, pb.last_reviewed_at,
			j.id, j.parent_id, j.kind, j.name, j.slug,
			t.id, t.slug, t.name
		FROM playbooks pb
		JOIN jurisdictions j ON j.id = pb.jurisdiction_id
		JOIN topics        t ON t.id  = pb.topic_id
		WHERE j.slug = $1 AND t.slug = $2 AND pb.language = $3`,
		jurisdictionSlug, topicSlug, language,
	).Scan(
		&p.Playbook.ID, &p.Playbook.JurisdictionID, &p.Playbook.TopicID,
		&p.Playbook.Language, &p.Playbook.Slug, &p.Playbook.Title,
		&p.Playbook.IntroMD, &p.Playbook.LastReviewedAt,
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
			c.source_id, c.locator,
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
	return pgx.BeginTxFunc(ctx, pg.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var playbookID int64
		err := tx.QueryRow(ctx, `
			INSERT INTO playbooks (jurisdiction_id, topic_id, language, slug, title, intro_md)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (jurisdiction_id, topic_id, language) DO UPDATE
			    SET slug = EXCLUDED.slug, title = EXCLUDED.title, intro_md = EXCLUDED.intro_md
			RETURNING id`,
			params.JurisdictionID, params.TopicID, params.Language,
			params.Slug, params.Title, params.IntroMD,
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
					INSERT INTO citations (statement_id, source_id, locator)
					VALUES ($1, $2, $3)
					ON CONFLICT (statement_id, source_id) DO UPDATE SET locator = EXCLUDED.locator`,
					stmtID, cite.SourceID, cite.Locator,
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
			&c.SourceID, &c.Locator, &c.SourceURL, &c.Publisher, &c.SourceKind,
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

// ErrNotFound is returned when a requested row does not exist.
var ErrNotFound = errors.New("not found")
