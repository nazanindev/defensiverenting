-- A note placed under a statement goes back to the top of its page.
UPDATE playbook_advice pa SET statement_key = NULL FROM advice a
WHERE pa.advice_id = a.id AND a.kind = 'page_note' AND pa.statement_key IS NOT NULL;

CREATE OR REPLACE FUNCTION playbook_advice_shape() RETURNS trigger AS $$
DECLARE k TEXT;
BEGIN
    SELECT kind INTO k FROM advice WHERE id = NEW.advice_id;
    IF k = 'tip' AND NEW.statement_key IS NULL THEN
        RAISE EXCEPTION 'a tip must name the statement it sits under';
    END IF;
    IF k = 'page_note' AND NEW.statement_key IS NOT NULL THEN
        RAISE EXCEPTION 'a page note belongs to the page, not a statement';
    END IF;
    RETURN NEW;
END $$ LANGUAGE plpgsql;

ALTER TABLE advice DROP COLUMN lead;
