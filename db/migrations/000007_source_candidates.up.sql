-- Source-discovery queue. A scraper/registry proposes candidate primary sources
-- for a jurisdiction; the author triages each into the sources table (approve) or
-- discards it (reject/snooze). Candidates never become statements automatically —
-- they only populate the author's research shelf (preserves the citation guarantee).
CREATE TABLE source_candidates (
    id              BIGSERIAL PRIMARY KEY,
    jurisdiction_id BIGINT REFERENCES jurisdictions(id),
    url             TEXT NOT NULL,
    publisher       TEXT NOT NULL DEFAULT '',
    title           TEXT NOT NULL DEFAULT '',
    -- kind_guess mirrors the sources.kind enum (kept in sync with migration 000006).
    kind_guess      TEXT NOT NULL CHECK (kind_guess IN
                        ('statute', 'regulation', 'gov_guidance', 'nonprofit', 'editorial', 'court_ruling')),
    rationale       TEXT NOT NULL DEFAULT '',
    confidence      REAL NOT NULL DEFAULT 0,           -- 0..1, used to rank the queue
    discovered_via  TEXT NOT NULL DEFAULT 'registry',  -- registry|search (Tier 2 is future work)
    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN
                        ('pending', 'approved', 'rejected', 'snoozed')),
    source_id       BIGINT REFERENCES sources(id),     -- set when approved
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at     TIMESTAMPTZ,
    -- Re-running discovery for a jurisdiction is idempotent: an existing
    -- candidate URL is left untouched (its triage status is preserved).
    UNIQUE (jurisdiction_id, url)
);

CREATE INDEX source_candidates_status_idx ON source_candidates (status, jurisdiction_id);
