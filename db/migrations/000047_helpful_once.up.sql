-- "Did this page help?" counts one answer per reader per page per day, and at
-- most a few dozen answers per reader per day across all pages (2026-10-04).
--
-- A reader is a one-way hash of their IP address and a random salt made for
-- that day. Yesterday's salt and hashes are deleted on the first answer of a
-- new day, so a hash can only be matched to an address on the day it was made,
-- and never after. The IP address itself is never stored.
CREATE TABLE helpful_salt (
    day  DATE  PRIMARY KEY,
    salt BYTEA NOT NULL
);

CREATE TABLE helpful_seen (
    day             DATE   NOT NULL,
    reader          BYTEA  NOT NULL,
    jurisdiction_id BIGINT NOT NULL,
    topic_id        BIGINT NOT NULL,
    language        TEXT   NOT NULL,
    PRIMARY KEY (day, reader, jurisdiction_id, topic_id, language)
);
