-- A page note can carry a short bold lead, so several notes read as
-- separate items instead of one block (Nazanin 2026-10-13: "the warnings on
-- most of these pages are just a big block of text"). Leads are filled in
-- once she has read them; a note without one renders as before.
ALTER TABLE advice ADD COLUMN lead TEXT CHECK (lead IS NULL OR (kind = 'page_note' AND btrim(lead) <> ''));

-- A page note may sit under the one statement it is about (the inspector
-- warning under the repair statement) instead of at the top. It is still
-- said once per page.
CREATE OR REPLACE FUNCTION playbook_advice_shape() RETURNS trigger AS $$
DECLARE k TEXT;
BEGIN
    SELECT kind INTO k FROM advice WHERE id = NEW.advice_id;
    IF k = 'tip' AND NEW.statement_key IS NULL THEN
        RAISE EXCEPTION 'a tip must name the statement it sits under';
    END IF;
    IF k = 'page_note' AND EXISTS (
        SELECT 1 FROM playbook_advice
        WHERE playbook_id = NEW.playbook_id AND advice_id = NEW.advice_id
          AND COALESCE(statement_key::text, '') <> COALESCE(NEW.statement_key::text, '')) THEN
        RAISE EXCEPTION 'a page note is said once per page';
    END IF;
    RETURN NEW;
END $$ LANGUAGE plpgsql;
