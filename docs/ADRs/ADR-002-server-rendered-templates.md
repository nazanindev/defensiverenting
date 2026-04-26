# ADR-002 — Server-Rendered Go Templates over SPA

| | |
|---|---|
| Status | Accepted |
| Date | 2026-04-25 |

## Context

The site is content-heavy (playbooks, source citations), accessed by users in crisis who may be on slow connections or older devices. We need to choose between a separate JavaScript SPA and server-rendered HTML.

## Decision

Use Go's `html/template` (stdlib) to render full HTML pages on the server. No client-side framework.

## Rationale

1. **Delivery speed.** Server-rendered HTML is visible in one round trip. A SPA requires JS to load, parse, execute, and then fetch data before rendering — 3–5 round trips on a cold load.
2. **No JS build pipeline.** Zero webpack, zero bundler, zero `node_modules`. The repo stays simple, CI stays fast.
3. **Accessible by default.** Server-rendered HTML works without JavaScript. Screen readers and assistive technologies work on standard HTML without needing hydration tricks.
4. **Citation safety.** Template data types (`RenderedStatement` with `Citations []CitationChip`) enforce at compile time that the data passed to templates is well-typed. A statement cannot render without its citation slice being present in the struct.
5. **`/api/v1/*` is additive.** The browse handlers are thin (fetch → validate → render). Adding JSON variants is mechanical — same data, different `Accept` header path. We don't need to build the API now to preserve the option.

## Tradeoffs

- No client-side navigation means full-page reloads on topic switches. Acceptable for an informational site.
- Future interactive features (wizard, saved progress) will need progressive enhancement or Turbo/HTMX. Both are compatible with server-rendered HTML.

## Note on `a-h/templ`

The design doc specified `templ` for compile-time type safety. The initial implementation uses `html/template` because `templ` requires a code-generation step that isn't available in the current build environment. The citation guarantee is preserved via Go type constraints on the page structs. Migrating to `templ` is a targeted refactor once the tool is available — the data types and template structure are designed to make this straightforward.
