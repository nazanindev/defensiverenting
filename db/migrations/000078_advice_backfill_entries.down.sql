DELETE FROM playbook_advice WHERE advice_id IN (SELECT id FROM advice WHERE slug IN
    ('award-win-and-pays', 'withhold-risk-evict', 'inspector-may-condemn',
     'photos-of-the-problem', 'private-inspector', 'ask-for-a-receipt', 'plan-in-writing'));
DELETE FROM advice_citations WHERE advice_id IN (SELECT id FROM advice WHERE slug IN
    ('award-win-and-pays', 'withhold-risk-evict', 'inspector-may-condemn'));
DELETE FROM advice WHERE slug IN
    ('award-win-and-pays', 'withhold-risk-evict', 'inspector-may-condemn',
     'photos-of-the-problem', 'private-inspector', 'ask-for-a-receipt', 'plan-in-writing');
ALTER TABLE advice DROP CONSTRAINT IF EXISTS advice_warns_check;
ALTER TABLE advice ADD CONSTRAINT advice_warns_check CHECK (warns IN ('owe', 'evict'));
