-- A statement may quote one source more than once: two subsections of one
-- statute page, or two passages of one guidance page. The key was
-- (statement_id, source_id), so every save kept only the last quote per
-- source and dropped the rest without a word; between 2026-09-19 and
-- 2026-09-22 that collapsed 154 approved proposals. A citation is now its own
-- row. The unique index keeps a save idempotent: the same quote from the same
-- source on one statement is still one row. md5 because a whole-subsection
-- quote can be longer than a btree entry allows.
ALTER TABLE citations DROP CONSTRAINT citations_pkey;
ALTER TABLE citations ADD COLUMN id BIGSERIAL PRIMARY KEY;
CREATE UNIQUE INDEX citations_statement_source_quote ON citations (statement_id, source_id, md5(quote));
CREATE INDEX citations_statement_idx ON citations (statement_id);
