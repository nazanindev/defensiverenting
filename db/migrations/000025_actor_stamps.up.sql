-- Record who last touched a page and who last confirmed each citation, now
-- that the portal has one login per person instead of one shared credential.
--
-- Deliberately coarse: one name per row, overwritten on every touch. Not an
-- audit trail — the superseded rows from migration 000015 already keep what a
-- page used to say; this adds who to ask about the current version.
--
-- Names are short handles ("Nazanin", "Cam", "drafting agent", "source
-- check"), never full names. Authoring-only in the same sense as
-- author_notes: the public render path never reads updated_by, and while
-- checked_by rides along in the shared citation scan, no public template
-- prints it.
ALTER TABLE playbooks ADD COLUMN updated_by TEXT NOT NULL DEFAULT '';

-- Pairs with checked_at (migration 000017): checked_at says when the quote
-- was last confirmed at the source, checked_by says by whom — a person who
-- saved with a live fetch or attested by hand, the drafting agent's
-- verbatim-quote guardrail, or the automated source check run. Empty where
-- checked_at is NULL: never confirmed means nobody to name.
ALTER TABLE citations ADD COLUMN checked_by TEXT NOT NULL DEFAULT '';
