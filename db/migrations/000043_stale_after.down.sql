DROP INDEX IF EXISTS statements_stale_after_idx;
ALTER TABLE statements DROP COLUMN IF EXISTS stale_after;
