-- name: InsertStatement :one
INSERT INTO statements (jurisdiction_id, language, body_md)
VALUES ($1, $2, $3)
RETURNING id, jurisdiction_id, language, body_md, last_reviewed_at, created_at;

-- name: InsertCitation :exec
INSERT INTO citations (statement_id, source_id, locator)
VALUES ($1, $2, $3)
ON CONFLICT (statement_id, source_id) DO UPDATE SET locator = EXCLUDED.locator;

-- name: GetStatementCitations :many
SELECT
    c.statement_id,
    c.source_id,
    c.locator,
    s.url,
    s.publisher,
    s.kind
FROM citations c
JOIN sources s ON s.id = c.source_id
WHERE c.statement_id = $1;
