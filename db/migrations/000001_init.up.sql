-- Jurisdiction hierarchy: country → state → city.
-- A self-referential table lets us walk the ancestor chain with a single recursive CTE.
CREATE TABLE jurisdictions (
    id        BIGSERIAL PRIMARY KEY,
    parent_id BIGINT REFERENCES jurisdictions(id),
    kind      TEXT NOT NULL CHECK (kind IN ('country', 'state', 'city')),
    name      TEXT NOT NULL,
    slug      TEXT NOT NULL UNIQUE
);

-- A citable primary source. kind='editorial' is the site's own editorial guidance.
-- content_hash enables a future freshness-check worker to detect silent upstream changes.
CREATE TABLE sources (
    id              BIGSERIAL PRIMARY KEY,
    url             TEXT NOT NULL UNIQUE,
    publisher       TEXT NOT NULL,
    jurisdiction_id BIGINT REFERENCES jurisdictions(id),
    kind            TEXT NOT NULL CHECK (kind IN ('statute', 'regulation', 'gov_guidance', 'nonprofit', 'editorial')),
    retrieved_at    TIMESTAMPTZ,
    content_hash    TEXT
);

-- An atomic, citable claim. Every statement must have ≥1 citation (enforced transactionally at ingest).
-- body_tsv is a generated column so search stays zero-config.
CREATE TABLE statements (
    id              BIGSERIAL PRIMARY KEY,
    jurisdiction_id BIGINT      NOT NULL REFERENCES jurisdictions(id),
    language        TEXT        NOT NULL DEFAULT 'en',
    body_md         TEXT        NOT NULL,
    body_tsv        TSVECTOR    GENERATED ALWAYS AS (to_tsvector('english', body_md)) STORED,
    -- embedding vector(1536) -- nullable; add via: ALTER TABLE statements ADD COLUMN embedding vector(1536)
    last_reviewed_at TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX statements_tsv_idx ON statements USING GIN (body_tsv);

-- Join table: statement ↔ source. locator carries the statute section or URL anchor.
CREATE TABLE citations (
    statement_id BIGINT NOT NULL REFERENCES statements(id) ON DELETE CASCADE,
    source_id    BIGINT NOT NULL REFERENCES sources(id),
    locator      TEXT   NOT NULL DEFAULT '',
    PRIMARY KEY (statement_id, source_id)
);

CREATE TABLE topics (
    id   BIGSERIAL PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL
);

CREATE TABLE statement_topics (
    statement_id BIGINT NOT NULL REFERENCES statements(id) ON DELETE CASCADE,
    topic_id     BIGINT NOT NULL REFERENCES topics(id),
    PRIMARY KEY (statement_id, topic_id)
);

-- An ordered composition of statements scoped to (jurisdiction, topic, language).
-- body_tsv covers title + intro for full-text search over playbook titles.
CREATE TABLE playbooks (
    id              BIGSERIAL PRIMARY KEY,
    jurisdiction_id BIGINT NOT NULL REFERENCES jurisdictions(id),
    topic_id        BIGINT NOT NULL REFERENCES topics(id),
    language        TEXT   NOT NULL DEFAULT 'en',
    slug            TEXT   NOT NULL,
    title           TEXT   NOT NULL,
    intro_md        TEXT   NOT NULL DEFAULT '',
    body_tsv        TSVECTOR GENERATED ALWAYS AS (
                        to_tsvector('english', title || ' ' || intro_md)
                    ) STORED,
    last_reviewed_at TIMESTAMPTZ,
    UNIQUE (jurisdiction_id, topic_id, language)
);

CREATE INDEX playbooks_tsv_idx ON playbooks USING GIN (body_tsv);

CREATE TABLE playbook_statements (
    playbook_id  BIGINT NOT NULL REFERENCES playbooks(id)  ON DELETE CASCADE,
    statement_id BIGINT NOT NULL REFERENCES statements(id) ON DELETE CASCADE,
    position     INT    NOT NULL,
    PRIMARY KEY (playbook_id, statement_id)
);

-- Seed: canonical editorial source. All editorial guidance cites this row.
-- Populated by migration so it is always present and has a known URL.
INSERT INTO jurisdictions (kind, name, slug) VALUES
    ('country', 'United States', 'united-states'),
    ('state',   'Massachusetts', 'massachusetts'),
    ('city',    'Boston',        'boston');

UPDATE jurisdictions SET parent_id = (SELECT id FROM jurisdictions WHERE slug = 'united-states')
WHERE slug = 'massachusetts';

UPDATE jurisdictions SET parent_id = (SELECT id FROM jurisdictions WHERE slug = 'massachusetts')
WHERE slug = 'boston';

INSERT INTO sources (url, publisher, kind, retrieved_at)
VALUES (
    'https://tenantrights.example/editorial',
    'Tenant Rights editorial',
    'editorial',
    NOW()
);
