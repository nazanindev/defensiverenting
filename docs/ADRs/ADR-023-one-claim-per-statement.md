# ADR-023 — One claim per statement: a word cap and split proposals

| | |
|---|---|
| Status | Accepted and shipped 2026-09-21 |
| Date | 2026-09-21 |
| Amends | ADR-003 (statements are atomic: now enforced), ADR-014 (a proposal may replace one statement with several), ADR-022 (the reading loop) |

## Context

The reading loop of ADR-022 ran over 305 draft statements on 2026-09-21. It rewrote about 150 of them. Its rule for a reader was "a claim must carry its conditions" and its rule for a judge was "not worse: do not drop a condition a renter needs". Both are right. Together they only add sentences. Rewritten statements averaged 94 words against 61 for the untouched, 29 ran past 120, and the longest reached 263: a rule, four provisos, a rebuttal, and a fee clause in one block. True, cited, and not a statement.

ADR-003 says a statement is one atomic claim. Nothing enforced it. The drafting prompt asked for 10 to 14 statements per page, which pushed the drafter to pack a topic into few rows, and the loop then packed the conditions into the same rows.

## Decision

**A statement body is at most 90 words.** `voice.MaxStatementWords`, checked wherever a statement body is linted: the drafting save, `triage check`, and the judge's apply rule. A rule, its exceptions, and its remedy are three claims and read as three statements, each with its own citation and its own stamp.

**A proposal may split a statement.** `ProposedStatement.Followers` lists statements inserted right after the replaced one when the proposal is applied. The replaced statement keeps its key, so its notes and its history stay with it; followers are new statements with new keys and start unreviewed, so the reading loop reads them. The queue shows followers under the proposed body. `triage check` and the judge hold every follower to the same rule as the replacement: voice lint, a quote on every citation, every quote confirmed live.

**The drafter's count follows the law.** The prompt now asks for 12 to 20 statements per page, one claim each under 90 words, the whole subsection quoted, and the condition stated when the law has one. The count was a proxy for depth; the cap is the real constraint.

**Existing long statements are split by the loop**, not trimmed. The conditions the readers added were missing for a reason; they move to their own rows rather than back out.

## Consequences

- Pages get longer as lists and shorter as paragraphs. That is the shape the site is meant to have (ADR-006, "lists, not boxes").
- A statement that still exceeds the cap after the split pass is a sign the claim itself is not atomic, and a person looks at it.
- Every follower is a new stamp to earn. The reading loop absorbs that.

## Rejected

- **Trimming conditions to fit.** Puts back the overstatements the readers caught.
- **A per-page cap on statements.** Wrong axis; the count should follow the law.
- **A soft guideline instead of a lint rule.** The loop showed that without a hard cap every fix adds and nothing removes.
