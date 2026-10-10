-- An advice source must say where it applies (ADR-016 A1): a state or city,
-- or the United States for a national source. A source with no place used
-- to count as national, so a state guide saved without its state would have
-- backed advice in every state.
UPDATE sources s SET jurisdiction_id = j.id
FROM jurisdictions j
WHERE j.slug = 'united-states' AND j.kind = 'country' AND s.jurisdiction_id IS NULL
  AND s.id IN (SELECT source_id FROM advice_citations);

CREATE OR REPLACE FUNCTION advice_citation_source_kind() RETURNS trigger AS $$
DECLARE k TEXT; j BIGINT;
BEGIN
    SELECT kind, jurisdiction_id INTO k, j FROM sources WHERE id = NEW.source_id;
    IF k NOT IN ('gov_guidance', 'nonprofit') THEN
        RAISE EXCEPTION 'advice may cite only government guidance or a nonprofit, not a % source', k;
    END IF;
    IF j IS NULL THEN
        RAISE EXCEPTION 'an advice source needs a place: its state or city, or the United States when national';
    END IF;
    RETURN NEW;
END $$ LANGUAGE plpgsql;
