CREATE OR REPLACE FUNCTION advice_citation_source_kind() RETURNS trigger AS $$
DECLARE k TEXT;
BEGIN
    SELECT kind INTO k FROM sources WHERE id = NEW.source_id;
    IF k NOT IN ('gov_guidance', 'nonprofit') THEN
        RAISE EXCEPTION 'advice may cite only government guidance or a nonprofit, not a % source', k;
    END IF;
    RETURN NEW;
END $$ LANGUAGE plpgsql;
