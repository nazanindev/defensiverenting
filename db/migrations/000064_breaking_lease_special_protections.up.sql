-- Breaking a lease: the common case leads (what you owe if you just leave),
-- and victim and military exits move to their own section near the end.
-- Decided by Nazanin 2026-10-06: "special protections".
UPDATE topics
SET stages = ARRAY['What you may still owe', 'When you can leave early', 'How to leave', 'Special protections', 'After you move out']
WHERE slug = 'breaking-lease';
