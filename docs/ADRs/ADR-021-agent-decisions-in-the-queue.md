# ADR-021 — The review agent decides queue items by stated rule, starting with widened quotes

| | |
|---|---|
| Status | Accepted 2026-09-20; D1 to D6 shipped 2026-09-20 with rules widen and flag |
| Date | 2026-09-20 |
| Amends | ADR-014 (a third decider in the queue), ADR-018 D2 (a stamp survives a widened quote) |

## Context

Fifty states by the end of September means about three hundred more pages and four thousand more statements. Every one of them arrives with its quotes already confirmed verbatim at the source by the checker. What a person adds at the Done button is judgement: does the quote support the claim, is the claim right. That judgement is the work, and at this volume one person cannot supply all of it in time.

The review queue (ADR-014) is where the site already makes bounded decisions. A proposal is a specific change with evidence attached, filed under a reason, decided by a named actor with a note. It is narrower than "is this statement true": it asks "is this change, with this evidence, right". That makes it the right place to let an agent decide first, because every decision it makes is recorded the same way a person's is and can be read back, and because the classes of proposal differ sharply in what a wrong decision can touch.

On 2026-09-20 the queue held 284 pending items. 138 were widen-quote proposals filed by `triage widen`: the statement's body and tags unchanged, one statute quote grown from a fragment to the whole subsection. The public site never renders quotes. Approving one cannot change a word a reader sees. It can only change what the checker watches for drift, and it changes it to more of the same passage.

The store already distinguishes people from the two non-human writers (`isReviewer`): the drafting agent and the source check never produce a review stamp. The queue's Apply button and the statement card's Resolved button approve through one function in the authoring server, which resolves sources, fetches every quote live, and saves the replacement under the approver's name.

## Decision

### D1. A third non-human actor, the review agent, decides through the person's path

`store.ActorReviewAgent` ("review agent") joins the drafting agent and the source check as a writer that is not a reviewer. It approves a proposal through the same path a person's click takes, now `drafting.ApplyProposal`, shared by the authoring server and the command line. Same source resolution, same live quote fetch, same publish gate on a live page. The decision is recorded on the proposal with `decided_by` the agent's name and a `decision_note` naming the rule it applied and what it checked. `ApproveProposalParams` gains `Note` for that; a person's click leaves it empty.

The agent never stamps a statement reviewed. `MarkStatementsReviewed` and the attest actions refuse it like the other non-human actors, and a save under its name does not run `stampIfWritten`. A statement whose words it changed reads as unreviewed until a person reads it.

### D2. A widened quote keeps the person's stamp

The review stamp is a hash of the body, tags, and every citation's source, locator and quote (migration 000041). Under that rule, widening a quote would return a reviewed statement to unreviewed, and on a live page the gate would refuse the save. That is the wrong reading. The person reviewed the claim and its evidence; a quote that contains what they read, from the same source at the same locator, is the same evidence with more of it shown.

So a save carries the stamp forward when the prior row with the same key had a valid stamp and the new row differs only in quotes, each of which contains the old quote after whitespace is collapsed. Same body, same tags, same sources at the same locators, no citation added or dropped. The stamp keeps its reviewer and its time and moves onto the new hash. Anything else that differs leaves the stamp behind, as before. This runs for every actor, so a person widening a quote by hand keeps their earlier stamp too, and `stampIfWritten` after it is a no-op when the hash already matches.

### D3. Rules, one at a time, each stated in code and in the decision note

The agent decides only what a stated rule decides. A rule is a pure function over the proposal and the statement as it reads on the target page today, plus the live checks the approval path already performs. What a rule does not decide stays pending for a person, with the reason printed. The agent has no reject verb yet; refusing is a person's call until a rule earns it.

**Rule widen** (shipped): the proposal's reason is `agent-pass:widen-quote`; body, concept and topic reference identical to the current statement; the same non-editorial sources at the same locators, none added or dropped; every quote that changes contains the current quote and stays under the widen cap of 2,500 words; at least one changes; and every changed quote is found verbatim at the live source, direct or rendered, never a snapshot. A source that cannot be read from here leaves the item for a person; the agent does not take a proposer's word for a quote.

