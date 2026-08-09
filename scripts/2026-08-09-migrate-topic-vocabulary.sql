-- Retire the legacy topic vocabulary (2026-08-09). ADR-005 D7 step 4.
--
-- Four legacy topics are folded into their canonical counterparts, and the old
-- slugs become aliases so every indexed URL keeps resolving:
--
--   security-deposit-not-returned  ->  security-deposits
--   landlord-entry-without-notice  ->  landlord-entry
--   uninhabitable-conditions       ->  repairs-and-habitability
--   notice-to-quit                 ->  eviction-defense
--
-- These are merges, not renames: the canonical topics already exist, so a
-- rename would collide on topics.slug. Playbooks are repointed instead, and the
-- legacy row is deleted once nothing references it.
--
-- cant-pay-rent, heat-not-working, rent-increase and discrimination are
-- unchanged. Keeping heat-not-working as a non-core topic rather than merging
-- it into repairs-and-habitability saves three published pages from redirects,
-- and it is the page a renter actually searches for in January.
--
-- The state segment needs no data: browse handlers derive it from the
-- jurisdiction row, so /j/boston/x already 301s to /j/massachusetts/boston/x.
--
-- REQUIRES migration 000013 (slug_aliases). Deploy the application first.
--
-- Idempotent. To dry run, replace COMMIT with ROLLBACK; the assertions still
-- run, so a rehearsal proves the script reaches a valid state.
--
-- Drop the backups once the site checks out:
--   DROP TABLE playbooks_backup_20260809, topics_backup_20260809;

BEGIN;

CREATE TABLE IF NOT EXISTS playbooks_backup_20260809 AS SELECT * FROM playbooks;
CREATE TABLE IF NOT EXISTS topics_backup_20260809   AS SELECT * FROM topics;

CREATE TEMP TABLE tmap(old_slug TEXT, new_slug TEXT) ON COMMIT DROP;
INSERT INTO tmap VALUES
    ('security-deposit-not-returned', 'security-deposits'),
    ('landlord-entry-without-notice', 'landlord-entry'),
    ('uninhabitable-conditions',      'repairs-and-habitability'),
    ('notice-to-quit',                'eviction-defense');

-- Refuse to run without somewhere to redirect to.
DO $$
DECLARE missing TEXT;
BEGIN
    IF to_regclass('public.slug_aliases') IS NULL THEN
        RAISE EXCEPTION 'aborting: slug_aliases does not exist. Deploy migration 000013 first.';
    END IF;
    SELECT string_agg(m.new_slug, ', ') INTO missing
      FROM tmap m WHERE NOT EXISTS (SELECT 1 FROM topics t WHERE t.slug = m.new_slug);
    IF missing IS NOT NULL THEN
        RAISE EXCEPTION 'aborting: canonical topic(s) missing: %', missing;
    END IF;
END $$;

-- 1. Eviction merge. Boston and Seattle each have an eviction-defense draft
-- sitting beside a published notice-to-quit page, and only one can exist per
-- (jurisdiction, topic). Reviewed 2026-08-09: the drafts are better, so they
-- replace the published pages rather than being discarded.
CREATE TEMP TABLE replaced_by_draft ON COMMIT DROP AS
SELECT DISTINCT p.jurisdiction_id
  FROM playbooks p
  JOIN topics t ON t.id = p.topic_id
 WHERE t.slug = 'notice-to-quit'
   AND EXISTS (
        SELECT 1 FROM playbooks d JOIN topics dt ON dt.id = d.topic_id
         WHERE d.jurisdiction_id = p.jurisdiction_id
           AND dt.slug = 'eviction-defense' AND d.status = 'draft');

DELETE FROM playbook_statements ps
 USING playbooks p, topics t
 WHERE ps.playbook_id = p.id AND p.topic_id = t.id AND t.slug = 'notice-to-quit'
   AND p.jurisdiction_id IN (SELECT jurisdiction_id FROM replaced_by_draft);

