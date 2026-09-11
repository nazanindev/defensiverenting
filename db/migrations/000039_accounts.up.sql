-- Reader accounts (ADR-017). Magic-link sign-in, email address only.
--
-- The point of an account today is one thing: carrying the reader's chosen
-- location across devices. Everything stored here is the minimum that makes a
-- link deliverable and a session recognisable. No name, no password, no
-- profile. Tokens are stored as SHA-256 hashes: a copy of this table cannot
-- sign anyone in, and a leaked row cannot be used against the person it
-- belongs to. Same posture as the forms Worker, which hashes IPs for the same
-- reason.
CREATE TABLE users (
    id         BIGSERIAL   PRIMARY KEY,
    email      TEXT        NOT NULL UNIQUE,   -- lower-cased, trimmed
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- A sign-in link. One use, short life. The row is keyed by the hash of the
-- secret in the link, never the secret itself. The email is copied in rather
-- than a user id because the user row is created only when a link is used:
-- typing a stranger's address into the form must not create an account for
-- them.
CREATE TABLE login_tokens (
    token_hash TEXT        PRIMARY KEY,
    email      TEXT        NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX login_tokens_email_created_idx ON login_tokens (email, created_at DESC);

-- A signed-in browser. Same hashing rule as login_tokens.
CREATE TABLE sessions (
    token_hash TEXT        PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX sessions_user_idx ON sessions (user_id);

-- What the reader has told us about themselves. One row per user, one column
-- per preference. The location references the jurisdiction row so a renamed
-- slug follows and a deleted place clears rather than dangles.
CREATE TABLE user_prefs (
    user_id     BIGINT      PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    location_id BIGINT      REFERENCES jurisdictions(id) ON DELETE SET NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
