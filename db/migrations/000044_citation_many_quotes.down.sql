-- Back to one citation per source per statement. Keeps the first row of each
-- pair, so quotes added since the up migration are lost.
DELETE FROM citations c USING citations d
 WHERE c.statement_id = d.statement_id AND c.source_id = d.source_id AND c.id > d.id;
DROP INDEX citations_statement_idx;
DROP INDEX citations_statement_source_quote;
ALTER TABLE citations DROP CONSTRAINT citations_pkey;
ALTER TABLE citations DROP COLUMN id;
ALTER TABLE citations ADD PRIMARY KEY (statement_id, source_id);
