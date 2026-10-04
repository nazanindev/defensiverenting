-- ADR-029: small local orgs are contacted before we send them renters.
--
-- An org is a website host. Every `nonprofit` source belongs to the org of
-- its host. A contact-first org shows on the live site only once it has said
-- yes; a host with no row counts as contact first and not contacted, so a new
-- org an agent cites stays off the site until someone looks at it.

-- The host of a source URL, lower case, without "www.".
CREATE FUNCTION source_host(url TEXT) RETURNS TEXT
LANGUAGE sql IMMUTABLE AS $$
    SELECT lower(substring(url FROM '^[A-Za-z][A-Za-z0-9+.-]*://(?:www\.)?([^/:?#]+)'))
$$;

CREATE TABLE help_orgs (
    host       TEXT        PRIMARY KEY,
    name       TEXT        NOT NULL,
    type       TEXT        NOT NULL DEFAULT 'contact_first'
                           CHECK (type IN ('public', 'contact_first')),
    -- Follows from the latest contact attempt (D2). Only 'ok' shows a
    -- contact-first org.
    status     TEXT        NOT NULL DEFAULT 'not_contacted'
                           CHECK (status IN ('not_contacted', 'contacted', 'maybe', 'ok', 'leave_off')),
    follow_up  DATE,
    updated_by TEXT        NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE help_org_contacts (
    id         BIGSERIAL   PRIMARY KEY,
    host       TEXT        NOT NULL REFERENCES help_orgs(host) ON DELETE CASCADE,
    method     TEXT        NOT NULL CHECK (method IN ('email', 'call')),
    reached    TEXT        NOT NULL DEFAULT '',
    outcome    TEXT        NOT NULL
                           CHECK (outcome IN ('no_answer', 'left_message', 'maybe', 'yes', 'no')),
    note       TEXT        NOT NULL DEFAULT '',
    follow_up  DATE,
    logged_by  TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX help_org_contacts_host_idx ON help_org_contacts (host, created_at DESC);

-- The switch for D5: hiding turns on only after every Local Help page has
-- public help. One row; flipped by hand, never by a deploy.
CREATE TABLE help_hiding (
    only_row BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (only_row),
    hiding   BOOLEAN NOT NULL DEFAULT FALSE
);
INSERT INTO help_hiding DEFAULT VALUES;

-- The host of the first contact-first org a statement cites that has not
-- said yes, or NULL when nothing holds the statement back. Ignores the
-- switch, so the authoring view can say what will hide before it does.
CREATE FUNCTION statement_held_by(sid BIGINT) RETURNS TEXT
LANGUAGE sql STABLE AS $$
    SELECT source_host(src.url)
    FROM citations c
    JOIN sources src ON src.id = c.source_id
    LEFT JOIN help_orgs o ON o.host = source_host(src.url)
    WHERE c.statement_id = sid
      AND src.kind = 'nonprofit'
      AND (o.host IS NULL OR (o.type = 'contact_first' AND o.status <> 'ok'))
    ORDER BY source_host(src.url)
    LIMIT 1
$$;

-- Whether the public site hides the statement now.
CREATE FUNCTION statement_hidden(sid BIGINT) RETURNS BOOLEAN
LANGUAGE sql STABLE AS $$
    SELECT (SELECT hiding FROM help_hiding) AND statement_held_by(sid) IS NOT NULL
$$;
