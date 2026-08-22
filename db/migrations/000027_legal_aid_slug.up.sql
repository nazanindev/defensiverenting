-- The display name became "Legal aid" (migration 000025) but the slug stayed
-- free-legal-help, so the term listed as Legal aid lived at
-- /c/free-legal-help and /c/legal-aid was a 404 — a name/slug mismatch the
-- term-of-art renames were supposed to end, not create. Slugs are public
-- anchors and normally stay put; this page is a day old, which is the last
-- moment a rename costs nothing. Statement tags follow automatically (the FK
-- is by id).
UPDATE concepts SET slug = 'legal-aid' WHERE slug = 'free-legal-help';
