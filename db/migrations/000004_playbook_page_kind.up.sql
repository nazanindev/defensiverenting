ALTER TABLE playbooks
    ADD COLUMN page_kind TEXT NOT NULL DEFAULT 'playbook'
        CHECK (page_kind IN ('playbook', 'directory', 'faq', 'checklist'));
