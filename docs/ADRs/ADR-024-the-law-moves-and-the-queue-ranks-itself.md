# ADR-024 — When the law moves under a statement, and what the queue shows first

| | |
|---|---|
| Status | Accepted and shipped 2026-09-22 |
| Date | 2026-09-22 |
| Amends | ADR-014 (the queue has an order), ADR-018 D1 (a person can file a note), ADR-021 (overturns are now countable) |

## Context

Two gaps, both found by using the system rather than by reading it.

**The reader had no verb for doubt.** Reading a page as a renter is the person's job (ADR-022). Their only actions were Done, edit, and publish, so noticing that something looks wrong meant either editing it themselves or letting it go. ADR-021 named overturns as the measure of whether an agent's rule can be trusted, and then gave nobody a way to record one.

**Only a vanished quote is watched.** The checker re-fetches every source and fires when a stored quote no longer appears. That catches a figure being changed. It misses an amendment that adds an exception and leaves every stored quote intact, which is precisely the defect the reading loop kept finding by hand; it misses a section being renumbered; and it misses a date passing, which no fetch can ever detect because nothing at the source changes.

Meanwhile the queue is one list in page order (ADR-014, the 2026-09-20 rework). With a live page's wrong claim sitting behind thirty draft-page quote widenings, order is not a detail.

## Decision

### D1. A reader flags a statement, with the reason

The statement card carries one more control beside edit: a reason and a Flag button. It files a reviewer note under the reader's name, which the triage agent answers like any other note, and the open item stops the page publishing until it is decided. A flag with no reason is refused; an agent cannot flag, because a flag is a person's doubt.

When the statement carried the review agent's stamp, the note records `overturned: "review agent"`. That is the overturn ADR-021 asked for and could not collect. A rule whose passes a person keeps flagging is a rule that narrows.

### D2. The checker watches the passage, not only the quote

Every citation already stores the passage its quote sat in at the last check. On a re-check where the quote survives, the new passage is compared to it, folding typography and whitespace the way the verbatim matcher does. When they differ, the law around the quote moved: a note is filed on the statement and the loop re-reads it. The comparison is skipped when there is no baseline, when the page text is byte-identical, or when a different extractor family read the page, since those differences are not the law's.

### D3. A dated claim carries its date

A statement whose claim depends on a date holds it in `stale_after`: a sunset, a cap set for one calendar year, a program with an end date. A full checker run files a note on every such statement within 30 days of that date. Ahead of it, not after: the point is that the fix is in the queue while the page is still right.

### D4. One path for all of it

A flag, a moved passage and a coming date all file the same thing, a reviewer note on the statement, and all run the same loop: the triage agent proposes, a judge applies, a reader re-reads. No alarm, no second channel, no special case for a live page. A live page needs none, because an edit to a published page is a person's decision already (ADR-021), so the fix arrives in their queue by itself instead of being applied behind them.

### D5. The queue ranks itself, and nobody sets a priority

The list stays one list with one item open. Its order is computed from what the queue already knows:

| Rank | | |
|---|---|---|
| 1 | live page, claim disputed | a person's flag or a checker note on a published page |
| 2 | live page | anything else a renter is reading now |
| 3 | last item on this page | the only thing between a draft and its Publish click |
| 4 | the rest | |

Each card carries the phrase for where it sits. No item carries a priority anyone typed: a queue where a filer can mark their own item urgent stops ranking anything, and every input above is a fact rather than a feeling.

## Consequences

- The overturn rate becomes a number instead of an intention, which is what decides whether ADR-022's stamp keeps its place in the gate.
- Amendments stop being invisible. The defect class the reading loop found by hand now finds itself on the next check run.
- `stale_after` is only as good as what fills it. The drafting and triage agents set it when a body states a date; a statement that states one and carries no date is worth a lint, which this ADR does not yet add.
- Rank 3 makes the queue finish pages rather than spread across them.

## Rejected

- **A manual urgent flag on proposals.** Everything becomes urgent. The ranking is derived instead.
- **Kind tabs or a second list for urgent work.** The one-list queue was deliberate; ordering is enough.
- **Taking a live page down when a claim goes stale.** A page slightly behind beats no page, and the decision is a person's. Firing the note early is what makes that safe.
