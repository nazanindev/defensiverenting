-- A proposed change to one statement, waiting for a person (ADR-014 D2).
--
-- A proposal is the whole statement as it should read afterward, filed
-- against the claim's durable key rather than a row id, so it survives the
-- saves that happen while it waits. Approving one is an ordinary save by the
-- reviewer (D3), so the publish gate judges it like any other edit.
--
-- reason is a closed vocabulary extended by migration, like topics and
-- concepts: a fixed code, or a fixed prefix with a free name after it.
CREATE TABLE statement_proposals (
    id             BIGSERIAL PRIMARY KEY,
    statement_key  UUID        NOT NULL,
    -- The page the proposer was looking at. The page the approval edits is
    -- resolved at decision time (a draft revision beside a live page wins),
    -- because the slot can change while the proposal waits.
    playbook_id    BIGINT      NOT NULL REFERENCES playbooks(id) ON DELETE CASCADE,
    reason         TEXT        NOT NULL CHECK (
                       reason = 'source-drift'
                       OR reason LIKE 'agent-pass:%'
                       OR reason LIKE 'source-quality:%'),
    -- The replacement statement (body, concept, citations), or NULL for a
    -- work item the proposer could not resolve itself (D4).
    proposed       JSONB,
    evidence       JSONB       NOT NULL DEFAULT '{}'::jsonb,
    proposed_by    TEXT        NOT NULL,
    status         TEXT        NOT NULL DEFAULT 'pending' CHECK (status IN
                       ('pending', 'approved', 'rejected', 'snoozed', 'superseded')),
    decided_by     TEXT        NOT NULL DEFAULT '',
    decided_at     TIMESTAMPTZ,
    decision_note  TEXT        NOT NULL DEFAULT '',
    snoozed_until  TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX statement_proposals_status_idx ON statement_proposals (status, playbook_id);
CREATE INDEX statement_proposals_key_idx    ON statement_proposals (statement_key);
