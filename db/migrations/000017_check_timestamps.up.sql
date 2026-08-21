-- When was this citation's quote last confirmed to still appear at its live
-- source? Stamped by every path that actually looks: the drafting guardrail at
-- save, the authoring form's quote verifier, a reviewer's manual attestation,
-- and each check-sources run. NULL means no confirmation has been recorded —
-- which is the honest state for every existing row: sources.retrieved_at is
-- bumped by UpsertSource on every save with no fetch involved, so no existing
-- stamp records a check. Deliberately not backfilled; the first check-sources
-- run after this migration stamps every checkable citation with a real value.
ALTER TABLE citations ADD COLUMN checked_at TIMESTAMPTZ;

-- When the checker last fetched this source and examined its cited quotes.
-- Distinct from retrieved_at (see above: bumped without fetching) and from
-- flagged_at (set only when a quote went missing). A source whose quotes all
-- carry no verbatim text is never examined, and its NULL here says so.
ALTER TABLE sources ADD COLUMN last_checked_at TIMESTAMPTZ;
