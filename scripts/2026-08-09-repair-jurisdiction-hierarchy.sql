-- Repair the jurisdiction hierarchy (2026-08-09). ADR-005 D7 step 2.
--
-- Context: cities and states were created ad hoc by whichever code path ran
-- first, and UpsertJurisdiction never required a parent. The result is a
-- half-null tree. ADR-005 D2 puts the parent state in every city URL, so a
-- city without a parent has no address at all — this has to be correct before
-- the URL migration can run.
--
-- Verified against prod 2026-08-09, before this script:
--   boston        -> (no parent)   should be massachusetts
--   seattle       -> (no parent)   should be washington, which does not exist
--   california    -> (no parent)   should be united-states
--   illinois      -> (no parent)   should be united-states
--   pennsylvania  -> (no parent)   should be united-states
--   texas         -> (no parent)   should be united-states
--   massachusetts -> united-states  already correct
--   new-york      -> united-states  already correct (created by cmd/promote)
--   austin, chicago, los-angeles, new-york-city, philadelphia, pittsburgh
--                                   already correctly parented
--
-- Seattle has been published since 2026-07-05 with no state row at all, so
-- nothing has ever been able to say which law governs it.
--
-- Idempotent: every statement is guarded, so re-running changes nothing.
-- To dry run, replace COMMIT with ROLLBACK — the assertion below still runs,
-- so a rehearsal proves the script reaches a valid state before you commit it.
--
-- Drop the backup once the site checks out:
--   DROP TABLE jurisdictions_backup_20260809;

BEGIN;

CREATE TABLE IF NOT EXISTS jurisdictions_backup_20260809 AS SELECT * FROM jurisdictions;

-- Washington is missing entirely despite Seattle being published.
INSERT INTO jurisdictions (kind, name, slug, parent_id)
SELECT 'state', 'Washington', 'washington', (SELECT id FROM jurisdictions WHERE slug = 'united-states')
WHERE NOT EXISTS (SELECT 1 FROM jurisdictions WHERE slug = 'washington');

-- Every state belongs to the country.
UPDATE jurisdictions
   SET parent_id = (SELECT id FROM jurisdictions WHERE slug = 'united-states')
 WHERE kind = 'state' AND parent_id IS NULL;

-- The two cities published before parents were being set.
UPDATE jurisdictions
   SET parent_id = (SELECT id FROM jurisdictions WHERE slug = 'massachusetts')
 WHERE slug = 'boston' AND parent_id IS NULL;

UPDATE jurisdictions
   SET parent_id = (SELECT id FROM jurisdictions WHERE slug = 'washington')
 WHERE slug = 'seattle' AND parent_id IS NULL;

-- Refuse to commit a tree that is still broken. Only the country may be
-- parentless; a city must sit under a state, and a state under the country.
DO $$
DECLARE
    orphans   int;
    misplaced int;
BEGIN
    SELECT count(*) INTO orphans
      FROM jurisdictions WHERE parent_id IS NULL AND kind <> 'country';
    IF orphans > 0 THEN
        RAISE EXCEPTION 'aborting: % jurisdiction(s) still have no parent', orphans;
    END IF;

    SELECT count(*) INTO misplaced
      FROM jurisdictions j JOIN jurisdictions p ON p.id = j.parent_id
     WHERE (j.kind = 'city'  AND p.kind <> 'state')
        OR (j.kind = 'state' AND p.kind <> 'country');
    IF misplaced > 0 THEN
        RAISE EXCEPTION 'aborting: % jurisdiction(s) have a parent of the wrong kind', misplaced;
    END IF;
END $$;

COMMIT;

-- Verification — every row should show a parent, and no city should be
-- missing its state:
--   SELECT j.kind, j.slug, COALESCE(p.slug, '(root)')
--     FROM jurisdictions j LEFT JOIN jurisdictions p ON p.id = j.parent_id
--    ORDER BY j.kind, j.slug;
