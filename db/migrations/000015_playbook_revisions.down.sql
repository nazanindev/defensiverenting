-- Superseded rows must go before the single-row-per-slot constraint can return,
-- and they are the only copy of what those pages used to say.
DELETE FROM playbooks WHERE status = 'superseded';

DROP INDEX IF EXISTS playbooks_superseded_idx;
DROP INDEX IF EXISTS playbooks_one_draft_per_slot;
DROP INDEX IF EXISTS playbooks_one_published_per_slot;

ALTER TABLE playbooks DROP CONSTRAINT playbooks_status_check;
ALTER TABLE playbooks ADD CONSTRAINT playbooks_status_check
    CHECK (status IN ('draft', 'published'));

ALTER TABLE playbooks ADD CONSTRAINT playbooks_jurisdiction_id_topic_id_language_key
    UNIQUE (jurisdiction_id, topic_id, language);
