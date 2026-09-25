# ADR-027 — Quality is a set of checks, and each page shows its worst one

| | |
|---|---|
| Status | Proposed 2026-09-24 |
| Date | 2026-09-24 |
| Amends | ADR-014 (fills the reserved `source-quality:<signal>` code), ADR-025 (page review gains actionability checks) |

## Context

Hundreds of pages are in draft or live, across cities, states and the nationwide guides, and more are coming. The review loop checks each claim against its stored quote (ADR-022) and each page as a page (ADR-025). Nothing yet says, across all pages, which ones are weakest and why.

Nazanin, 2026-09-24, asked for a consistent way to watch quality with a few metrics: the plainness score shipped that day (ab48c10), a source-quality check ("laws are strongest, non-profits are weakest"), and a page-level check of how actionable a page is for a renter in that situation.

Two things shape the design:

- A number a model makes up, like "actionability 7/10", drifts between runs and cannot be argued with. A named check with evidence ("no statement tells the renter what to do first") can be fixed and then shown fixed.
- Ranking sources by kind alone is wrong. "Call 311 to ask for an inspection" is best backed by the city's own page. A statute adds nothing to it.

## Decision

### D1. Checks, not scores

Every metric is a set of pass or fail checks. A check is either computed in code or read by an agent against a stated rule, in the local session (no API calls in commands). A failure is filed through the paths that exist: a `source-quality:<signal>` proposal (ADR-014) or a page note (ADR-025). QA never gets its own queue.

There is no combined score. An average hides the one check that failed.

### D2. Plainness joins as a check

Plainness is the ab48c10 lint: New Dale-Chall under 6.5, unfamiliar words glossed, 120-word cap. It already blocks at save and publish. Here it becomes one line on the report: the share of a page's statements that pass.

### D3. Source strength is judged against the kind of claim

Source tiers, strongest first:

| Tier | Kinds |
|---|---|
| 1 | `statute`, `regulation` |
| 2 | `court_ruling` |
| 3 | `gov_guidance` |
| 4 | `nonprofit` |
| 5 | `editorial` |

Nazanin's decisions (2026-09-24): court rulings sit one tier below law, and a nonprofit may back up a claim but never carry a legal rule alone. For now every `court_ruling` counts as tier 2. Telling a published appellate opinion from a trial-court ruling is deferred (see Later).

Each statement gets a claim kind:

| Claim kind | What it says | Passes with |
|---|---|---|
| legal rule | a deadline, an amount, a right, what a landlord must or cannot do | tier 1 or 2 |
| procedure | where to file, which form, who to call | tier 3 or better |
| practical | keep records, take photos | any tier |

A statement is judged by its best citation, not the average. One statute beats three nonprofit pages.

The claim kind starts computed from the text: a number with days, dollars or a percent, or "must", "cannot", "may not", "has to", "is required", marks a legal rule. A statement that names an agency, court or form with no such marker is a procedure. Everything else is practical. The miss rate is measured on the first run before anything is stored. If it is high, the review agent reads the edge cases, the same way every other rule was introduced (ADR-021).

Signals, each filed as `source-quality:<signal>`:

- `below-tier`: a legal rule or procedure with no citation at the tier it needs.
- `nonprofit-alone`: a legal rule whose only support is a nonprofit or editorial source. This is a special case of `below-tier`, named because Nazanin singled it out.
- `archive-url`: cited from a Wayback snapshot when the live page exists.
- `browser-tier-only`: only the browser tier can fetch the source.
- `no-locator`: a statute citation with no section.
- `unchecked`: an empty quote, or no readable check in the last 7 days.

A signal's proposal has NULL `proposed` (a work item). It carries an added citation only when `find_sources` returns a tier-1 or tier-2 candidate for that place. Agents act on drafts only. On a live page the finding shows, and the edit stays with a person (2026-09-22 decision).

### D4. Actionability is ADR-025 plus five checks

ADR-025 already reads a page for duplicates, relevance, order, contradictions, gaps and title fit, and ADR-026 added misplaced statements. Those stay. ADR-026 D1 says a playbook answers one situation as steps, so actionability means the page gives those steps. The page reviewer adds:

1. **Answer first.** One of the first two statements answers the title question. *Read.*
2. **A first step.** At least one statement tells the renter to do something, starting with a verb ("Write", "Send", "Keep"). *Computed.*
3. **Deadlines have numbers.** When a cited quote on the page sets a time limit, the page states the number of days. *Computed hint, read to confirm.*
4. **Somewhere to go.** The page names a place to get help for this location: an agency, a court, or legal aid. *Computed from the local-help region and contact citations.*
5. **Risks paired.** A risky step carries its warning. The existing lints cover the known cases (lease, withholding, inspector, police). *Computed.*

A failure is a page note with kind `actionability:<check>`, filed like every ADR-025 finding. It holds a draft from publishing the same way.

### D5. One line per page, weakest pages first

Each page shows one thing: its worst failing check in plain words, for example "2 legal rules cite only a nonprofit". Worst means: source failures on legal rules, then actionability, then everything else. A page with no failures shows nothing.

The dashboard gets one list: pages ordered from most failing checks to fewest, with that line beside each. The queue order from ADR-024 D5 does not change. QA items rank by where they sit, like everything else.

### D6. Later: conflicts across pages

ADR-025 catches two statements that disagree on one page. The same concept with different numbers on a city page and its state page is not caught. A local rule sometimes overrides the state one, so a match is a note for a person, never an edit. This needs the numbers pulled out per concept per place. It comes after D3 and D4.

## Later: court rulings by court level

A trial-court or unpublished ruling binds no one else and should count as tier 3. Sorting them needs `court_ruling` sources to record the court level and whether the opinion is published (CourtListener has both). Deferred by Nazanin, 2026-09-24. Until then a weak ruling can pass as tier 2.

## Later: the coverage page

The author portal's coverage page is to be redesigned together with this work, as the place where coverage and quality are read. Nazanin wants something more advanced than a list with a line per place, and will set the design when she implements it (2026-09-24). Known problems with today's page: states are missing from the topic matrix (it covers cities and the US row only), and the concept matrix grows a column per place. D5's dashboard list may be replaced by that page.

## Build order

1. D3 as a report only: claim kind, tiers, signals, counted across all pages, with no filing. Nazanin reads the counts and the miss rate on claim kind.
2. D3 filing on drafts.
3. D5 line and dashboard list.
4. D4 checks in the page reviewer.
5. D6.

## Consequences

- The last open item from ADR-014 (the source-quality scorer) is done.
- The report gives the next state waves a bar to draft to: every legal rule on tier 1 or 2 before a page reaches review.

## Rejected

- **A combined quality score.** It hides which check failed, and "one number per page" is better served by the worst failure.
- **An agent rating pages from 1 to 10.** It cannot be reproduced or fixed against.
- **Ranking sources by kind alone.** It would flag every procedure that cites the agency that runs the procedure.
- **Share of statements a person has reviewed.** Statement approval is moving to agents under stated rules (ADR-021), so this would measure a process being retired. Nazanin, 2026-09-24.
- **`single-citation` as a signal.** Under best-citation scoring, one statute is enough.
