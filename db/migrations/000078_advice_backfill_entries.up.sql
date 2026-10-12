-- Advice registry entries for the backfill of the older topics (ADR-016 A4
-- step 1; the list approved by Nazanin, 2026-10-11). The old live pages
-- repeat the same warning and habit sentences inside statements: the
-- win-and-pays line 274 times on 6 topics, the withholding warning 89 times,
-- the legal-aid condemnation caution 21 times, "Save texts and emails" 16
-- times, and about 300 editorial-only statements that say to ask in writing,
-- keep a copy, take photos, get a receipt, or get a plan in writing. Each
-- becomes one registry entry, said once per page.
--
-- Two new warning kinds: "award" (money a court awards comes only if you win
-- and the landlord pays) and "condemn" (an inspector can condemn the home).
-- The lint accepts a statement without its own line when the page carries
-- the matching note (voice.LintOn), the same as "owe" and "evict".
--
-- The three page notes warn, so they need a government or nonprofit quote
-- before any page that references them can publish (A1). They are seeded
-- unbacked; the quotes come by later migration as each state's source is
-- found (option B, migration 000070). The four tips are plain habits, site
-- voice (000068).

ALTER TABLE advice DROP CONSTRAINT IF EXISTS advice_warns_check;
ALTER TABLE advice ADD CONSTRAINT advice_warns_check CHECK (warns IN ('owe', 'evict', 'award', 'condemn'));

INSERT INTO advice (slug, kind, warns, site_voice, body_md) VALUES
    ('award-win-and-pays', 'page_note', 'award', false,
     'Where this page says a court can award you money, you get it only if you win your case and your landlord pays.'),
    ('withhold-risk-evict', 'page_note', 'evict', false,
     'If you hold back rent, pay less, or take repair costs out of your rent, a court can later disagree. Then you can owe the rent and face eviction. Get legal help first.'),
    ('inspector-may-condemn', 'page_note', 'condemn', false,
     'One caution from legal aid: if the home is in very bad shape, an inspector can condemn it. Then everyone must move out, you too.'),
    ('photos-of-the-problem', 'tip', NULL, true,
     'Take photos of each problem, with the date. Take more if it gets worse.'),
    ('private-inspector', 'tip', NULL, true,
     'If you can pay for it, a private home inspector can check the problems. Their written report and photos help show the conditions.'),
    ('ask-for-a-receipt', 'tip', NULL, true,
     'Ask for a receipt every time you pay rent, even when you pay only part of it, and keep it.'),
    ('plan-in-writing', 'tip', NULL, true,
     'Get any payment plan or rent change in writing, with the amounts and the dates.')
ON CONFLICT (slug) DO NOTHING;
