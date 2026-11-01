-- Site copy pages: the about, editorial, privacy and support pages, edited
-- by the editor in the authoring app instead of by a code change.
--
-- One row per page slug. No row means the page shows the built-in text that
-- ships with the binary (internal/sitecopy/defaults), so this table is
-- always an override and the site renders with it empty. Every save also
-- writes a history row, so a bad edit can be read back and undone by hand.
CREATE TABLE site_pages (
    slug        TEXT        PRIMARY KEY,
    title       TEXT        NOT NULL,
    description TEXT        NOT NULL,
    body_md     TEXT        NOT NULL,
    updated_by  TEXT        NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE site_page_history (
    id          BIGSERIAL   PRIMARY KEY,
    slug        TEXT        NOT NULL,
    title       TEXT        NOT NULL,
    description TEXT        NOT NULL,
    body_md     TEXT        NOT NULL,
    updated_by  TEXT        NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX site_page_history_slug_idx ON site_page_history (slug, updated_at DESC);
