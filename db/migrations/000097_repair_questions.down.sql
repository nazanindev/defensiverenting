UPDATE statements SET concept_id = NULL
 WHERE concept_id IN (SELECT id FROM concepts WHERE slug IN ('repair-deadline', 'repair-lawsuit', 'repair-court-case'));
DELETE FROM concepts WHERE slug IN ('repair-deadline', 'repair-lawsuit', 'repair-court-case');
