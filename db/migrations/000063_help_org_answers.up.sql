-- ADR-029 D4 (Orgs page, 2026-10-04): each question the editor asks an org
-- is its own field.
--   1. "May we list you?"  -> help_orgs.status (unchanged; only 'ok' lists)
--   2. "Is the information correct?" -> info_correct, with the corrections
--   3. "How would you like renters to reach you?" -> help_org_channels, any
--      number of contacts of any type, some marked preferred
ALTER TABLE help_orgs
    ADD COLUMN info_correct TEXT NOT NULL DEFAULT 'not_asked'
        CHECK (info_correct IN ('not_asked', 'yes', 'needs_changes')),
    ADD COLUMN info_changes TEXT NOT NULL DEFAULT '';

CREATE TABLE help_org_channels (
    id         BIGSERIAL   PRIMARY KEY,
    host       TEXT        NOT NULL REFERENCES help_orgs(host) ON DELETE CASCADE,
    kind       TEXT        NOT NULL
               CHECK (kind IN ('phone', 'email', 'website', 'intake_form', 'text', 'address', 'walk_in', 'other')),
    value      TEXT        NOT NULL CHECK (btrim(value) <> ''),
    label      TEXT        NOT NULL DEFAULT '',
    preferred  BOOLEAN     NOT NULL DEFAULT FALSE,
    added_by   TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX help_org_channels_host_idx ON help_org_channels (host);
