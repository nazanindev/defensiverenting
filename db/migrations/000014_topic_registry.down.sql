-- Seeded topics are left in place: dropping rows that playbooks reference would
-- cascade into published content. Only the flag is reversible.
ALTER TABLE topics DROP COLUMN IF EXISTS is_core;
