-- Track sources whose upstream content changed since the author last reviewed
-- them. The source-change checker re-fetches each cited source and, when a hash
-- of its content differs from the stored content_hash, sets flagged_at so the
-- source surfaces in the authoring dashboard. Dismissing clears the flag.
ALTER TABLE sources ADD COLUMN flagged_at TIMESTAMPTZ;
