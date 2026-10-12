DELETE FROM advice_citations
WHERE advice_id = (SELECT id FROM advice WHERE slug = 'leave-risk-owe') AND position = 12;
UPDATE advice
SET body_md = 'Even with a good reason to leave, your landlord can sue you for the rent. If a court disagrees with you, you can still owe it.'
WHERE slug = 'leave-risk-owe';
