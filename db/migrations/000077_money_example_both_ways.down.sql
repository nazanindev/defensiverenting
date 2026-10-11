UPDATE advice
SET body_md = 'This is only an example to show how the rule works. If your landlord sues, a court decides the real amount.',
    site_voice = false
WHERE slug = 'money-example';
INSERT INTO advice_citations (advice_id, source_id, quote, position)
SELECT a.id, s.id,
       'Most creditors can only garnish wages or benefits after a court issues a judgment saying that you owe the debt and that the creditor can garnish your wages or benefits to satisfy the debt.', 0
FROM advice a, sources s
WHERE a.slug = 'money-example'
  AND s.url = 'https://www.consumerfinance.gov/ask-cfpb/can-a-debt-collector-take-or-garnish-my-wages-or-benefits-en-1439/';
