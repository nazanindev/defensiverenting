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

## Amendment, 2026-09-24: 120 words, with a readability floor

The 90-word cap pushed agents to fit statements under the line by reaching for denser, harder words, the opposite of the voice rules. The cap is now 120 words, and readability rules in code make that trade fail instead of relying on the prompt. A first version counted syllables (Flesch-Kincaid, grade 10); it was replaced the same day because what stops a renter is an unfamiliar word, not a long one ("utility" is short and opaque, "refrigerator" is long and plain).

- **Unfamiliar words.** A word outside everyday English fails unless it is replaced or glossed in parentheses right after it (`voice.UnfamiliarWords`). Everyday English is every word with a Zipf frequency of 3.5 or more in wordfreq 3.1.1 (CC BY-SA 4.0 data, `internal/voice/wordlists/familiar_en.txt`), plus a short list of words renters meet every day (landlord, lease, eviction, deposit, hotline). Capitalized names and web addresses are skipped.
- **Utilities** always need a gloss (an explain rule): common in general English, opaque in the housing sense.
- **Reading score.** A statement with a New Dale-Chall score of 6.5 or more fails (`voice.MaxStatementScore`), which keeps statements near a grade 7 reader. The score uses the Dale-Chall list plus very common English words (Zipf 5.0 or more, `wordlists/common_en.txt`), because the 1940s list misses "within", "problem" and "local". Measured before shipping: median 5.4, 11% at 6.5 or above; 460 statements had a word outside everyday English.
- **No harder edits.** In `triage check` and the judge's apply rule, a replacement may not add an unfamiliar word the current body lacks, and may not raise the score by more than 0.5 once it reaches 6.0 (`voice.HarderThan`).

Existing statements that break the new rules are not blocked; the lint runs on saves and edits, and the loop fixes them as it touches them.
