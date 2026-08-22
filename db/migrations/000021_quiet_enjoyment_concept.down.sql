UPDATE statements SET concept_id = NULL
    WHERE concept_id = (SELECT id FROM concepts WHERE slug = 'quiet-enjoyment');
DELETE FROM concepts WHERE slug = 'quiet-enjoyment';
UPDATE concepts SET name = 'Right to a safe, livable home' WHERE slug = 'habitability-standard';
