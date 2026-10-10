# ADR-033 — A citation says which sentences it backs

| | |
|---|---|
| Status | Proposed 2026-10-09. Page changes deferred. |
| Date | 2026-10-09 |
| Amends | ADR-003 (a citation gains sentence positions), ADR-021 (one more rule an agent decides by), ADR-032 D2 (the source line gains sentence markers when this ships) |

## Context

Nazanin, 2026-10-09, on the redesign mockup: "I do like the idea of sentences being cited instead of entire statements."

Today a citation backs a whole statement. The stored verbatim quote is checked against the source every week (ADR-014, check receipts), and the review loop stamps the statement when the quote supports it. Under that cover, a statement often carries sentences no source backs. On the Pennsylvania deposit guide: the worked examples with the $1,200 rent, "ask in writing for the extra money back", "you get this money only if you win your case and your landlord pays". The page says the statement is sourced. It does not say which of its sentences are.

Saying so is the honest version of "database plus language rigor" (ADR-030): a competitor verifies figures, we verify sentences. It also surfaces what should become a tip or go.

## Decision

### D1. The statement stays the unit

Statements keep their keys, stamps, notes, queue items, concept reuse and the 120-word plainness cap. Nothing is split. A citation gains a list of sentence positions within its statement.

### D2. A citation carries sentence positions

`citations.sentences` is an array of 1-based sentence indexes into the statement body, sentence-split by the same splitter the queue's sentence-snapped suggestions use. Empty means not yet decided, which renders exactly as today. A statement edit that changes the sentence count clears the positions on its citations, so a stale marker never points at the wrong sentence. The checker is unchanged: it compares the stored quote to the live source, nothing more.

### D3. An agent decides the positions by a stated rule

A new review rule under ADR-021: given the statement body and the stored quote only, never the source page, say which sentences the quote supports. The rule follows [[feedback-pass-means-stored-quote]]: a sentence is backed when the quote states what the sentence states, in the quote's words or plainer ones. A sentence that goes beyond the quote is not backed, however true it is. Decisions are filed as proposals and applied through the one ApplyProposal path, with the stamp kept.

### D4. Conventions for what falls out

Three kinds of unbacked sentence will appear. The pilot settles them:

- **A worked example** ("if your rent is $1,200, the most is $2,400") inherits the positions of the sentence it illustrates, when it adds no new rule. The agent marks it so.
- **Advice** ("ask in writing for the extra money back") gets the editorial citation on that sentence, or moves into a tip under the statement (ADR-016 A2). The agent proposes, a person picks.
- **Filler** is proposed for removal under the existing edit rule.

### D5. The page, when it ships

A small superscript number after each backed sentence, matching the numbered source line of ADR-032 D2. A sentence with no number is visibly ours. One statement with one source whose quote backs every sentence shows no markers, since the source line already says it. Deferred until the pilot has run and Nazanin has seen how many unmarked sentences a page carries.

### D6. Pilot

One state, chosen when the pilot starts. Measure: how many sentences are unbacked, which of D4's three kinds they fall into, and how the pages read with markers. Decide site-wide after that.

## Consequences

- Readers see which sentence a source backs, and which are ours. Some pages will look thinner. That is the point.
- One migration, one review rule, one proposal kind. The sentence splitter already exists.
- Until D5 ships nothing changes on the site. The positions accumulate in the background.

## Rejected

- **One statement per sentence.** The cleanest model and the wrong move: every tool, stamp and page is per statement. It would redo the review loop.
- **Markers with no data, one source marking every sentence.** Claims more than we checked.
