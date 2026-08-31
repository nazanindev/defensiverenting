-- Remove the topic only while nothing references it: dropping a row that
-- playbooks point at would cascade into content (same caution as 000014's
-- down migration).
DELETE FROM topics t
 WHERE t.slug = 'constructive-eviction'
   AND NOT EXISTS (SELECT 1 FROM playbooks p WHERE p.topic_id = t.id);
