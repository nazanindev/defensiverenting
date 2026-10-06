-- Special protections sits at the very bottom of breaking-a-lease pages,
-- after "After you move out" (Nazanin 2026-10-06).
UPDATE topics
SET stages = ARRAY['What you may still owe', 'When you can leave early', 'How to leave', 'After you move out', 'Special protections']
WHERE slug = 'breaking-lease';
