-- name: UpsertSource :one
INSERT INTO sources (url, publisher, jurisdiction_id, kind, retrieved_at, content_hash)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (url) DO UPDATE
    SET publisher        = EXCLUDED.publisher,
        jurisdiction_id  = EXCLUDED.jurisdiction_id,
        kind             = EXCLUDED.kind,
        retrieved_at     = EXCLUDED.retrieved_at,
        content_hash     = EXCLUDED.content_hash
RETURNING id, url, publisher, jurisdiction_id, kind, retrieved_at, content_hash;

-- name: GetEditorialSource :one
SELECT id, url, publisher, jurisdiction_id, kind, retrieved_at, content_hash
FROM sources
WHERE kind = 'editorial'
LIMIT 1;
