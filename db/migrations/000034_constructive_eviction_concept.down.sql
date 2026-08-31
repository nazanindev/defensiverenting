UPDATE statements SET concept_id = NULL
 WHERE concept_id IN (SELECT id FROM concepts WHERE slug = 'constructive-eviction');
DELETE FROM concepts WHERE slug = 'constructive-eviction';
