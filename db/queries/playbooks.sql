-- name: UpsertTopic :one
INSERT INTO topics (slug, name)
VALUES ($1, $2)
ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name
RETURNING id, slug, name;

-- name: ListTopicsByJurisdiction :many
SELECT t.id, t.slug, t.name
FROM topics t
JOIN playbooks p ON p.topic_id = t.id
WHERE p.jurisdiction_id = $1 AND p.language = $2
ORDER BY t.name;

-- name: GetPlaybookBySlug :one
SELECT
    p.id,
    p.jurisdiction_id,
    p.topic_id,
    p.language,
    p.slug,
    p.title,
    p.intro_md,
    p.last_reviewed_at
FROM playbooks p
JOIN jurisdictions j ON j.id = p.jurisdiction_id
JOIN topics       t ON t.id = p.topic_id
WHERE j.slug = $1
  AND t.slug = $2
  AND p.language = $3;

-- name: GetPlaybookStatements :many
-- Returns all statements for a playbook in order, with citations joined.
SELECT
    s.id            AS statement_id,
    s.body_md,
    ps.position,
    c.source_id,
    c.locator,
    src.url         AS source_url,
    src.publisher   AS source_publisher,
    src.kind        AS source_kind
FROM playbook_statements ps
JOIN statements s   ON s.id  = ps.statement_id
JOIN citations  c   ON c.statement_id = s.id
JOIN sources    src ON src.id = c.source_id
WHERE ps.playbook_id = $1
ORDER BY ps.position, c.source_id;

-- name: UpsertPlaybook :one
INSERT INTO playbooks (jurisdiction_id, topic_id, language, slug, title, intro_md)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (jurisdiction_id, topic_id, language) DO UPDATE
    SET slug    = EXCLUDED.slug,
        title   = EXCLUDED.title,
        intro_md = EXCLUDED.intro_md
RETURNING id, jurisdiction_id, topic_id, language, slug, title, intro_md, last_reviewed_at;

-- name: DeletePlaybookStatements :exec
DELETE FROM playbook_statements WHERE playbook_id = $1;

-- name: InsertPlaybookStatement :exec
INSERT INTO playbook_statements (playbook_id, statement_id, position)
VALUES ($1, $2, $3);
