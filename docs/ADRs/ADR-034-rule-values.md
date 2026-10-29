# ADR-034 — A number lives once, in a typed row, and the prose is checked against it

| | |
|---|---|
| Status | Proposed 2026-10-29. Not built. |
| Date | 2026-10-29 |
| Amends | ADR-027 (one more quality check), ADR-020 (the concept-page table gets its data source), ADR-028 D5 (coverage records stay separate and are read beside the new rows) |

## Context

Nazanin, 2026-10-29: "I want numbers to be stored in the database in a clean way. Like, say we have every state's rule for rent increase notice. I want the displayed number to be derived from an actual number in the database. That way as things change there is a built-in check, and we can do clear and easy analysis."

Today a number such as a rent-increase notice period exists only inside prose: a statement body tagged to the concept `increase-notice-period` for one place. Nothing in the database knows that Ohio's number is 30 and California's is 30 or 90. So:

- Two statements on different pages can carry different numbers for the same rule and nothing notices.
- A source drifts, the checker flags the quote, and a person has to find every sentence that repeats the number.
- "Which states give less than 30 days?" is a reading task across fifty pages, not a query.
- The 50-state comparison table that ADR-020 deferred has no data to draw from.

The grid the numbers belong in already exists: concepts, statements tagged to a concept per place, and coverage records for "we searched and found no law" (ADR-028 D5). What is missing is one table.

## Decision

### D1. One row per place, concept and condition

A new table `rule_values`:

| Column | Meaning |
|---|---|
| jurisdiction_id, concept_id | The place and the question, as everywhere else |
| condition | Plain words for when this value applies: "month-to-month", "increase over 10 percent", "lease of a year or more". Empty when the rule has one value. Part of the unique key. |
| kind | `days`, `dollars`, `months_of_rent`, `percent`, `count` |
| value | The number |
| value_max | Optional. Set when the law gives a range or a tiered rule reads better as one row ("30 to 60 days"). NULL otherwise. |
| source_id, locator, quote | The same citation shape statements carry. The quote is verbatim and non-empty. |
| checked_by, checked_at | The same stamps every claim carries |
| note | Optional, for the reviewer: why this condition, what the statute calls it |

Unique on (jurisdiction_id, concept_id, condition). A concept with two conditions in one state is two rows. Many places have one row per concept.

### D2. The value row carries its own citation

The number is a claim about the law. Every claim on the site is backed by a verbatim quote the weekly checker re-reads (ADR-014, check receipts). A pointer to a statement would break the moment that statement is rewritten, merged or retired, which the review loop does often, and a comparison table that cites a statement id is not citable. The row cites the section and quote directly. The citation is usually the one the statement already has, so the backfill copies it rather than researching it twice.

The checker treats a rule value's citation like any other: same fetch, same receipt, same drift finding, filed as a proposal. The source-quality checks of ADR-027 D3 apply to it as to a statement.

### D3. "No law found" stays in coverage records

ADR-028 drew the line on purpose: a coverage record is a dated, signed fact about our search, with no quote because there is nothing to quote. A rule value of kind "none" would carry an empty citation and weaken D2 silently. The two stay in two tables. A reader of the grid takes the rule value where one exists, else the coverage record, the same lookup `RulesAnswers` already does for rules pages.

### D4. Prose is checked, not generated

Statements keep their hand-written bodies. The site does not build sentences from the table. Generated sentences would fight the plain-sentence voice, the per-sentence citations of ADR-033, the 120-word cap and the readability lints, and every tool, stamp and note is per statement.

Instead a new quality check under ADR-027: for every statement tagged to a concept that has rule values in its place, extract the numbers in the body (digits and number words, with their unit words: days, dollars, months, percent) and compare them to the rows for that place and concept. The check fails when the body carries a number of that kind that no row has, or when the rows carry a value the body does not mention and the statement's condition words match. A statement that mentions no number at all passes; the check is about disagreement, not completeness.

The check runs where the other ADR-027 checks run: the quality map cell, the page drill-down, the publish gate under the live no-worse rule. Its details name the statement positions and the two numbers.

### D5. Concept pages and analysis read the table

The 50-state table on a concept page (ADR-020, deferred) is one query over `rule_values` joined with coverage records: one line per state, the value in plain words, the condition when there is one, the source line per cell. The table is a projection of the rows, as a concept page is a projection of statements (ADR-012). The query is the analysis surface too; "states with fewer than 30 days" is a `WHERE`.

Rendering the number in words follows site copy: "30 days", "two months' rent", "no limit" never appears because no-limit is not a row. A deferred detail: the condition column reads as the question's own words, not the statute's.

### D6. A change reaches the number and the prose together

When a source drifts, the checker files one drift finding on the citation. The rule value and the statement both carry that citation, so one finding lists both. The review agent (ADR-021) proposes the new value and the edited sentence as one proposal. ApplyProposal refuses to apply a value change while the D4 check would fail on a published statement in that place, and refuses a prose change that would disagree with an unchanged row. A person can still override by editing the row first; the gate protects against half-applied changes, not against people.

### D7. Backfill and pilot

The pilot is `increase-notice-period` for every state. An agent reads the stamped statements tagged to that concept per place, writes one row per condition it finds, copies the citation, and stamps the row as agent-filed. A person spot-checks, then the D4 check runs and lists every place where prose and row disagree. Those disagreements are the first real finding and decide whether the extraction rules in D4 are good enough.

After the pilot, the numeric concepts in order of how often renters search them: `entry-notice-period`, `deposit-cap`, `deposit-return-deadline`, `grace-period`, `late-fees`, then notice periods for non-renewal and pay-or-quit, deposit interest and the small-claims limit as concepts exist for them. About fifteen concepts in all.

## Consequences

- One migration, one quality check, one proposal kind, one query for concept pages. No change to statements, pages or the review loop.
- Data entry grows by one row per place, concept and condition. The backfill is agent work; verification is the same stamp as a statement.
- The first run of the D4 check will fail cells that are fine today. That is the check doing its job; the quality map shows them, and the disagreements are the work list.
- Analysis becomes SQL. The comparison table that ADR-020 deferred can ship once the pilot concept is full.

## Rejected

- **Generate the sentence from the row.** The cleanest derivation and the wrong move for a site whose unit is a plain, cited, stamped sentence. The check gives the invariant without the mail merge.
- **The row points at a statement instead of carrying a citation.** Less entry, but the pointer dies with every rewrite and the table cannot cite its own cells.
- **"No law" as a value of kind none.** One table for the query, and a hole in the rule that every row has a quote.
- **A JSON column on the statement.** Puts the number back inside the thing it is meant to check, and makes the cross-page query a JSON scan.
