-- Free-text working notes for the authoring portal: reminders, review context,
-- things to re-check next visit. Authoring-only — the public queries never
-- select this column, so nothing a renter sees can include it. Lives on the
-- playbook row (not statements) so notes survive re-saves: statements are
-- replaced wholesale on every save, the playbook row persists.
ALTER TABLE playbooks ADD COLUMN author_notes TEXT NOT NULL DEFAULT '';
