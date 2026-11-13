-- Three questions the repairs pages answer and the registry did not have
-- (2026-11-13). The tagging pass found 58 statements in 30 places that
-- state this law with no question to carry.
INSERT INTO concepts (slug, name, topic_id, question, definition, position)
SELECT v.slug, v.name, t.id, v.question, v.definition, v.position
FROM (VALUES
    ('repair-deadline',   'Repair deadline',    25,
     'How fast does my landlord have to make repairs?',
     'How long your landlord has to start or finish a repair after you ask.'),
    ('repair-lawsuit',    'Suing over repairs', 95,
     'Can I sue my landlord over repairs?',
     'Going to court to make your landlord fix a problem, cut your rent, or pay you.'),
    ('repair-court-case', 'Repair court case',  97,
     'What happens in a court case about repairs?',
     'How a court case about repairs works, including paying your rent to the court.')
) AS v(slug, name, position, question, definition)
JOIN topics t ON t.slug = 'repairs-and-habitability';
