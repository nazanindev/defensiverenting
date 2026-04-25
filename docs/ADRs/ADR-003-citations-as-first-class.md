# ADR-003 — Citations as a First-Class Data Primitive

| | |
|---|---|
| Status | Accepted |
| Date | 2026-04-25 |

## Context

The differentiating property of this product is the citation guarantee: every claim visible to a tenant traces to a primary source (statute, regulation, or government guidance). This guarantee must be maintained as the content corpus grows and new contributors add material.

## Decision

Enforce the citation guarantee at three independent layers so that no single failure mode can produce a citation-less statement in production.

## The Three Layers

### 1. Schema layer

The `citations` join table is the only way to associate a source with a statement. There is no `source_url` column on `statements`. The `IngestPlaybook` function wraps the entire playbook write in a single transaction and returns an error (aborting the transaction) if any `IngestStatementParams.Sources` slice is empty:

```go
if len(sp.Sources) == 0 {
    return fmt.Errorf("statement %d has no citations — ingest aborted", i)
}
```

No statement row can enter the database without at least one citation row in the same transaction.

### 2. Content-author layer

The `content.Parse` function enforces that every prose paragraph in a markdown file ends with a `[source-ref]` citation token. Files that fail this check exit with a non-zero status, failing the CI ingest check:

```
paragraph 3 has no citation reference — add [source-id] at the end
```

The `editorial` source ID is implicitly available for guidance that is accurate but not derivable from a single statute. Editorial statements cite the canonical `editorial` source row, which is seeded by migration and has `kind = 'editorial'`. The render layer displays a gray "Editorial guidance" chip instead of a blue statute chip, so readers always know the type of authority.

### 3. Render layer

The `handlers.Browse` handler validates that `CitedStatement.Citations` is non-empty before constructing `RenderedStatement`. In development this logs a loud error; in production it skips the statement so a UI bug never shows a citation-less claim to a tenant:

```go
if len(s.Citations) == 0 {
    logger.ErrorContext(ctx, "statement missing citations — invariant violated")
    continue
}
```

The `RenderedStatement` type itself carries a documented invariant: `Citations` is always non-empty.

## Consequences

- Content authors must add a citation reference to every prose paragraph. The parse error message is explicit and actionable.
- The `editorial` escape hatch preserves the guarantee without requiring authors to invent a citation — but editorial statements are first-class upgrade candidates as the corpus matures.
- Any future playbook import tool (org-submitted content, CSV import) must be written to pass non-empty `Sources` slices or it will fail transactionally.
