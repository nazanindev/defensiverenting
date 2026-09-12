DROP VIEW statement_review_hash;
ALTER TABLE statements DROP COLUMN reviewed_hash, DROP COLUMN reviewed_by;
UPDATE statements SET last_reviewed_at = NULL;
