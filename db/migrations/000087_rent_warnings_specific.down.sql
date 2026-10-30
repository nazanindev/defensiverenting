DELETE FROM playbook_advice WHERE advice_id = (SELECT id FROM advice WHERE slug = 'leave-repairs-sue-rent');
DELETE FROM advice WHERE slug = 'leave-repairs-sue-rent';
UPDATE advice SET
  lead = 'You can face eviction.',
  body_md = 'If you hold back rent, pay less, or take repair costs out of your rent, a court can later disagree. Then you can owe the rent and face eviction. Get legal help first.'
WHERE slug = 'withhold-risk-evict';
