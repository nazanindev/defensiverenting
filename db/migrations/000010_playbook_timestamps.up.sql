-- Authoring metadata: when a playbook was first created and last edited, shown
-- in the authoring dashboard. (last_reviewed_at already tracks review sign-off;
-- these track creation and content edits.)
ALTER TABLE playbooks
    ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
