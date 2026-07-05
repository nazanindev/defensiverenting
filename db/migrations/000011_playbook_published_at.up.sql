-- When a playbook first went live. Nullable — drafts have no publish date.
-- Backfill existing published rows from created_at (best available signal).
ALTER TABLE playbooks ADD COLUMN published_at TIMESTAMPTZ;
UPDATE playbooks SET published_at = created_at WHERE status = 'published';
