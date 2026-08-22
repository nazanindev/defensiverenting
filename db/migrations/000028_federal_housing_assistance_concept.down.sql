UPDATE statements SET concept_id = NULL
    WHERE concept_id = (SELECT id FROM concepts WHERE slug = 'federal-housing-assistance');
DELETE FROM concepts WHERE slug = 'federal-housing-assistance';
