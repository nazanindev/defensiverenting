DROP INDEX IF EXISTS statements_key_idx;
ALTER TABLE statements DROP COLUMN IF EXISTS key;
