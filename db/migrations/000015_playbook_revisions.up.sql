-- Let a proposed revision sit alongside the page it revises.
--
-- Until now one row could exist per (jurisdiction, topic, language), so a
-- research pass over an already-published page had nowhere to put its result.
-- save_draft_playbook refused outright — correctly, since silently rewriting
-- live legal content is the thing that must never happen — but that left no way
-- to re-draft a page at all, short of unpublishing it and 404ing a URL that has
-- search traffic.
--
-- Splitting the constraint by status lets the live page stay live while a
-- revision is drafted and reviewed beside it. Nothing reaches the public until
-- a human publishes, which is unchanged.
ALTER TABLE playbooks DROP CONSTRAINT playbooks_jurisdiction_id_topic_id_language_key;

CREATE UNIQUE INDEX playbooks_one_published_per_slot
    ON playbooks (jurisdiction_id, topic_id, language) WHERE status = 'published';

CREATE UNIQUE INDEX playbooks_one_draft_per_slot
    ON playbooks (jurisdiction_id, topic_id, language) WHERE status = 'draft';

-- Publishing a revision retires the page it replaces rather than deleting it.
-- Superseded rows are deliberately not covered by a unique index, so they
-- accumulate: this is the only record of what a page used to say. It is not
-- full version history, but it means replacing a page is no longer destructive,
-- and a reviewer can see what changed after the fact.
ALTER TABLE playbooks DROP CONSTRAINT playbooks_status_check;
ALTER TABLE playbooks ADD CONSTRAINT playbooks_status_check
    CHECK (status IN ('draft', 'published', 'superseded'));

-- Every read path filters status = 'published', so superseded rows leave the
-- public site the moment they are retired. This index keeps the authoring
-- dashboard's history lookups off a sequential scan as they build up.
CREATE INDEX playbooks_superseded_idx
    ON playbooks (jurisdiction_id, topic_id, language, updated_at DESC)
    WHERE status = 'superseded';
