-- Rule values: a number lives once, in a typed row (ADR-034).
--
-- A rent-increase notice period, a deposit cap, a return deadline: today
-- each exists only inside the prose of a statement tagged to the concept.
-- This table holds the number itself, once per place, concept and
-- condition, with its own verbatim quote. Statements keep their prose; a
-- quality check compares the numbers in a body to these rows (D4), concept
-- pages read them for their 50-state tables (D5).
--
-- "No law found" is not a row here. It stays in coverage_records (ADR-028
-- D5): a fact about our search, with nothing to quote.

CREATE TABLE rule_values (
    id              BIGSERIAL   PRIMARY KEY,
    jurisdiction_id BIGINT      NOT NULL REFERENCES jurisdictions(id) ON DELETE CASCADE,
    concept_id      BIGINT      NOT NULL REFERENCES concepts(id) ON DELETE CASCADE,
    -- Plain words for when this value applies ("month-to-month", "increase
    -- over 10 percent"). Empty when the rule has one value. Part of the key,
    -- so a concept with two conditions in one place is two rows.
    condition       TEXT        NOT NULL DEFAULT '',
    kind            TEXT        NOT NULL CHECK (kind IN ('days', 'dollars', 'months_of_rent', 'percent', 'count')),
    value           NUMERIC     NOT NULL CHECK (value >= 0),
    -- Set when the law gives a range ("30 to 60 days"). NULL otherwise.
    value_max       NUMERIC     CHECK (value_max IS NULL OR value_max >= value),
    -- The citation, in the shape statements carry (D2). The quote is
    -- verbatim and non-empty, so the weekly checker can re-read it.
    source_id       BIGINT      NOT NULL REFERENCES sources(id),
    locator         TEXT        NOT NULL DEFAULT '',
    quote           TEXT        NOT NULL CHECK (btrim(quote) <> ''),
    -- For the reviewer: why this condition, what the statute calls it.
    note            TEXT        NOT NULL DEFAULT '',
    -- Who filed or last confirmed the row, and when.
    checked_by      TEXT        NOT NULL CHECK (btrim(checked_by) <> ''),
    checked_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Set by the source checker, as on advice_citations: when the quote was
    -- last found at the live source, and when it was last missing from it.
    source_checked_at TIMESTAMPTZ,
    drift_at          TIMESTAMPTZ,
    drift_note        TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (jurisdiction_id, concept_id, condition)
);

CREATE INDEX rule_values_concept_idx ON rule_values (concept_id);
CREATE INDEX rule_values_source_idx ON rule_values (source_id);

-- A number is a claim about the law. It is backed by law or official
-- guidance, never by the site's own editorial guidance.
CREATE FUNCTION rule_value_source_kind() RETURNS trigger AS $$
DECLARE k TEXT;
BEGIN
    SELECT kind INTO k FROM sources WHERE id = NEW.source_id;
    IF k = 'editorial' THEN
        RAISE EXCEPTION 'a rule value cites law or official guidance, not editorial guidance';
    END IF;
    RETURN NEW;
END $$ LANGUAGE plpgsql;

CREATE TRIGGER rule_value_source_kind
    BEFORE INSERT OR UPDATE ON rule_values
    FOR EACH ROW EXECUTE FUNCTION rule_value_source_kind();
