-- "Did this page help?" (2026-10-04). One row per page per day with a yes
-- and a no count. Nothing about the reader is stored: no cookie, no IP, no
-- account. A page is its place, topic and language, not its playbook row,
-- so a republished revision keeps counting on the same line.
CREATE TABLE page_helpful (
    jurisdiction_id BIGINT NOT NULL REFERENCES jurisdictions(id),
    topic_id        BIGINT NOT NULL REFERENCES topics(id),
    language        TEXT   NOT NULL,
    day             DATE   NOT NULL DEFAULT CURRENT_DATE,
    yes             INT    NOT NULL DEFAULT 0,
    no              INT    NOT NULL DEFAULT 0,
    PRIMARY KEY (jurisdiction_id, topic_id, language, day)
);
