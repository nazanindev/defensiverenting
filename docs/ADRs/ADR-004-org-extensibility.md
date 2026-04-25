# ADR-004 — Org Extensibility Without Schema Rewrite

| | |
|---|---|
| Status | Accepted |
| Date | 2026-04-25 |

## Context

The long-term vision includes local tenant organizations contributing their own content, playbooks, and source citations for jurisdictions where they have domain depth. We need to design the MVP schema so that org accounts can be added later without a destructive migration.

## Decision

Reserve org extensibility via additive nullable foreign keys and optional join tables. No org-specific columns or tables are created in the MVP — only the architectural seam is documented here.

## The Seam

When org accounts are implemented (v3), the migration will:

```sql
CREATE TABLE organizations (
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL UNIQUE,
    verified_at TIMESTAMPTZ
);

-- Additive: existing rows are NULL (= platform-authored content)
ALTER TABLE sources    ADD COLUMN org_id BIGINT REFERENCES organizations(id);
ALTER TABLE statements ADD COLUMN org_id BIGINT REFERENCES organizations(id);
ALTER TABLE playbooks  ADD COLUMN org_id BIGINT REFERENCES organizations(id);
```

No existing rows change. No application code that doesn't mention `org_id` is affected. Queries that need org filtering add `AND (org_id IS NULL OR org_id = $org_id)`.

## Why Nullable Rather Than a Default Value

Using `NULL` to mean "platform-authored" avoids creating a placeholder organization row that would need to be excluded from org-scoped queries. `IS NULL` is idiomatic and index-friendly.

## Draft/Publish Flow (v3)

A per-org draft/publish workflow would add:

```sql
ALTER TABLE playbooks ADD COLUMN status TEXT NOT NULL DEFAULT 'published'
    CHECK (status IN ('draft', 'review', 'published'));
```

Browse routes filter on `status = 'published'`. Org editors see drafts in their dashboard. Moderation queue = `WHERE status = 'review' AND org_id IS NOT NULL`.

## Consequences

- MVP has no org surface area — no UI, no API, no tables.
- The v3 migration is additive and non-breaking. Existing data, routes, and tests require no changes.
- Content ownership is always attributable: `org_id IS NULL` → Tenant Rights editorial; `org_id IS NOT NULL` → named organization.
