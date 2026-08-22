UPDATE statements SET concept_id = NULL
    WHERE concept_id IN (SELECT id FROM concepts WHERE slug IN ('small-claims-court', 'records-and-evidence'));
DELETE FROM concepts WHERE slug IN ('small-claims-court', 'records-and-evidence');
