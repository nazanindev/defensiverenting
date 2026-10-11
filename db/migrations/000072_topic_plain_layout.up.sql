-- The plain layout (ADR-016 as amended; the Arizona pattern, Nazanin
-- 2026-10-11) is switched on per topic, on purpose, one topic at a time
-- after its own pilot. On a plain-layout topic the lint holds statements to
-- 75 words, no code section numbers and deadlines that say from when, and
-- the publish gate holds a page to 12 statements and 650 words (statements
-- that cite only national guidance, like the debt-collector and pay
-- statements every state page shares, are not counted). Rules pages never
-- use it.
ALTER TABLE topics ADD COLUMN plain_layout BOOLEAN NOT NULL DEFAULT false;
UPDATE topics SET plain_layout = true WHERE slug = 'breaking-lease';