**Rule flag** (shipped 2026-09-20, the first rule with judgement in it): a reviewer flag (`agent-pass:flag`) on a draft page is read by a second model, Claude Opus 5, independent of the drafter, which sees only what the card shows: the statement, the doubt, and the fetched text of each cited source, cut at 80,000 characters and told so. It has no tools and cannot look anywhere the statement does not already cite. It answers "stands" only by quoting the passage that settles the doubt. The invariants in code: the passage must appear verbatim in one fetched source (the same match the checker uses), be at most 150 words, and come with a reason. A "stands" that fails any of these is thrown away and the item stays pending. A "stands" that passes closes the flag the way `triage stands` does for a person, rejected with the model, the reason, and the passage in the decision note. Everything else stays for a person. Published pages are out of scope for this rule.

Rules planned, in order of what a wrong decision can touch, each its own amendment here: triage edits on draft pages; drift findings that re-cite the same section. Body edits on published pages stay a person's decision until the earlier rules have a clean audit.

### D4. The audit is one command, and overturns are the measure

`triage decide audit` lists every proposal the review agent decided, newest first, with the page, the statement, and the note. A person reads it after each run. The measure of a rule is overturns: a person later rejecting or reversing what the agent approved. The first rule to earn a successor is the one with none. A rule with overturns is narrowed or withdrawn, and its past decisions re-read.

### D5. Where it runs

Like the triage pass, the agent runs on a person's machine over the tunnel, by `triage decide widen -apply`. Nothing in production decides anything. Without `-apply` the command prints what it would do, and that dry run is the first thing to read before any batch.

### D6. An approval on a live page is not a publish over the other open items (amendment, 2026-09-20)

The first `-apply` run approved 39 and had 74 refused at the save: the page was live, and the gate's `undecided-item` check counted the other pending widen items on the same page, queued right behind the one being applied. On a page with five, the first four fail and only the last can go through. A person's Apply click had the same limit; it had simply never been hit five times on one page.

The check exists so nobody publishes a page over an open question (ADR-018 D3). An approval is a decision about one change. The other items stay pending, stay visible, keep the statement unstampable and the page unpublishable by hand. So a save that applies a decided proposal (`AuthorUpdatePlaybookParams.Approval`) runs the live gate without `undecided-item`; every other check runs in full, and `AuthorPublishPlaybook` still counts it. In-place edits on the statement card are not approvals and keep the full gate.

The same run showed that the D2 carry refused to carry when the statement had an open reviewer note. The note blocks a new stamp; the carried one is the old stamp, which the note never invalidated. The carry now ignores undecided items.

## Consequences

- The 138 widen-quote items, and every one `triage widen` files after them, no longer wait on a person. The queue holds what needs judgement.
- A statement's stamp now survives a quote widening, on every path. Fewer live pages fall back to unreviewed after the checker's own housekeeping.
- The authoring server's apply path moved into `internal/drafting` and lost nothing. One function to change when the approval rules change.
- The trust line can say what is true: quotes confirmed at the source, changes decided by a named actor with the rule on record, every statement read by a person before it is published.

## Rejected

- **Letting the agent stamp statements Done.** The stamp is the site's claim that a person read the sentence. An agent's pass is a different fact and deserves a different column; ADR-021's successor for statement review, if there is one, adds that column rather than borrowing this one.
- **A general "agent-approved" flag on proposals.** The decision is the same decision whoever makes it. The actor name and the note carry the difference; a second status would fork the queue.
- **Auto-rejecting what the rule cannot decide.** "Not decidable by this rule" is not "wrong". Those stay pending.
- **Running the agent in production on a schedule.** The pipeline's shape is: production detects, a local agent proposes and now decides over the tunnel, a person reads the audit. No AI runs in production.
