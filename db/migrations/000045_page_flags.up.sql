-- A page flag is a doubt about the page as a whole (ADR-025, amended
-- 2026-10-03): a gap, a duplicate pair, an intro that misleads, a page too
-- thin to publish. Until now these were filed as a reviewer note on the
-- first statement the finding named, so the hold read as a problem with
-- statement 1 and could only be cleared with a no-change proposal on that
-- key. An open page flag holds the page from publishing, like a statement's
-- open note (ADR-013 gate).
CREATE TABLE page_flags (
    id           BIGSERIAL PRIMARY KEY,
    playbook_id  BIGINT      NOT NULL REFERENCES playbooks(id) ON DELETE CASCADE,
    -- What kind of page problem: duplicate, off-topic, order, contradiction,
    -- gap, intro, thin, or drafter for the drafting agent's own page doubt.
    kind         TEXT        NOT NULL,
    note         TEXT        NOT NULL CHECK (btrim(note) <> ''),
    -- The statements the flag is about, when it names any. A reference for
    -- the reader, never what holds the page.
    keys         UUID[]      NOT NULL DEFAULT '{}',
    filed_by     TEXT        NOT NULL,
    status       TEXT        NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'closed')),
    closed_by    TEXT        NOT NULL DEFAULT '',
    closed_at    TIMESTAMPTZ,
    close_note   TEXT        NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX page_flags_open_idx ON page_flags (playbook_id) WHERE status = 'open';

-- Move the open page-review notes off their first statement.
WITH moved AS (
    INSERT INTO page_flags (playbook_id, kind, note, keys, filed_by, created_at)
    SELECT sp.playbook_id,
           coalesce(substring(sp.evidence->>'note' FROM '^Page review \(([a-z-]+)\)'), 'page'),
           regexp_replace(sp.evidence->>'note', '^Page review \([^)]*\): ', ''),
           ARRAY[sp.statement_key],
           sp.proposed_by,
           sp.created_at
    FROM statement_proposals sp
    WHERE sp.reason = 'agent-pass:flag'
      AND sp.status IN ('pending', 'snoozed')
      AND sp.evidence->>'note' LIKE 'Page review (%'
    RETURNING id
)
UPDATE statement_proposals SET status = 'superseded', decided_by = 'migration', decided_at = NOW(),
       decision_note = 'moved to a page flag (migration 000045)'
WHERE reason = 'agent-pass:flag'
  AND status IN ('pending', 'snoozed')
  AND evidence->>'note' LIKE 'Page review (%';
