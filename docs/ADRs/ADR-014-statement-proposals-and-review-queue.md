# ADR-014 — Statement proposals and the author review queue

| | |
|---|---|
| Status | Accepted, shipped 2026-09-08 (quality scorer deferred to its own ADR) |
| Date | 2026-09-08 |

## Context

Every change to a live page today arrives as a whole page. An agent research pass writes a draft revision beside the published page (one per slot, see `AuthorUpdatePlaybook`); the author diffs it in their head against the live version and publishes or discards all of it. The source checker (`cmd/check-sources`) works one level down but stops short of the page: when a cited quote no longer appears at its URL it stamps `flagged_at` on the *source* and the dashboard lists the URL at the top with a dismiss button. Which statement broke, and what the source says now, the author has to rediscover by hand.

Three kinds of work share a shape the portal has no unit for: a machine proposes a change to one statement, and a person decides.

1. **Bulk agent passes.** Re-running the sources on twenty pages, retagging concepts, tightening a locator format. Each pass touches a few statements per page on pages that are otherwise fine. Filing twenty draft revisions asks the author to re-read twenty pages to find three changed sentences each.
2. **Source drift.** The Wayback repoint work (2026-08-22) found two real LAHD drifts. Each is one statement whose quote moved. The answer is a per-statement edit with the new source text in front of the reviewer, not a source-level flag with no edit attached.
3. **Source quality.** Citations that are adequate today but structurally weak: guidance cited where a statute exists, an archive snapshot standing in for a live URL, a PDF only the browser tier can fetch, a statement carried by one citation. No signal is collected today, and when it is, its output is again "look at this one statement".

The candidate-source queue (`source_candidates`: pending, approved, rejected, snoozed) already proves the pattern works for authors at source granularity. This ADR brings it to statements.

One structural fact blocks a direct build. Statements have no stable identity. Every save, including the once-a-minute autosave from ADR-013 D4, re-links a fresh set of `statements` rows and deletes the orphans. A queue item that points at statement id 4821 goes stale the moment anyone saves the page.

## Decision

### D1. Statements get a durable key that survives saves

`statements` gains `key UUID NOT NULL DEFAULT gen_random_uuid()`. The edit form round-trips it as a hidden field per statement; a statement added on the form gets none and the save mints one. `AuthorUpdatePlaybook` and `IngestPlaybook` accept an optional key per statement and carry it onto the replacement row. The drafting agent's `save_draft_playbook` does the same when it revises an existing page, so a revision keeps the keys of the statements it kept. Deleting a statement retires its key; reordering does not change it.

The key is the identity of a claim across edits. It is what statement-level history, Spanish counterparts, and this queue all need, and it is the reason this ADR is worth doing even if the queue were never built.

### D2. A proposal is a whole replacement statement, filed against a key

```
statement_proposals
  id             BIGSERIAL
  statement_key  UUID        the claim this replaces
  playbook_id    BIGINT      the page it was observed on, for listing and grouping
  reason         TEXT        closed vocabulary, see D4
  proposed       JSONB       the full replacement in IngestStatementParams shape,
                             or NULL when the proposer has nothing to offer (D4)
  evidence       JSONB       reason-specific: old and new quote, source excerpt,
                             quality signals; rendered, never interpreted
  proposed_by    TEXT        actor stamp: agent name or person, same rule as updated_by
  status         TEXT        pending | approved | rejected | snoozed | superseded
  decided_by     TEXT, decided_at TIMESTAMPTZ, decision_note TEXT
  snoozed_until  TIMESTAMPTZ
  created_at     TIMESTAMPTZ
```

A proposal is the *whole* statement as it should read afterward: body, concept, citations with quotes. Not a patch. The reviewer sees old and proposed side by side and either approves, edits then approves, rejects with a note, or snoozes. A second proposal against a key with one still pending supersedes the first; the queue never shows two competing edits for one claim.

