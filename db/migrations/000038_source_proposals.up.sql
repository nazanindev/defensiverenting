-- A proposal to delete a source no page cites (ADR-014, amendment D7).
--
-- Sources are upserted the moment a URL is typed and never removed, so rows
-- drift into disuse silently. Each one still costs a fetch on every check
-- run and clutters the import picker. The check run files one of these per
-- unused source; a person decides on the review queue, beside the statement
-- proposals. Approving deletes the source row outright.
--
-- The URL and publisher are copied in so the record of the decision
-- survives the deletion it approves; source_id is nulled by the delete.
CREATE TABLE source_proposals (
    id             BIGSERIAL PRIMARY KEY,
    source_id      BIGINT      REFERENCES sources(id) ON DELETE SET NULL,
    url            TEXT        NOT NULL,
    publisher      TEXT        NOT NULL DEFAULT '',
    kind           TEXT        NOT NULL DEFAULT '',
    reason         TEXT        NOT NULL CHECK (reason = 'unused-source'),
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

-- One open proposal per source: repeated check runs must not refile what a
-- person can already see.
CREATE UNIQUE INDEX source_proposals_open_idx ON source_proposals (source_id)
    WHERE status IN ('pending', 'snoozed');
CREATE INDEX source_proposals_status_idx ON source_proposals (status);