DELETE FROM playbooks p
 USING topics t
 WHERE p.topic_id = t.id AND t.slug = 'notice-to-quit'
   AND p.jurisdiction_id IN (SELECT jurisdiction_id FROM replaced_by_draft);

-- Publishing stamps last_reviewed_at, matching what the authoring tool does:
-- these were reviewed before this ran.
UPDATE playbooks p
   SET status = 'published',
       published_at = COALESCE(p.published_at, NOW()),
       last_reviewed_at = NOW(),
       updated_at = NOW()
  FROM topics t
 WHERE t.id = p.topic_id AND t.slug = 'eviction-defense' AND p.status = 'draft'
   AND p.jurisdiction_id IN (SELECT jurisdiction_id FROM replaced_by_draft);

-- 2. Repoint every remaining playbook from its legacy topic to the canonical
-- one. The NOT EXISTS guard skips a row whose city already holds the canonical
-- topic, which would violate the (jurisdiction, topic, language) unique key.
-- playbooks.slug is written but never read (routing keys on topics.slug); it is
-- kept in step to avoid leaving a column that visibly disagrees with reality.
UPDATE playbooks p
   SET topic_id = n.id, slug = n.slug, updated_at = NOW()
  FROM tmap m
  JOIN topics o ON o.slug = m.old_slug
  JOIN topics n ON n.slug = m.new_slug
 WHERE p.topic_id = o.id
   AND NOT EXISTS (
        SELECT 1 FROM playbooks x
         WHERE x.jurisdiction_id = p.jurisdiction_id
           AND x.topic_id = n.id AND x.language = p.language);

-- 3. Retire the legacy rows. The alias has to come after the delete: an alias
-- that shadows a live slug is unreachable, since lookups try live slugs first.
DELETE FROM topics t
 USING tmap m
 WHERE t.slug = m.old_slug
   AND NOT EXISTS (SELECT 1 FROM playbooks p WHERE p.topic_id = t.id);

INSERT INTO slug_aliases (alias, namespace, topic_id)
SELECT m.old_slug, 'topic', n.id
  FROM tmap m JOIN topics n ON n.slug = m.new_slug
 WHERE NOT EXISTS (SELECT 1 FROM topics live WHERE live.slug = m.old_slug)
ON CONFLICT (namespace, alias) DO UPDATE SET topic_id = EXCLUDED.topic_id;

-- Refuse to commit a half-finished migration.
DO $$
DECLARE
    leftover  TEXT;
    unaliased TEXT;
    orphaned  int;
BEGIN
    SELECT string_agg(t.slug, ', ') INTO leftover
      FROM topics t JOIN tmap m ON m.old_slug = t.slug;
    IF leftover IS NOT NULL THEN
        RAISE EXCEPTION 'aborting: legacy topic(s) still present, so playbooks still reference them: %', leftover;
    END IF;

    SELECT string_agg(m.old_slug, ', ') INTO unaliased
      FROM tmap m
     WHERE NOT EXISTS (SELECT 1 FROM slug_aliases a
                        WHERE a.namespace = 'topic' AND a.alias = m.old_slug);
    IF unaliased IS NOT NULL THEN
        RAISE EXCEPTION 'aborting: retired slug(s) with no alias, their URLs would 404: %', unaliased;
    END IF;

    SELECT count(*) INTO orphaned FROM playbooks p
     WHERE NOT EXISTS (SELECT 1 FROM topics t WHERE t.id = p.topic_id);
    IF orphaned > 0 THEN
        RAISE EXCEPTION 'aborting: % playbook(s) point at a topic that no longer exists', orphaned;
    END IF;
END $$;

COMMIT;

-- Verification:
--   SELECT j.slug, t.slug, p.status FROM playbooks p
--     JOIN jurisdictions j ON j.id = p.jurisdiction_id
--     JOIN topics t ON t.id = p.topic_id
--    WHERE p.status = 'published' ORDER BY j.slug, t.slug;
--   SELECT alias, namespace FROM slug_aliases ORDER BY alias;
