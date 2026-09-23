# ADR-025 — A page is reviewed as a page

| | |
|---|---|
| Status | Accepted 2026-09-22 (Nazanin). Step 1 (D1-D3, drafts only) shipped the same day: `triage decide page`. D4 verbs shipped the same day; on drafts they are applied by agents (D5 amended below). D6 outline in the command follows. |
| Date | 2026-09-22 |
| Amends | ADR-014 (new proposal kinds), ADR-021 (a new rung on the ladder), ADR-022 (PASS readers see the page outline) |

## Context

Every agent in the review loop reads one statement at a time: the reader passes it or leaves a note, the triage agent proposes a replacement, a judge applies it. That loop now checks each claim against its stored quote well. It cannot see the page.

Nazanin's flags on 2026-09-22 were all page-level defects that the loop had passed:

- Pittsburgh Landlord Entry 9 is a utility-shutoff statement. True, quoted, stamped, and on the wrong page.
- New York Repairs 24 and 25 restate the retaliation exceptions that statement 23 already gives. Part of that came from the loop itself: splitting a statement to stay under the 90-word cap (ADR-023) produced a follower that repeated a neighbour nobody in the loop could see.
- The Pittsburgh Local Help reader could not check for duplicate entries, because it was handed only the entries it was stamping.

A renter reads the page top to bottom. Duplicates, an off-topic claim, a risk that comes before the rule it qualifies, two statements that disagree, or a missing step are defects in what they read, even when every statement is individually true.

The queue also has no way to act on these. A proposal replaces one statement's text. It cannot remove a statement, merge two, move one, or add one.

## Decision

### D1. A page reviewer reads the page skeleton, not the sources

When every statement on a draft page is stamped and nothing is pending, a page reviewer reads it once. It is given the page's title and intro and, for each statement, its position, body text and concept tag. It is not given quotes, citations or source pages.

That is deliberate. Everything it checks is visible in the text, and the quote work is already done by the statement loop. A smaller context keeps its judgement on the page and its cost per page low.

### D2. What it checks

1. **Duplicates and overlap:** two statements making the same claim, including a follower that repeats a neighbour.
2. **Relevance:** each statement answers the page's title question, or belongs on another topic (then the fix is a one-line pointer with `topic_ref`, not the claim).
3. **Order:** the sequence a renter acts in. What is true, what to do first, the risks, where to get help.
4. **Contradictions:** two statements that disagree, including state and city scope drifting across the page.
5. **Gaps:** a claim the matching state page or the concept registry says a renter needs, missing here. A gap is reported with the concept slug, never written.
6. **Title and intro:** they match what the page now says.

### D3. It starts advisory

The reviewer files one page note per finding, naming the statement keys involved and the kind (duplicate, off-topic, order, contradiction, gap, intro). Nazanin decides each. Her overturns of its findings are the measure, as with every rule under ADR-021.

A page with an open page note does not publish, the same as a statement note (ADR-013 gate).

### D4. Three new proposal kinds

A proposal can now:

- **remove** a statement from a page, with the reason. Invariant: it cannot remove the only statement on the page carrying a concept the page's topic requires, and cannot remove a statement whose key another page links to by anchor, without saying so.
- **merge** two statements into one. Invariant: the merged statement keeps every citation and quote of both (the stored-quote rule then holds by construction), and keeps one of the two keys; the other key is retired with a pointer to the kept one.
- **reorder** a page: a full new order of the page's keys. Invariant: the set of keys is unchanged.

Adding a statement already exists: a replacement proposal with followers.

Each applies through the one approval path (`drafting.ApplyProposal`), under the name of whoever decides.

### D5. The ladder

1. Advisory page notes; person decides (D3). No agent applies a page-level change.
2. Remove, merge and reorder proposals, filed by the triage agent in answer to a page note, decided by a person.
3. A judge (never the proposer) may apply duplicate-merges and reorders on draft pages, once the advisory audit shows few overturns. Removals and live pages stay with a person.

### D6. Statement readers see a page outline

PASS readers and judges also get a one-line outline of the rest of the page: position and first sentence of each statement. Not the full statements and not their quotes. That is enough to notice "this repeats statement 23" at the statement level, which is where the duplicate follower of 2026-09-22 would have been caught.

## Consequences

- One more read per page, on the page skeleton only. At roughly 15 statements a page this is a small context.
- Duplicates created by splits are caught before a person reads the page.
- The proposal table gains three kinds; the queue page renders a remove as the statement struck through, a merge as the two statements beside the merged text, and a reorder as the old and new order side by side.
- Redraft agents (the city pages) are told: state each rule once per page.

## Answers to the open questions (2026-09-22)

1. **Drafts first.** Nazanin: "start with drafts". Published pages wait until the overturn rate on page notes is known.
2. **Gap is in step 1, computed, not read.** The command lists the concept tags the page one level up carries and this page does not (`store.ConceptGaps`), so the reviewer gets a short list of slugs and never reads the other page. On the first run (Pittsburgh vs Pennsylvania) the list was short and mixed real gaps (no mediation or eviction-record statement on Can't Pay Rent) with tagging misses (quiet enjoyment covered but untagged), which is exactly the judgement the reviewer is there for. A gap finding names the slug; the reviewer never writes the missing claim.

## Step 1 as built

A finding `{playbook_id, kind, keys, note}` is filed as an ordinary reviewer note on the first key it names, prefixed `Page review (<kind>):` and listing the positions involved, under the review agent. It reuses the note machinery on purpose: it shows on the statement card, it holds the page from publishing until decided, and it needs no new table. Refused: unknown kinds, empty notes, keys not on the page, and any page that is not a draft.

## Amendment: agents apply page-level changes on drafts (2026-09-22)

Nazanin, after the first run: "I actually think the page agent should be able to make changes like the other agents, not just proposals." So D5 steps 1 and 2 are skipped for drafts and step 3 applies at once, with removals included:

- The page reviewer files a finding (a note); the triage agent answers it with a remove, merge or reorder proposal (`action` on the proposed statement); a different agent judges it and applies it through `triage decide edit`, under the review agent. The proposer is never the judge.
- Drafts only. `ApproveProposal` refuses a page-level change on a published page unless a person approves it. No queue screen for page-level proposals is built yet; until it is, none are filed on published pages.
- Invariants in the store: a page keeps at least one statement; a merge keeps every citation of both statements and the first statement's key; a reorder keeps the same set of statements. Open items on a statement that leaves the page close with it, pointing at the proposal.
- Overturns stay the measure: a person who flags a page-level change undoes it by hand and the flag records it.
