# ADR-018 — Review is per statement; publishing is per page

| | |
|---|---|
| Status | Accepted; D1, D2, D3 shipped 2026-09-11 (migration 000041), D4 lists and D5 bulk actions pending |
| Date | 2026-09-11 |

## Context

The site's content model is modular by design: a page is an ordered set of atomic, individually cited statements (ADR-003), each with a durable key (ADR-014 D1), many tagged with a concept that recurs across jurisdictions (ADR-011). The review process ignores all of that. The only unit a reviewer can sign off is the page: `AuthorPublishPlaybook` stamps `playbooks.last_reviewed_at` and nothing else. `statements.last_reviewed_at` has existed since migration 000001 and has never been written.

Two consequences, both visible in the 2026-09-08 state wave (35 drafts, awaiting review):

1. **The agent's doubts live outside the database.** A drafting agent that is unsure of a claim writes the doubt into a markdown review sheet (`docs/review-states-wave1-2026-09-08.md`, 33 page entries). The portal cannot show it. The reviewer reads the page in one window and the sheet in another and joins them by statement number.
2. **Every page costs a full read.** Verification work that is naturally shared, one source cited by twelve statements on five pages, one concept stated by five states, is done page by page, twelve times. There is no way to record that a statement has been looked at without publishing the page around it.

The tools to do better already exist: keys, concepts, per-source issue reporting (`issues.go`), site-wide source usage, a proposal shape that may carry no proposed text (ADR-014 D4), and fetch receipts that record what a check actually saw (commit f9de323). This ADR uses them; it adds one column set and one input field.

## Decision

### D1. Agent doubts are filed as proposals on the statement they concern

`StatementInput` (the `save_draft_playbook` tool and `cmd/draft`) gains `reviewer_note`, optional free text: what the agent was unsure of and why, in one or two sentences. On save, each non-empty note files a proposal against the statement's key with reason `agent-pass:flag`, `proposed` NULL, and the note as evidence. Per ADR-014 D4 that is a work item, shown in the queue with "open in editor", and decided the usual way: rejected with a note ("checked, the statute is silent, the negative claim stands"), or superseded by an edit.

The drafting prompt tells the agent to use the field for exactly what the review sheets carried: inferred claims, simplifications, guidance-only support, time-sensitive figures, and what was left out. The review sheet as a document is retired. The wave-1 sheet is back-filled into the queue by a one-off script so no flag is lost.

### D2. A statement carries its own review stamp, bound to what was reviewed

`statements` gains `reviewed_by TEXT` and `reviewed_hash TEXT` beside the existing `last_reviewed_at`. Marking a statement reviewed stamps all three: the actor, the time, and a hash over the statement body, its concept, and every citation's URL, locator, and quote. A statement counts as reviewed only while the stored hash matches the hash of its current content. Any edit to the words or the evidence silently returns it to unreviewed. Nothing has to invalidate; the stamp is simply no longer true, the way a receipt hash is no longer true after drift.

The stamp cannot be set while the statement has a pending proposal of any reason. A doubt from D1, or a source-drift item from the checker, must be decided first. A snoozed proposal is a deliberate deferral and does not block.

Editorial-only statements (every citation is `kind = 'editorial'`) are still reviewed: the stamp is about the claim, not only its quotes.

**A person's save stamps what that person wrote** (amendment, 2026-09-11). When a named person saves a page, a statement that is new to its key, or whose content differs from every earlier row with that key, is stamped by them at that save. A statement they carried forward unchanged keeps whatever stamp it inherited, which may be none: saving a page is not reading every statement on it. The drafting agent and the source checker never stamp. Without this rule, every edit to a live page would be refused at the gate until the editor stamped by hand the statement they had just written, and approving a queue item (which is a save by the reviewer) could never pass the live gate. The approval decides the item before it saves for the same reason.

The stamp travels with the key across saves, the way a citation's confirmation does, and is judged against the hash on read. Pages that were already live when this shipped were back-filled by the migration: their statements are stamped by whoever last published the page, at the publish time, since publishing was the sign-off until now.

