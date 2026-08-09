-- Redirect the URLs the 2026-08-01 cleanup left 404ing (2026-08-09).
--
-- That cleanup merged city-prefixed topics into shared ones and deleted the old
-- rows. There was no redirect mechanism at the time, so every URL Google had
-- already indexed under the old slugs started returning 404. Search Console
-- reported this on 2026-08-09 as "Not found (404)".
--
-- Each retired slug was reachable at two addresses, so 13 retired topics is
-- around 26 indexed pages:
--   /j/pittsburgh/pittsburgh-cant-pay-rent   and   /t/pittsburgh-cant-pay-rent
--
-- topics_backup_20260801, written by that cleanup, is the authoritative record
-- of what was retired. Targets are derived by stripping the city prefix and
-- resolving what remains: directly if it is still a live topic, otherwise
-- through its own alias, so this is correct whether or not the vocabulary
-- migration has already folded that slug into a canonical one.
--
-- REQUIRES migration 000013 (slug_aliases). Run AFTER
-- 2026-08-09-migrate-topic-vocabulary.sql so chains resolve in one hop.
--
-- Idempotent.

BEGIN;

DO $$
BEGIN
    IF to_regclass('public.slug_aliases') IS NULL THEN
        RAISE EXCEPTION 'aborting: slug_aliases does not exist. Deploy migration 000013 first.';
    END IF;
    IF to_regclass('public.topics_backup_20260801') IS NULL THEN
        RAISE EXCEPTION 'aborting: topics_backup_20260801 is gone, so the retired slugs cannot be recovered from the database. Recover them from Search Console instead.';
    END IF;
END $$;

-- Retired slug -> the topic that now answers for it.
CREATE TEMP TABLE retired ON COMMIT DROP AS
WITH gone AS (
    SELECT b.slug
      FROM topics_backup_20260801 b
     WHERE NOT EXISTS (SELECT 1 FROM topics t WHERE t.slug = b.slug)
),
-- Strip the "{city}-" prefix using the jurisdiction list rather than a
-- hardcoded one, so this stays correct if more cities were affected.
stripped AS (
    SELECT g.slug AS old_slug,
           COALESCE(
               (SELECT substring(g.slug from length(j.slug) + 2)
                  FROM jurisdictions j
                 WHERE g.slug LIKE j.slug || '-%'
                 ORDER BY length(j.slug) DESC
                 LIMIT 1),
               g.slug
           ) AS base_slug
      FROM gone g
)
SELECT s.old_slug,
       COALESCE(live.id, chained.topic_id) AS target_id
  FROM stripped s
  LEFT JOIN topics live ON live.slug = s.base_slug
  LEFT JOIN slug_aliases chained
         ON chained.namespace = 'topic' AND chained.alias = s.base_slug
 WHERE s.base_slug <> s.old_slug;

INSERT INTO slug_aliases (alias, namespace, topic_id)
SELECT r.old_slug, 'topic', r.target_id
  FROM retired r
 WHERE r.target_id IS NOT NULL
   -- An alias may not shadow a live slug: lookups try live slugs first, so it
   -- could never be reached.
   AND NOT EXISTS (SELECT 1 FROM topics t WHERE t.slug = r.old_slug)
ON CONFLICT (namespace, alias) DO UPDATE SET topic_id = EXCLUDED.topic_id;

-- Every retired slug must now resolve, or its URLs stay 404 and this script
-- did not do its job.
DO $$
DECLARE unresolved TEXT;
BEGIN
    SELECT string_agg(r.old_slug, ', ') INTO unresolved
      FROM retired r
     WHERE NOT EXISTS (SELECT 1 FROM slug_aliases a
                        WHERE a.namespace = 'topic' AND a.alias = r.old_slug);
    IF unresolved IS NOT NULL THEN
        RAISE EXCEPTION 'aborting: retired slug(s) still have no alias: %', unresolved;
    END IF;
END $$;

COMMIT;

-- Verification — every row should name a live topic:
--   SELECT a.alias, t.slug FROM slug_aliases a
--     JOIN topics t ON t.id = a.topic_id
--    WHERE a.namespace = 'topic' ORDER BY a.alias;
