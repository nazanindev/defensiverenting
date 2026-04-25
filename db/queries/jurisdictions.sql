-- name: ListCityJurisdictions :many
SELECT id, parent_id, kind, name, slug
FROM jurisdictions
WHERE kind = 'city'
ORDER BY name;

-- name: GetJurisdictionBySlug :one
SELECT id, parent_id, kind, name, slug
FROM jurisdictions
WHERE slug = $1;

-- name: GetJurisdictionAncestors :many
-- Returns jurisdiction + all ancestors (walk up the parent chain).
WITH RECURSIVE ancestors AS (
    SELECT id, parent_id, kind, name, slug
    FROM   jurisdictions
    WHERE  id = $1
    UNION ALL
    SELECT j.id, j.parent_id, j.kind, j.name, j.slug
    FROM   jurisdictions j
    JOIN   ancestors a ON j.id = a.parent_id
)
SELECT id, parent_id, kind, name, slug FROM ancestors;

-- name: UpsertJurisdiction :one
INSERT INTO jurisdictions (parent_id, kind, name, slug)
VALUES ($1, $2, $3, $4)
ON CONFLICT (slug) DO UPDATE
    SET name = EXCLUDED.name, kind = EXCLUDED.kind, parent_id = EXCLUDED.parent_id
RETURNING id, parent_id, kind, name, slug;
