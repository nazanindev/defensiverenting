# Tenant Rights

Plain-language, citation-backed tenant rights guides. Every claim a tenant reads traces to a primary source — a statute, regulation, or government document — that they can verify themselves.

Live: [defensiverenting.fly.dev](https://defensiverenting.fly.dev)

---

## What makes this interesting (architecturally)

### Citation guarantee enforced at three independent layers

The differentiating property of the product is that no statement can ever appear on screen without a citation. This is enforced at three layers so no single failure mode breaks it:

**1. Schema layer.** There is no `source_url` column on `statements`. Citations live in a separate `citations` join table. The ingest function wraps every playbook write in a single transaction and aborts if any statement arrives with an empty sources slice:

```go
if len(sp.Sources) == 0 {
    return fmt.Errorf("statement %d has no citations — ingest aborted", i)
}
```

**2. Content-author layer.** The markdown parser requires every prose paragraph to end with a `[source-ref]` citation token. Files that fail this check exit non-zero, failing CI before anything reaches the database.

**3. Render layer.** The browse handler validates `Citations` is non-empty before constructing a `RenderedStatement`. In production it skips the statement silently; in development it logs loudly. A citation-less claim cannot reach the UI.

The editorial escape hatch (for objective practical guidance that isn't derivable from a single statute) preserves the guarantee: it cites a canonical `editorial` source row seeded at migration time, rendered with a distinct gray chip so readers always know the authority type.

---

### Schema design

```sql
jurisdictions  →  self-referential (country → state → city)
sources        →  url, publisher, kind, content_hash, retrieved_at
statements     →  body_md, body_tsv (generated), embedding vector (nullable)
citations      →  statement_id, source_id, locator
playbooks      →  jurisdiction_id, topic_id, language, intro_md
```

**Recursive jurisdiction queries.** A tenant in Boston inherits Boston + Massachusetts + federal rules. A single `WITH RECURSIVE` CTE pulls all ancestor rows; application code never needs to know the hierarchy depth.

**Full-text search without an external service.** `tsvector` generated columns on `statements.body_md` and `playbooks.title || intro_md`, GIN-indexed, queried with `ts_rank_cd`. No Elasticsearch, no Meilisearch, no sync job.

**pgvector seam.** `statements.embedding vector` is nullable from day one. Populating it for RAG retrieval is a v2 offline job — no schema migration required when that work starts.

**Org extensibility without a rewrite.** When tenant organizations contribute content (v3), the migration is additive:

```sql
ALTER TABLE sources    ADD COLUMN org_id BIGINT REFERENCES organizations(id);
ALTER TABLE statements ADD COLUMN org_id BIGINT REFERENCES organizations(id);
ALTER TABLE playbooks  ADD COLUMN org_id BIGINT REFERENCES organizations(id);
```

`NULL` means platform-authored. Existing rows, routes, and tests require no changes.

---

### Stack

| Layer | Choice | Why |
|---|---|---|
| Language | Go 1.23 | Single binary, fast compile, straightforward concurrency |
| Database | PostgreSQL 16 | `tsvector` search, recursive CTEs, pgvector path |
| Query layer | sqlc + pgxpool | Type-safe generated queries, connection pooling |
| Migrations | golang-migrate | Versioned, transactional, CI-verified |
| Router | chi | Lightweight, idiomatic middleware chaining |
| Templates | `html/template` | Compiled into binary, type-checked page structs |
| CI | GitHub Actions | vet, lint, race tests against a real Postgres service container |
| Hosting | Fly.io | Distroless image, nonroot, auto-stop machines (~$3/mo idle) |

---

### Running locally

```bash
docker compose up -d db
docker compose run --rm ingest
go run ./cmd/server
```

Or bring up everything at once:

```bash
docker compose --profile ingest up
```

Tests run against a real Postgres (no mocks):

```bash
go test ./... -race
```

---

### Design decisions

Full context lives in [`docs/DESIGN.md`](docs/DESIGN.md) and four ADRs:

- [ADR-001](docs/ADRs/ADR-001-postgres-over-sqlite.md) — PostgreSQL over SQLite
- [ADR-002](docs/ADRs/ADR-002-server-rendered-templates.md) — Server-rendered templates over SPA
- [ADR-003](docs/ADRs/ADR-003-citations-as-first-class.md) — Citations as a first-class data primitive
- [ADR-004](docs/ADRs/ADR-004-org-extensibility.md) — Org extensibility without schema rewrite
