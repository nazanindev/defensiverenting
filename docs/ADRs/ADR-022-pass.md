# ADR-022 — A PASS from the review agent counts as review

| | |
|---|---|
| Status | Accepted and shipped 2026-09-21 |
| Date | 2026-09-21 |
| Amends | ADR-018 D2 and D3 (a second kind of stamp), ADR-021 (its "Rejected" note on borrowing the stamp is withdrawn) |

## Context

ADR-018 made review per statement: a person reads each sentence with its quotes and stamps it. That stamp is the publish gate. With fifty states in scope, about four thousand statements are on their way, every one with its quotes already confirmed verbatim by the checker. The queue work of ADR-021 removed what blocked those stamps; it did not remove the stamps. One person cannot supply four thousand of them in the time that matters.

The principle Nazanin set on 2026-09-21: minimise the human labour per page to what only a person can do. That is two things. Decide what the agent said it could not decide. Put the page live. Everything else is the checker's or the agent's.

The agent can already do the reading. Rule flag (ADR-021) has run clean: a reader in a local session, a verbatim passage on record, a command that refuses any answer whose passage is not at the live source. A PASS is that same act, applied to a statement nobody has doubted yet.

## Decision

**A PASS is a flag-shaped answer to the question the stamp asks.** The reader reads the statement against its cited sources as fetched now. It passes when a passage in a cited source supports the claim as written and nothing nearby narrows or contradicts it. It records the source, the passage, and one sentence of reason. The command (`triage decide pass`) refuses a PASS whose source the statement does not cite, whose passage is over 150 words, or whose passage is not found verbatim at the live source, and leaves the statement for a person with the reason printed. A source this machine cannot read leaves it too.

**A PASS is the review stamp, under the agent's name.** `PassStatement` writes the ordinary stamp, bound to the content hash, with `reviewed_by = "review agent"`. An edit clears it the way it clears a person's. The card shows who stamped. The standing rule, the dashboard, and the gate are unchanged: a stamped statement is a reviewed statement. The PASS itself is filed as an already-approved proposal (`agent-pass:pass`) with the evidence, so `triage decide audit` lists every one. A statement with an open queue item cannot be passed; the item is decided first.

**Publish stays a person's click, and that is all it is.** No page-read step, no sampling. A person who wants to read the page before publishing may. The design does not require it, because nothing a page read catches is something the checker, the readers, or a later flag would not.

**What comes to a person is what the reader left**, with the reason, on the statements screen and in the queue. That is the necessary labour, and it is the only labour.

**What a reader leaves is filed, not printed (amendment, 2026-09-21).** A refused PASS or a held edit files the reader's reason as a reviewer note on the statement, under the review agent's name, one per distinct reason. That note is the triage agent's work: `triage decide work` lists it with the statement and its sources, the triage agent proposes a fix, a separate judge applies it, and the statement goes back through PASS. Rule flag does not read the review agent's own notes, so nothing bounces. What reaches a person is what survives that loop: the triage agent skips it as editorial, or a judge holds the fix again.

**Overturns remain the measure.** An edit to a passed statement, a flag filed on one, or a page taken down are the signals. If they arrive, the rule narrows. The revert is one line: the gate counts only stamps whose `reviewed_by` is a person.

## Consequences

- Per state, the person's work is the leftovers and seven Publish clicks.
- The public trust line can say what is true: every quote confirmed at the source, every sentence read against its source by a second reader with the passage on record, every change decided by a named actor with the rule on record, every page put live by a person.
- ADR-021's note that the stamp column must not be borrowed is withdrawn. The column records who; a PASS says who.

## Rejected

- **A sampling read of passed statements before publish.** Proposed as a way to feed the overturn count. Rejected as an extra step: the agent's judgement is the judgement, and the feedback loop that finds a wrong pass (readers, checks, flags) exists already.
- **A separate page-read stamp before Publish.** More buttons for a distinction the Publish click already makes.
- **PASS as information only.** Changes nothing about the month.
