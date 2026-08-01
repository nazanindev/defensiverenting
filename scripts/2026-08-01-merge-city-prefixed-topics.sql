-- Merge city-prefixed topics into shared canonical topics (2026-08-01).
-- Context: early drafting runs created per-city topic slugs
-- (pittsburgh-cant-pay-rent, seattle-*) instead of the shared slugs Boston
-- uses. This repoints their playbooks at the canonical topics so topic hubs
-- and cross-city links connect, renames pittsburgh-discrimination (no
-- canonical counterpart) to discrimination, and deletes the orphaned topics.
-- Backup tables are created first; drop them once the site checks out:
--   DROP TABLE topics_backup_20260801, playbooks_backup_20260801,
--              statement_topics_backup_20260801;
-- ID mapping verified against prod 2026-08-01:
--   45,13 -> 1 cant-pay-rent          50,14 -> 2 heat-not-working
--   49,15 -> 3 landlord-entry-without-notice   16 -> 4 notice-to-quit
--   47,17 -> 5 security-deposit-not-returned
--   48,18 -> 6 uninhabitable-conditions        57 -> 69 rent-increase
--   59 renamed in place to discrimination

BEGIN;

CREATE TABLE topics_backup_20260801 AS SELECT * FROM topics;
CREATE TABLE playbooks_backup_20260801 AS SELECT * FROM playbooks;
CREATE TABLE statement_topics_backup_20260801 AS SELECT * FROM statement_topics;

CREATE TEMP TABLE tmap(old_id bigint, new_id bigint);
INSERT INTO tmap VALUES
  (45,1),(13,1),(50,2),(14,2),(49,3),(15,3),(16,4),
  (47,5),(17,5),(48,6),(18,6),(57,69);

UPDATE playbooks p SET topic_id = m.new_id FROM tmap m WHERE p.topic_id = m.old_id;

INSERT INTO statement_topics (statement_id, topic_id)
  SELECT st.statement_id, m.new_id FROM statement_topics st
  JOIN tmap m ON st.topic_id = m.old_id
  ON CONFLICT DO NOTHING;
DELETE FROM statement_topics st USING tmap m WHERE st.topic_id = m.old_id;

UPDATE topics SET slug = 'discrimination', name = 'Discrimination' WHERE id = 59;

DELETE FROM topics t USING tmap m WHERE t.id = m.old_id;

COMMIT;

-- Verify: every published topic should now list its cities together.
SELECT t.slug, count(p.id) AS published,
       string_agg(j.slug, ',' ORDER BY j.slug) AS cities
FROM topics t
JOIN playbooks p ON p.topic_id = t.id AND p.status = 'published'
JOIN jurisdictions j ON j.id = p.jurisdiction_id
GROUP BY t.slug ORDER BY t.slug;
