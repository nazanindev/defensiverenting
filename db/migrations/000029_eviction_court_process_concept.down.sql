UPDATE statements SET concept_id = NULL
    WHERE concept_id = (SELECT id FROM concepts WHERE slug = 'eviction-court-process');
DELETE FROM concepts WHERE slug = 'eviction-court-process';
