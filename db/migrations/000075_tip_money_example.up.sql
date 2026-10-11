-- Any dollar amount on a page is an example of how a rule works, not a
-- bill (Nazanin 2026-10-11). One quiet line under each statement with a
-- worked money example says so. Backed by the national CFPB quote that a
-- court judgment decides what is owed.
INSERT INTO advice (slug, kind, body_md) VALUES
    ('money-example', 'tip',
     'This is only an example to show how the rule works. If your landlord sues, a court decides the real amount.')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id,
       'Most creditors can only garnish wages or benefits after a court issues a judgment saying that you owe the debt and that the creditor can garnish your wages or benefits to satisfy the debt.', 0
FROM advice a, sources s
WHERE a.slug = 'money-example'
  AND s.url = 'https://www.consumerfinance.gov/ask-cfpb/can-a-debt-collector-take-or-garnish-my-wages-or-benefits-en-1439/'
  AND NOT EXISTS (SELECT 1 FROM advice_citations c WHERE c.advice_id = a.id);
