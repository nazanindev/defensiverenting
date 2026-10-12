-- The condemnation caution is true everywhere: every state has local code
-- inspectors, and an inspector who finds a home unfit can order everyone out.
-- Option B (ADR-016 A9) only showed the note where the state had a quote, so
-- 14 states carried it at the top and the rest kept the sentence inside a
-- statement. Nazanin, 2026-10-12: "I'd much rather have it at the top as a
-- warning ... it should fall back to site voice if it has nothing. It's
-- important." So this one warning is site voice: it shows on every page that
-- references it, and a state's government or nonprofit quote still shows as
-- the chip where one exists (the renderer already does this). The
-- "a warning is never site voice" check goes; the line now lives here, in
-- migrations a person reviews, the same as the habit/claim line (000068):
-- a warning may be site voice only when the risk is the same in every state.
-- The withholding and award notes stay per state, because those vary.
ALTER TABLE advice DROP CONSTRAINT IF EXISTS advice_warning_needs_source;

UPDATE advice SET site_voice = true,
  body_md = 'You or a neighbor can ask the city to send a code inspector. Be careful: if the home is in very bad shape, the inspector can condemn it (say no one may live there). Then everyone must move out, you too, sometimes on short notice. Think about where you would go before you call.'
WHERE slug = 'inspector-may-condemn';
