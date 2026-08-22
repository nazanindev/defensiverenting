UPDATE statements SET concept_id = NULL
    WHERE concept_id = (SELECT id FROM concepts WHERE slug = 'late-fees');
DELETE FROM concepts WHERE slug = 'late-fees';
