UPDATE statements SET concept_id = NULL
 WHERE concept_id IN (SELECT id FROM concepts WHERE slug = 'utility-shutoff-protection');
DELETE FROM concepts WHERE slug = 'utility-shutoff-protection';
