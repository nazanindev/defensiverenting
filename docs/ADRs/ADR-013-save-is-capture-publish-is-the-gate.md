# ADR-013 — Saving captures, publishing guarantees

| | |
|---|---|
| Status | Accepted |
| Date | 2026-08-22 |

## Context

The authoring portal enforced its content invariants at save time: a statement without a citation, a quote that could not be confirmed, a statute locator naming no provision, a missing title — any of these refused the save. The publish step, by contrast, checked only one thing (citations without verbatim quotes, added with the publish guard).

That put the enforcement at the wrong boundary, and it hurt in both directions:

1. **Saving lost work.** A reviewer part-way through a twelve-statement page who hit one unverifiable quote could not save any of it. The form grew increasingly elaborate machinery (`preloadFromForm`, re-render preservation) to keep rejected work alive in the browser — the one place work should never have to live. A browser tab is not a database: a crash, a session timeout, or a mis-click discarded everything since the last accepted save.
2. **Publishing guaranteed little.** Because save was assumed strict, publish checked almost nothing. But the assumption did not hold across time: 17 of 19 legacy pages reached production with citations that were never verified, because they were saved before the checks existed. An invariant enforced at save protects only what is saved after the check ships; an invariant enforced at publish protects everything that crosses it, forever.

The drafting pipeline is different on both counts: the agent's `save_draft_playbook` verifies quotes as it writes and has no half-finished state worth capturing, so its guardrails are unchanged.

## Decision

### D1. Draft saves never refuse content

The portal's save paths (new page, draft edit, autosave) accept whatever is on the form: empty title, empty statements, uncited statements, unconfirmed quotes, bad statute locators. `AuthorUpdatePlaybook` drops its content validation for drafts; `IngestPlaybook` gains `AllowIncomplete`, valid only with status `draft`, which the portal sets and the agent/seeding paths do not. The only save-blocking conditions left are structural: a new page needs a jurisdiction and topic (there is no slot to save into without them), and infrastructure failures are still failures.

Two capture limits are surfaced rather than silently applied: a source card with no URL cannot be stored (sources are keyed by URL), and a reference-only domain may never become a source (the lawyer-blog rule, unchanged). Both come back as notes on the save instead of refusals of it.

### D2. One issues checker feeds the dashboard, the view page, and the publish gate

`internal/store/issues.go` computes the critical issues for a page from its stored rows: `no-title`, `no-statements`, `empty-statement`, `uncited-statement`, `missing-quote`, `unverified-quote`, `statute-locator`, `source-no-publisher`. The dashboard marks each draft carrying issues with a ⚠ count badge (details in the hover) and a "drafts not yet publishable" metric; the view page lists them above the publish button; the publish gate refuses on them. One implementation, three surfaces — what the author sees is exactly what publishing will say.

`unverified-quote` is the invariant D1 displaced, re-established at the gate: the old save path guaranteed every stored quote had been confirmed (fetched-and-matched or attested); now that unconfirmed text can be stored, `checked_at IS NULL` without attestation blocks publish instead. For the same reason `CitationQuoteExists` — the "already stored, skip the re-fetch" fast path — now matches only confirmed pairs, so storing an unverified quote cannot launder it into a verified one.

### D3. A live page's save runs the publish gate

Editing a published page applies on save with no second confirmation (that behavior predates this ADR). Those saves therefore *are* publishes, and `AuthorUpdatePlaybook` runs the same gate inside the same transaction when the page's status is `published`: a save that would leave the live page unpublishable is refused whole, with the full issue list, and writes nothing. Drafts get capture; the public site keeps the guarantee. Autosave is disabled on published pages — only a person's explicit Save gets to try the gate.

### D4. The form autosaves

Once a minute, if anything changed, the form posts itself with `autosave=1`; the server answers JSON (saved, and the current issue list) instead of redirecting, and the footer shows "Draft autosaved 12:04 · 2 issues to fix before publish". On the new-page form the first autosave creates the draft — but only into an empty slot: a timer never gets the save-over-the-existing-draft semantics a person clicking Save has always had; it declines and says whose draft is in the way. Autosaves skip live source fetches (a blocked source must not be hammered once a minute); new quotes stay unverified until a manual save or the form's blur-check confirms them, which the issue list reports honestly.

Because every save re-links a fresh set of statement rows, `AuthorUpdatePlaybook` now also deletes statements no playbook references, after the citation inserts have inherited their confirmation stamps from the rows being replaced — autosave would otherwise grow the statements table by one detached copy per minute of editing.

## Consequences

- Work lands in the database within a minute of being typed. The browser-side preservation machinery remains as a fallback for the errors that still re-render (infrastructure, live-page gate refusals).
- The publish gate now protects against everything the save gate used to, plus what the save gate historically missed. Legacy published pages carrying pre-gate issues show them on their view page; their next save or publish must fix them, and a manual save re-verifies reachable quotes automatically (the known-quote skip no longer accepts unconfirmed pairs).
- `cmd/promote` and `cmd/ingest` can still write `status='published'` rows without passing the gate; they are operator seeding tools, out of the authoring workflow. Tightening them is future work if they outlive the pilot.
- The old failure mode "one bad quote loses the session" is structurally gone rather than mitigated.