Proposals are filed through the store (`FileProposal`), reached by a CLI (`cmd/propose`, reads JSON) and by an MCP tool (`propose_statement`) so both agent front-ends can file from day one. Neither path writes to `statements`. The MCP tool holds the replacement to the same guardrails as `save_draft_playbook` (voice lint, verbatim quotes from this session's fetches, no reference-only sources) and files each verified quote as checked, so the approval can stamp it on the proposer's word.

### D3. Approval is a save, so the gates already built keep holding

Approving a proposal loads the page, substitutes the statement under that key, and runs it through `AuthorUpdatePlaybook` with the reviewer as `updated_by`. Nothing else changes. On a draft, that is a capture (ADR-013 D1). On a live page, that save *is* a publish and runs the live gate (ADR-013 D3): an approval that would leave the page unpublishable is refused whole, with the issue list, and the proposal stays pending. Quotes in a proposal follow the citation rules unchanged: a quote the proposer confirmed arrives `CheckedNow`; one it did not stays unverified and blocks at the gate until the reviewer's save checks it live. There is no second enforcement point and no way for an approval to launder text past the checker.

A slot holds at most one draft revision. When a page has a pending draft revision, proposals against its keys apply to the revision, not the live page; the revision is where that slot's next version is being assembled. The queue shows which one it will touch.

### D4. Reason codes are closed, and a proposal may carry no proposed text

`reason` is a registry, extended by migration like topics and concepts (ADR-005 D5, ADR-011 D1):

- `agent-pass:<name>`: a named bulk pass. The name is free text after the prefix and identifies the run in the queue's grouping.
- `source-drift`: filed by the checker instead of `flagged_at`. The checker knows the quote vanished; it does not know the replacement. Evidence carries the old quote and the nearest passage in the new fetch (fuzzy match). `proposed` is the same statement with the citation's quote swapped for that passage when the match is close, and NULL when it is not. The reviewer edits either way. `sources.flagged_at` and the dashboard flag list are retired once the checker files proposals; the dismiss button becomes a rejection with a note, which is what it always meant.
- `source-quality:<signal>`: filed by a scorer that does not exist yet. Signals worth collecting when it does: `guidance-not-statute`, `archive-url`, `browser-tier-only`, `no-locator`, `single-citation`. `proposed` is NULL, or a statement with an added citation when `find_sources` turns up a candidate. This code is reserved here so the table shape does not need to change; the scorer is a later ADR.

A proposal with NULL `proposed` is a work item, not an edit. The queue shows it with an "open in editor" action instead of approve.

### D5. The queue is one page, keyed by claim, in reading order

`/queue` lists pending proposals grouped by page then position, with reason, proposer, age, and a one-line summary of the change. Each row opens to old and proposed statement side by side, the evidence rendered beneath, and the four actions. Keyboard driven, one item at a time, next item on decision. It is a list (see the browse-surface rule: lists, not boxes). Snoozed items return on their date; rejected and approved items stay readable under a filter, because a rejection with a note is the record of why a source was judged fine.

The dashboard gets one metric, "N proposals waiting", and loses the flagged-source block once D4's checker change lands.

### D6. Language scope

A proposal targets one statement key, and keys are per language. A change approved on an English statement does not file anything against its Spanish counterpart. Cross-language follow-through needs a link between counterpart keys, which is the same link the translation tooling (ADR-008) needs and does not have yet. Out of scope here; the queue shows the page language so a reviewer knows which one they are editing.

## Consequences

- Agent passes stop producing whole-page revisions for statement-scale changes. Whole-page rewrites stay draft revisions; a seven-statement rewrite reviewed as seven disjoint approvals would lose page-level coherence.
- Source drift becomes a per-statement work item with the new text attached, and its dismissal becomes a recorded decision rather than a cleared flag.
- Every approval is an ordinary save by a named person and is subject to the publish gate, so the ADR-013 guarantee is unchanged.
- The statement key is a migration and a form change that touch every save path. It ships first, alone, and is verified before any proposal is filed.
- Order of work: D1; then D2, D3, D5 with `cmd/propose` as the only producer; then D4's checker change; then a quality scorer under its own ADR.
