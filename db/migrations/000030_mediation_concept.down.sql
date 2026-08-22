UPDATE statements SET concept_id = NULL
    WHERE concept_id = (SELECT id FROM concepts WHERE slug = 'mediation');
DELETE FROM concepts WHERE slug = 'mediation';
