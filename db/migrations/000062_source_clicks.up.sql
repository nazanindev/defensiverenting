-- ADR-029 D6: clicks on a source's link, and taps on a phone number in a
-- statement, counted per source per day. A reader counts once per source per
-- day, under the same daily salt and deletion rule as "Did this page help?"
-- (migration 000047): the hash is gone when the day ends, and the address is
-- never stored.
CREATE TABLE source_clicks (
    source_id BIGINT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    day       DATE   NOT NULL DEFAULT CURRENT_DATE,
    clicks    INT    NOT NULL DEFAULT 0,
    PRIMARY KEY (source_id, day)
);

CREATE TABLE click_seen (
    day       DATE   NOT NULL,
    reader    BYTEA  NOT NULL,
    source_id BIGINT NOT NULL,
    PRIMARY KEY (day, reader, source_id)
);
