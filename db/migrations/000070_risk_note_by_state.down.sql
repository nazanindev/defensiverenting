DELETE FROM advice_citations WHERE advice_id IN (SELECT id FROM advice WHERE slug IN ('leave-owe-short', 'leave-court-may-disagree'));
DELETE FROM advice_citations WHERE advice_id = (SELECT id FROM advice WHERE slug = 'leave-risk-owe') AND position >= 10;
DELETE FROM advice WHERE slug IN ('leave-owe-short', 'leave-court-may-disagree');
