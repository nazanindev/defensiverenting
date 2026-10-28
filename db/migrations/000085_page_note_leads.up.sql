-- Bold leads for the page notes (ADR-016 A11), as Nazanin approved them
-- 2026-10-28.
UPDATE advice SET lead = 'Get legal help early.' WHERE slug = 'breaking-lease-disclaimer';
UPDATE advice SET lead = 'You can still owe rent.' WHERE slug IN ('leave-risk-owe', 'leave-owe-short');
UPDATE advice SET lead = 'A court decides.' WHERE slug = 'leave-court-may-disagree';
UPDATE advice SET lead = 'You can face eviction.' WHERE slug = 'withhold-risk-evict';
UPDATE advice SET lead = 'Money is not automatic.' WHERE slug = 'award-win-and-pays';
UPDATE advice SET lead = 'Think before you call an inspector.' WHERE slug = 'inspector-may-condemn';
