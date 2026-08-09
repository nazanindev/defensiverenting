-- Slug renames are permanent redirects, not breakages.
--
-- Every rename records the old slug here; browse handlers fall back to this
-- table on a miss and reply 301 to the canonical URL. Without it a slug change
-- silently 404s every indexed link to the old address, which is what the
-- 2026-08-01 topic cleanup did to the /t/pittsburgh-* URLs.
--
-- ADR-005 D4 specifies a single polymorphic target_id. Two nullable foreign
-- keys are used instead so the database can guarantee an alias never outlives
-- what it points at — a dangling alias would redirect to a 404, which is worse
-- than the 404 it was added to prevent.
CREATE TABLE slug_aliases (
    alias           TEXT        NOT NULL,
    namespace       TEXT        NOT NULL CHECK (namespace IN ('jurisdiction', 'topic')),
    jurisdiction_id BIGINT      REFERENCES jurisdictions(id) ON DELETE CASCADE,
    topic_id        BIGINT      REFERENCES topics(id)        ON DELETE CASCADE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (namespace, alias),
    CONSTRAINT slug_aliases_target_matches_namespace CHECK (
        (namespace = 'jurisdiction' AND jurisdiction_id IS NOT NULL AND topic_id IS NULL)
     OR (namespace = 'topic'        AND topic_id IS NOT NULL AND jurisdiction_id IS NULL)
    )
);

-- Redirect chains are resolved to their destination at write time, so lookups
-- are a single hop and these indexes stay small.
CREATE INDEX slug_aliases_jurisdiction_idx ON slug_aliases (jurisdiction_id) WHERE jurisdiction_id IS NOT NULL;
CREATE INDEX slug_aliases_topic_idx        ON slug_aliases (topic_id)        WHERE topic_id IS NOT NULL;
