DELETE FROM advice_citations WHERE advice_id IN (SELECT id FROM advice WHERE slug IN ('award-win-and-pays', 'withhold-risk-evict', 'inspector-may-condemn'));
