-- Topics become a closed registry. ADR-005 D5.
--
-- Until now the topic vocabulary lived in two hardcoded Go lists that
-- disagreed: internal/draftagent (CoreTopics, 5 entries, used by the AI batch)
-- and cmd/authoring (knownTopics, 13 entries, used by the form). Neither was
-- checked against this table, and UpsertTopic created whatever either one
-- supplied. That is how two vocabularies for the same subjects came to exist —
-- security-deposits alongside security-deposit-not-returned, and so on.
--
-- From here the canonical set is data, seeded here and edited by migration.
-- is_core marks the topics every new city is seeded with; the rest are added
-- to a city only where its law justifies one.
ALTER TABLE topics ADD COLUMN is_core BOOLEAN NOT NULL DEFAULT false;

-- Display names are renter-facing (page titles, topic hubs), so they follow the
-- editorial voice: plain words, no legal jargon. This deliberately overwrites
-- the slugToTitle output now live in production ("Cant Pay Rent" with no
-- apostrophe, "Notice To Quit"), and drops "habitability", "notice to quit",
-- and "fundamentals" as words a stressed renter should not have to parse.
--
-- The ON CONFLICT DO UPDATE is intentional: this migration is the authority on
-- names, while the application no longer touches them at all.
INSERT INTO topics (slug, name, is_core) VALUES
    -- Core: seeded for every city.
    ('cant-pay-rent',            'Can''t Pay Rent',             true),
    ('eviction-defense',         'Eviction',                    true),
    ('repairs-and-habitability', 'Repairs and Unsafe Conditions', true),
    ('security-deposits',        'Security Deposits',           true),
    ('landlord-entry',           'Landlord Entry',              true),
    ('rent-increase',            'Rent Increases',              true),
    ('resource-directory',       'Local Help',                  true),
    -- Non-core: added per city where the law or the climate justifies it.
    ('heat-not-working',         'Heat Not Working',            false),
    ('rent-stabilization',       'Rent Stabilization',          false),
    ('discrimination',           'Housing Discrimination',      false),
    ('move-in-checklist',        'Move-In Checklist',           false),
    ('move-out-checklist',       'Move-Out Checklist',          false),
    ('lease-renewal',            'Lease Renewal',               false),
    ('noise-complaints',         'Noise Complaints',            false),
    ('renting-fundamentals',     'Renting Basics',              false)
ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name, is_core = EXCLUDED.is_core;

-- The legacy vocabulary (notice-to-quit, security-deposit-not-returned,
-- landlord-entry-without-notice, uninhabitable-conditions) is deliberately NOT
-- seeded here. Those rows still hold published playbooks; they are retired by
-- the URL migration (D7 step 4), which moves their playbooks and records a
-- slug alias so the old URLs keep resolving.
