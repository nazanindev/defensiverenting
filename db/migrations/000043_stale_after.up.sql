-- A statement whose claim depends on a date (a sunset, a cap that is set for
-- one calendar year, a program that closes) goes stale on that date without
-- anything at the source changing. No fetch can detect it, so the statement
-- carries the date itself (ADR-024). NULL means the claim does not depend on
-- a date. The checker files a reviewer note before the date passes, so the
-- fix is in the queue while the page is still right.
ALTER TABLE statements ADD COLUMN stale_after DATE;

CREATE INDEX statements_stale_after_idx ON statements (stale_after) WHERE stale_after IS NOT NULL;
