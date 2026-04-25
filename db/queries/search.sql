-- name: SearchStatements :many
-- Full-text search over statements, scoped to the given jurisdiction's ancestor chain.
WITH RECURSIVE ancestors AS (
    SELECT id, parent_id FROM jurisdictions WHERE id = $1
    UNION ALL
    SELECT j.id, j.parent_id
    FROM   jurisdictions j
    JOIN   ancestors a ON j.id = a.parent_id
),
query AS (SELECT plainto_tsquery('english', $2) AS q)
SELECT
    s.id,
    s.body_md,
    s.jurisdiction_id,
    ts_rank_cd(s.body_tsv, query.q) AS rank,
    j.slug AS jurisdiction_slug,
    t.slug AS topic_slug,
    p.slug AS playbook_slug,
    p.title AS playbook_title
FROM statements s
CROSS JOIN query
JOIN ancestors a ON a.id = s.jurisdiction_id
LEFT JOIN playbook_statements ps ON ps.statement_id = s.id
LEFT JOIN playbooks p ON p.id = ps.playbook_id
LEFT JOIN topics t ON t.id = p.topic_id
LEFT JOIN jurisdictions j ON j.id = s.jurisdiction_id
WHERE s.body_tsv @@ query.q
  AND s.language = $3
ORDER BY rank DESC
LIMIT 20;

-- name: SearchPlaybooks :many
WITH query AS (SELECT plainto_tsquery('english', $1) AS q)
SELECT
    p.id,
    p.slug,
    p.title,
    p.intro_md,
    ts_rank_cd(p.body_tsv, query.q) AS rank,
    j.slug AS jurisdiction_slug,
    t.slug AS topic_slug
FROM playbooks p
CROSS JOIN query
JOIN jurisdictions j ON j.id = p.jurisdiction_id
JOIN topics t ON t.id = p.topic_id
WHERE p.body_tsv @@ query.q
  AND ($2::BIGINT IS NULL OR p.jurisdiction_id = $2)
  AND p.language = $3
ORDER BY rank DESC
LIMIT 10;
