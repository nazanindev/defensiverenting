# ADR-001 — PostgreSQL over SQLite

| | |
|---|---|
| Status | Accepted |
| Date | 2026-04-25 |

## Context

We need a database that supports full-text search, recursive jurisdiction queries, and can grow to support pgvector embeddings and multi-jurisdiction concurrency. SQLite is simpler to operate but has material limitations for this workload.

## Decision

Use **PostgreSQL 16** as the primary datastore.

## Rationale

| Requirement | PostgreSQL | SQLite |
|---|---|---|
| Recursive CTE (`WITH RECURSIVE`) for jurisdiction ancestor traversal | ✅ | ✅ |
| `tsvector` / `tsquery` full-text search with GIN index | ✅ | ❌ (needs extension) |
| `pgvector` for embedding-based RAG (v2) | ✅ | ❌ |
| Connection pooling (`pgxpool`) for concurrent requests | ✅ | limited (WAL mode helps) |
| Managed cloud offering (Fly.io Postgres) | ✅ | ❌ |
| `GENERATED ALWAYS AS` columns | ✅ (v12+) | ✅ (v3.31+) |

The full-text search requirement alone (GIN-indexed `tsvector` generated columns) effectively requires Postgres. Adding pgvector later is a `CREATE EXTENSION` and a nullable column — no application migration needed if the column is seeded as nullable from day one.

## Consequences

- Local development requires a running Postgres instance. `compose.yaml` provides a one-command setup.
- CI uses `services: postgres` in GitHub Actions.
- Fly.io managed Postgres is ~$3/mo on the shared plan, within the $6/mo budget target.
