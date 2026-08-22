UPDATE concepts SET name = 'Free legal help' WHERE slug = 'free-legal-help';
UPDATE statements SET concept_id = NULL
    WHERE concept_id = (SELECT id FROM concepts WHERE slug = 'complaint-line');
DELETE FROM concepts WHERE slug = 'complaint-line';