### D3. Publishing a page requires every statement reviewed

`collectIssues` gains two checks. `unreviewed-statement`: a non-directory page is not publishable while any statement lacks a valid stamp. `undecided-item`: no page of any kind publishes while a statement on it has a pending proposal; snoozing is how a reviewer defers one on purpose. The dashboard badges, the view page, and the gate show both together, as with every other issue (ADR-013). No second enforcement point. A page ingested straight to published (`cmd/ingest`, `cmd/promote`) is stamped whole by the person running the tool, like a directory.

Publishing continues to stamp `playbooks.last_reviewed_at` and continues to be a single-page, named-person action. What changes is that the person may have done the reading days earlier, in a different order, in groups.

**Directory pages are excluded.** On a directory the meaning of a statement comes from the organisation heading above it; the hours line means nothing alone. Directory pages keep page-level review: publishing one stamps every statement on it, and the `unreviewed-statement` check does not run against `page_kind = 'directory'`.

### D4. Review is done in groups, over three axes that already exist

Three list pages under `/review`, each showing statements with their citations and quotes in full, each with a "mark reviewed" action over the visible set:

- **By source.** Every statement citing one source, across every page, with the source's fetch receipt and the source text one click away. Recheck and attest work over the whole group (D5). This is where `source-unreachable` issues get worked: open the source once, attest twelve quotes.
- **By concept.** Every unreviewed statement tagged with one concept, one per jurisdiction, side by side. Inconsistency between states is visible at a glance.
- **By flag.** Statements with a pending `agent-pass:flag` proposal, worst first by page, which is what the review sheet was. This is the queue filtered by reason; it may be a filter on `/queue` rather than a separate page.

Lists, not boxes. Keyboard driven where the queue is.

The dashboard's draft list gains a per-page count, "n of m statements reviewed", beside the existing issue badge. The page view is the first group: each statement shows its stamp, or a mark-reviewed button, or a pointer to its undecided queue item, and the page offers to stamp the remainder in one action since every statement is rendered there with its quotes.

### D5. "Verify all" is four buttons, because it is four actions

- **Recheck quotes** (machine): re-fetch and re-match every quote in the group, recording receipts. Never stamps review. Exists today as the check run; this scopes it to a group.
- **Attest quotes** (human, per source): after opening the source, attest every unconfirmed quote from it in one action. Sets `manually_verified` and `checked_at` on each, with the actor. Only offered when the source's last receipt says it could not be read by the checker; a fetchable source is rechecked, not attested.
- **Mark reviewed** (human, per group): stamps D2 on every statement in the visible group. The group view is the page the stamp is about: the reviewer stamps only what is rendered in front of them, with quotes shown, never a count.
- **Publish ready pages** (human, dashboard): runs `AuthorPublishPlaybook` over every draft that carries zero issues, one transaction per page, and reports each refusal with its issue list. Nothing skips the gate.

### D6. What stays page-level

Whole-page coherence is still judged once, at publish, by the person clicking it: the intro, the order, whether the set of statements is the right set. Statement review does not replace that reading; it removes the citation-checking and fact-checking from it, which is most of the time.

## Consequences

- The wave-1 review sheet becomes queue items; future waves produce none. The drafting prompt and the subagent brief change together.
- Review effort scales with distinct claims and distinct sources, not with pages. A source cited on ten pages is verified once.
- A stamped statement that is later edited is unreviewed again with no code path to forget. The hash is the invariant, not a flag someone remembers to clear.
- Bulk publish is a loop over the existing gate. There is no bulk path that bypasses it, and no page publishes with an unreviewed statement or an undecided doubt.
- Cost: one migration (two columns), one field on the drafting input, one issue code, three list pages, four actions, one back-fill script. No change to the public site.
- Order of work: D1 with the back-fill (the queue is already the surface); D2 and D3 together, since a stamp nobody enforces is a flag; then D4 by-source, which unblocks the most `source-unreachable` issues; then by-concept and by-flag; then D5's publish button last, once there are ready pages to publish.
