ALTER TABLE statements DROP COLUMN body_tsv;
ALTER TABLE statements ADD COLUMN body_tsv TSVECTOR
    GENERATED ALWAYS AS (to_tsvector('english', body_md)) STORED;
CREATE INDEX statements_tsv_idx ON statements USING GIN (body_tsv);

ALTER TABLE playbooks DROP COLUMN body_tsv;
ALTER TABLE playbooks ADD COLUMN body_tsv TSVECTOR
    GENERATED ALWAYS AS (to_tsvector('english', title || ' ' || intro_md)) STORED;
CREATE INDEX playbooks_tsv_idx ON playbooks USING GIN (body_tsv);

DROP FUNCTION lang_regconfig(TEXT);
