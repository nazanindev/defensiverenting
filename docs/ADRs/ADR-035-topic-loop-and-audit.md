# ADR-035 — A topic is drafted from one spec and is done when one audit is clean

| | |
|---|---|
| Status | Accepted 2026-11-11. Built so far: `triage audit` as a report (D2, the computed checks of D3) core questions (D7), the tip checks (D8) and the checks against the stored topic shape (D9). |
| Date | 2026-11-11 |
| Amends | ADR-027 (the computed checks gate new topics instead of only reporting), ADR-028 (the topic record carries the whole spec), ADR-016 (which notes a topic needs is stated on the topic), ADR-021 and ADR-025 (the agent's read rules are listed in one place) |

## Context

Nazanin, 2026-11-11: "We need to consolidate the review loop I have been piecing together topic by topic. With the pipeline as is, the process will be: I run Claude on a loop for a new topic, across 50 states, edit in bulk with Claude, publish. If we codify this workflow, we can prevent all the repeated work I keep doing. We need some more robust rules, like verifying every state has the proper warnings, and that statements are actionable."

The loop works. It took over 400 pages from draft to verified. The repeated work has four causes:

- **Rules arrive after the drafts.** Warnings as notes (ADR-016 A12), the renter-use rule, common case first, and the plain layout were each decided after 50 pages of a topic existed. Each became a 50-state sweep: the advice backfill, the stage backfill, the trim pass, the plain rewrite.
- **Most rules live only in prompts.** `common-rules.md` in the loop skill is a dated log of corrections. Some entries now contradict later ones: it still approves a risk sentence inside a statement, which A12 bans. An agent that reads the file top to bottom gets both.
- **The quality checks only report.** `risks-paired`, `first-step`, `deadline-number`, `somewhere-to-go` and the source tiers are computed (ADR-027) and shown on `/quality`. None of them stops a draft from being saved or published.
- **Nothing answers "is this topic done in all 50 states?"** `decide pass`, `decide work` and `decide edit` are read page by page. A state with no page and no hold is invisible.

What a topic needs is also spread out: stages and `plain_layout` on the topic row, concepts in the topic map, warning notes in the advice registry, caps in Go constants, the intro and title patterns in briefs.

## Decision

### D1. One spec per topic

The topic record carries everything a drafter, the lint, the audit and the publish gate need. Columns that exist stay (`stages`, `plain_layout`, `rules_for`, `is_core`). The spec adds:

| Field | Meaning |
|---|---|
| required concepts | The concepts every place must answer, or carry a no-law record for (ADR-028 D5) |
| required notes | The advice entries a page of this topic must carry when a statement names the matching step (ADR-016), and the disclaimer every page of the topic opens with, if any |
| caps | Most statements and most words per page. Today these are constants that apply only to plain-layout topics. |
| title pattern, intro pattern | The question title (2026-09-08) and what the intro must open with |
| shared statements | Site-voice statements that are the same on every page of the topic (the CFPB collector and pay statements) |
| pilot places | Three places for D4 |

The spec is data in the database, set by migration or by a `triage spec` file, never prose in a brief. The drafter brief is printed from it (`triage spec <topic> -brief`), so a brief cannot disagree with the gate.

### D2. One audit, and it is the definition of done

`triage audit <topic>` prints one grid: a row per place, a column per check, a cell that is clean or names the first failure. It exits non-zero when any cell fails. A topic is done when the grid is clean. Nothing else decides it.

The audit adds no new queue and files nothing. It reads what exists: page issues (`issues.go`), quality checks (`qualityrules.go`), the voice lint, stamps, notes, flags, proposals and coverage records.

The same computed checks run in three places, from one function:

- **At save**, so a drafter is refused at once and fixes its own draft. ADR-013 holds: a person's save in the authoring app still never blocks. The refusal applies to agent saves through `save_draft_playbook` and to `triage check`.
- **In the loop**, as the audit grid.
- **At publish**, as gate issues (D5).

### D3. The checks

**Computed**

| Group | Check |
|---|---|
| Coverage | Every place has a page, an open page flag saying why not, or no-law records for every required concept |
| Warnings | Every statement that names a risky step has its note on the page, under the first statement that needs it |
| | No warning sentence sits inside a statement (A12) |
| | No note sits on a page with no statement that needs it |
| | The same step carries the same note in every place |
| | The topic disclaimer opens the intro, word for word |
| Actionable | The page has a first step and a place to get help (ADR-027 D4.2, D4.4) |
| | Every deadline says who gets the days, from when, and what happens |
| | A statement with no step, number, right, limit or help place is a renter-use candidate. This is a hint: the reader decides (below). |
| Sources | A legal rule cites law or a court ruling; no archive link; a statute has a section; every quote confirmed live in the last 7 days |
| Shape | Caps; stage order; every statement staged; one statement per concept on a rules page; question title; no place name and no section number in a statement |
| Voice | The save lint, unchanged |
| Outliers | A page far from the topic's median length |
| | A required concept most places answer and this place skips with no record |
| | A shared statement that differs from the spec's text |
| Finished | Every statement stamped against its current quote; no open note, page flag or pending proposal |

Outliers are a hint for a person. They show in the grid and never block.

**Read by the review agent, by stated rule (ADR-021)**

1. The stored quote backs every sentence (ADR-022).
2. Renter use: the statement gives a step, a right, a limit, a number, a place to get help, or a consequence (2026-10-31).
3. Common case first within each stage; exceptions last (2026-10-06).
4. One of the first two statements answers the title question (ADR-027 D4.1).
5. Statements that repeat a rule are merged, keeping every condition and citation.

A read rule cannot be audited by code. The audit shows instead whether each page has been read under the current rules: a stamp records the rule set it was made under, and a stamp from an older set shows as "read again".

### D4. Three places before the other 48

A new topic is drafted for the spec's three pilot places first: one with detailed law, one with thin law, one whose code has no free official copy. The loop runs on them until the audit is clean. Then it stops for Nazanin.

She reads the three pages once. Every change she asks for lands as a spec field, a check, or a line in the rules file (D6) before any other place is drafted. The other 48 are then drafted against the corrected spec.

This is the step that removes the sweeps. A rule decided on 3 pages costs 3 rewrites. The same rule decided on 51 costs 51.

### D5. What blocks

- **New topics** (drafted after this ADR): every computed check except outliers is a publish-gate issue.
- **Existing pages**: the checks stay a report. 482 of 590 pages fail at least one quality check today (2026-10-11), and a gate would freeze them. The no-worse rule for live saves (9afc1cf) applies as now.
- **Bulk edits** go through the queue as proposals, as now. `triage check` runs the audit's checks on each proposal, so a bulk edit cannot add a failure.

### D6. One skill, one rules file

- `state-review-loop` becomes `topic-loop`. Its steps: spec, pilot, stop, draft the rest two at a time, read, fix, judge, audit, bulk edit, publish.
- `common-rules.md` is rewritten as the rules in force today, grouped by the D3 table, with no dates and no corrections. History lives in the ADRs and in git.
- A rule is not learned until the audit can fail on it. A new rule from Nazanin lands in the same commit as its check or spec field. A rule only an agent can read gets a line under "Read" and bumps the rule set, so D3 shows which pages need reading again.
- Per-state source quirks stay in `state-brief.md` and `no-free-code-states.md`. They are about places, not topics.

### D7. Core questions, and a research step for the gaps (2026-11-11)

The first audit of the 62 eviction drafts showed the warnings in place and the pages still not usable: 25 had no statement telling the renter to do anything, and each state covered a different slice of the process. Nazanin, 2026-11-11, on letting a rework add a few statements where a stage was empty: "We probably need a bigger fix for this one, like rules for the drafting agent, or letting the review loop find more research if stuff is missing."

- **A topic names its core questions.** The topic's must-answer rows in `topic_questions` (ADR-036 D4, migration 000093) name the concepts every page of the topic answers. This is D1's "required concepts". The first build kept them as a `core` flag on the concept. That flag was dropped before it shipped, so there is one list, and a question can be must-answer on two topics. Eviction has 8: the late-rent notice, the notice before court, paying to stop the case, answering court papers, what happens in court, missing the court date, time to move after losing, and appeal. A free lawyer, things left behind and eviction records stay optional: many states have no law on them.
- **The drafter drafts to that list.** One statement per core question, in stage order, each opening with what the renter does when the quote supports a step. The caps leave room for 4 more statements: a defense, a local rule.
- **Coverage is a column of the audit.** Each core question on each page is answered (a statement tagged with the concept), recorded as "searched, no law found" (ADR-028 D5), or a gap. It is the check rules pages have had since ADR-028, applied to situation pages. On the eviction drafts it found 167 gaps; 48 of 62 pages did not say what happens when a renter misses the court date.
- **The loop researches a gap instead of leaving it.** The agent first looks for an untagged or mistagged statement on the page that already answers the question. If there is none, it searches the sources the page already cites, then the court system's self-help pages, then the legislature's site, within a fixed budget (30 tool calls a page). It adds one statement with a quote copied from the live source, or reports the URLs it searched. A new statement is read against its quote like any other. A miss becomes a no-law record once a second agent confirms it.
- **Rework and research are one pass per page.** The agent cuts and merges what is there, then fills the gaps, so a page is opened once.

### D8. Tips are placed by rule and read like statements (2026-11-11)

Nazanin, 2026-11-11, on the pilot: "I am more concerned with how the tips are being placed and if they are right. The agents can reason with the rules about the shape fine."

A read of the 62 eviction drafts showed 51 pages with no tip or note, the only 2 tips under the wrong statement, and the award warning in three forms. The audit had passed all of it, because it checked only that some warning existed.

- **Each tip names the statement it may sit under.** The court papers tip sits on every eviction page, under the statement that answers "What do I do when I get eviction court papers?". Where a page has no such statement, it sits under the first statement that tells the renter to answer or go to court. It never sits under the statement on missing the court date, because that statement already says it. The receipt tip sits under the first statement that tells the renter to pay the landlord, never under paying into court. The plan-in-writing tip sits only where the landlord may agree to something. The keep-a-record tip sits only where the renter writes or complains to the landlord, never under a court filing.
- **A statement can carry more than one tip, when each one fits it.** The first version of this decision allowed one tip per statement. Nazanin reversed it on 2026-11-12 when she approved the eviction topic shape: 69 statements on prod already carried more than one tip, and the pages were fine.
- **The audit fails on a missing court papers tip, and on a tip under a statement that does not name its step** (`tip-missing`, `tip-misplaced`). A court papers tip under the missed-court statement is misplaced. It also fails when a statement says a court can make the landlord pay and the page carries no award warning.
- **A reader checks each tip against the statement above it.** This is read rule 6.
- **A tip names no step the law of some state contradicts.** The court papers tip said "Go to court", which is wrong where the renter must file a written answer first. It now says "If you do not respond by the date on the papers, you can lose the case." (migration 000094).
- **No inspector note on an eviction page.** The retaliation defense describes a report already made. It does not tell the renter to call an inspector.
- **The award warning stays a sentence in the statement only where the place has no backing quote for the note.** Everywhere else it is the note.

### D9. The audit reads the stored topic shape (2026-11-12)

ADR-036 D4 stores a topic's shape in four parts. The audit reads them instead of constants:

- **Must-answer questions** come from `topic_questions` (role must). A must answer under the wrong stage fails (`question-stage`).
- **Title** is checked against the topic's title pattern (`title-pattern`).
- **Tips and notes**: one the topic marks for every page is missing (`tip-missing`); one the topic never uses is on the page (`advice-not-used`). Where a tip sits stays the tip's own rule (D8).
- **Size**: the statement cap, the word cap, the words in one statement and the count of extras (`over-statements`, `over-words`, `statement-over-words`, `over-extras`). A long statement on a never-cut extra, and a page that runs over because of one, are hints: the topic's order rules say where a cap bends.

Nazanin, 2026-11-12, after the first run against the shape: a statement may run to 90 words, not 75, and a city page may hold one more statement and one more extra than a state page (migration 000095, `topics.city_more`). 33 of the 42 statements over 75 words were over by 15 or fewer, most because a reader fix added a condition the quote attaches. The source-tier and deadline-wording checks stay a report on these drafts, as D5 says for pages that existed before this ADR.

Each finding names its part (scope, tips, size), so the Quality tab can show one column per part beside must-answer. A cut never-cut statement cannot be seen from the page alone, so that rule stays with the reader.

## Build order

1. The spec fields, filled for the topics that exist, and `triage spec`.
2. `triage audit` as a report over existing topics, built on the functions that exist. Nazanin reads the grid for one topic and says which checks are wrong or missing.
3. The warning checks and the deadline check that do not exist yet.
4. The checks at agent save and in `triage check`.
5. The publish gate for new topics.
6. The `topic-loop` skill and the rewritten rules file.
7. The rule-set stamp for read rules.

The next new topic is the test: it runs pilot first, and the count of pages edited after the full draft is the measure.

## Consequences

- "Is this topic done?" is one command with one answer.
- A drafter cannot save a page that the gate would later refuse for a computed reason.
- A missing state is a failing row, not an absence.
- The pilot adds one stop per topic, before the bulk of the work instead of after it.
- The rules file gets shorter. Prompt rules that code now enforces are deleted from it.

## Rejected

- **A stricter gate on every existing page.** It would block hundreds of live and ready pages over checks they were never drafted against.
- **More rules in the briefs.** That is the current method, and it is what produced the sweeps.
- **A score per topic.** ADR-027 D1 holds: named checks, not a number.
- **Agent judgement inside the audit command.** Commands list and apply; reading happens in the local session (2026-09-21).

## Open questions

- Which three places are the default pilot. Suggested: California (detailed law), Wyoming (thin), Tennessee (no free code).
- Whether "confirmed live in the last 7 days" blocks publish or only shows, since one unreachable statute host would hold a whole state.
