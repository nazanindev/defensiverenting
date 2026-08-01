-- Full-text search was hardcoded to the 'english' regconfig, which silently
-- mis-stems any non-English content. Map each row's language to its config
-- via an IMMUTABLE function (required for generated columns) and rebuild the
-- tsvector columns. Add languages by extending the CASE here.
CREATE FUNCTION lang_regconfig(lang TEXT) RETURNS regconfig AS $$
    SELECT CASE lang
        WHEN 'en' THEN 'english'::regconfig
        WHEN 'es' THEN 'spanish'::regconfig
        ELSE 'simple'::regconfig
    END
$$ LANGUAGE SQL IMMUTABLE;

ALTER TABLE statements DROP COLUMN body_tsv;
ALTER TABLE statements ADD COLUMN body_tsv TSVECTOR
    GENERATED ALWAYS AS (to_tsvector(lang_regconfig(language), body_md)) STORED;
CREATE INDEX statements_tsv_idx ON statements USING GIN (body_tsv);

ALTER TABLE playbooks DROP COLUMN body_tsv;
ALTER TABLE playbooks ADD COLUMN body_tsv TSVECTOR
    GENERATED ALWAYS AS (to_tsvector(lang_regconfig(language), title || ' ' || intro_md)) STORED;
CREATE INDEX playbooks_tsv_idx ON playbooks USING GIN (body_tsv);
