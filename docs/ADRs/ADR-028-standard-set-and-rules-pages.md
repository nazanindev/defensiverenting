# ADR-028 — The standard set is built from renter situations, and rules live on rules pages

| | |
|---|---|
| Status | Accepted 2026-10-04 (Nazanin). Phase 1 built 2026-10-04 on branch worktree-adr028-voice (migration 000048); see Build notes. |
| Date | 2026-10-03, settled 2026-10-04 |
| Amends | ADR-020 (lifts the deferral; adds rules pages), ADR-026 (D4 parked statements get a home; topic map rewritten), ADR-012 (projection rule kept, one new page kind) |

## Context

Every state and DC now has published guides, which is the trigger ADR-020 set for building concept pages. Before drafting more, Nazanin asked for an audit of the standard set: the 7 `is_core` topics every place is seeded with.

The audit, against prod on 2026-10-03:

- The 7 core topics are complete in every city and in nearly every state. Ten state pages are missing, each skipped on purpose for lack of a fetchable law or a thin page (AR, IN, MS, TN, WV, WY).
- Recovery questions are covered almost everywhere: deposit return deadline 50 states, itemization 51, what you can win 49.
- Move-in questions are nearly empty: deposit cap 9 states, interest 4, receipt 1. The topic map sent them to `move-in-checklist`, which was never drafted, so drafters never wrote them.
- Concept pages project only statements published on a playbook (ADR-012). So "How much can a landlord charge for a security deposit in Ohio?" cannot exist until an Ohio playbook carries the Ohio cap.

Putting the cap on a per-state move-in checklist would publish it, but the page would not be found. Nazanin, 2026-10-03: "We can't guarantee a renter searching 'security deposit rules Ohio' is going to land on the right move-in checklist page." A search lands on the page whose title matches it.

So we listed, area by area, the situations a renter meets and sorted each one by what kind of page answers it. Every call below was agreed with Nazanin on 2026-10-03.

## Decision

### D1. Four kinds of content

| Kind | Answers | Varies by place | Backed by |
|---|---|---|---|
| Situation playbook | "This happened to me. What do I do?" Steps in order, with risks | Yes | Law, editorial for advice |
| Concept | "What is the rule?" One fact | Yes | Law only |
| Checklist | "How do I prepare?" | Mostly no, so one national page | Editorial, links to concepts for the numbers |
| Term | "What does this word mean?" | No | Plain definition (`/terms`) |

A fact that sits on a page of another kind is a placement, not a fifth kind. A situation that is mostly federal law gets one national playbook, and its state add-ons are concepts.

### D2. The standard set

**Per place (every state and DC), 15 topics, `is_core`:**

| Slug | Situation | |
|---|---|---|
| `cant-pay-rent` | Can't pay rent (before any notice) | exists |
| `eviction-defense` | Eviction notice or court papers, through removal | exists, stays one page |
| `security-deposits` | Deposit not returned or wrongly kept (also roommates, left early) | exists |
| `repairs-and-habitability` | Landlord won't fix it | exists |
| `landlord-entry` | Landlord entering without notice | exists, narrowed to entry and privacy |
| `rent-increase` | Rent raised | exists |
| `resource-directory` | Where to get help | exists |
| `breaking-lease` | Need to leave before the lease ends (also unlivable home) | new |
| `lease-renewal` | Landlord won't renew, or month-to-month ending | new pages, slug exists |
| `locked-out` | Locked out, belongings taken, utilities cut to force you out | new |
| `utility-shutoff` | Utilities off: landlord cut them, landlord did not pay, or I could not pay | new |
| `building-sold` | Building sold or foreclosed: lease, who to pay, deposit | new |
| `move-out-bill` | Billed after moving out: damage charges, rent still owed | new |
| `discrimination` | Treated unfairly because of who I am | standard now, 1 page today |
| `heat-not-working` | Heat or AC not working | standard now, 5 pages today; title becomes "Heat or AC Not Working in {State}: What Can I Do?" |

The thin-page rule still decides: a state with too little law for a topic gets no page, and its facts stay as rules-page statements (D4).

**Heat or AC.** Added after the first draft of this ADR: the one situation a reader has emailed about is AC, and "landlord won't fix AC" is a common search in hot states. A read-only survey of all 50 states and DC on 2026-10-03 (official sites, quotes still to be re-fetched by drafters) found:

- A heat rule of some kind in about 42 states; a set temperature or season in about 17.
- AC in the "maintain if supplied" clause in about 25 states. Stronger AC rules in Arizona, Virginia, Nevada and Utah.
- A renter's right to install a cooling device in Colorado and Oregon, and Washington from 2026-06-11 (not yet verified).
- City AC rules in Phoenix, Dallas and Montgomery County; reported new rules in Houston and Austin (code text not yet read).
- Nothing at state level in Hawaii, Louisiana, Michigan, Missouri, Texas. New York, Pennsylvania and Tennessee could not be read and are checked before drafting.

So the slug stays (5 live pages use it) and the topic becomes standard, with the thin-page rule. Its line with `repairs-and-habitability`: a general "keep the heat working" duty stays on the repairs page; this page carries the temperatures and seasons, AC if supplied, cooling-device rights, and the hot or cold weather safety steps.

**National only (`united-states`):**

| Slug | Kind |
|---|---|
| `move-in-checklist` | checklist |
| `move-out-checklist` | checklist |
| `rental-application` | checklist (fees, screening, scams, what to do if turned down) |
| `assistance-animal` | situation (federal law) |
| `renting-fundamentals` | overview (exists) |

**Retired:** `constructive-eviction`. The concept moves to `breaking-lease`. The live US and PA pages stay live until `breaking-lease` publishes in those places; then they are retired and their URLs redirect to `breaking-lease`, so a reader never meets a gap. **Deleted:** `noise-complaints`, `rent-stabilization` (no pages; `quiet-enjoyment` and `rent-control` cover them).

**Cities:** a city gets a new topic only where city law adds to the state's (for example just-cause rules in Los Angeles, Seattle, New York City, or an AC rule in Austin where Texas has none). Otherwise the city reader falls to the state page by the existing chain walk.

### D3. Concepts: 45 exist, 46 new

| Area | New concepts |
|---|---|
| Deposits | nonrefundable-fees, deposit-increase, move-in-condition-report, holding-deposit, deposit-last-month-rent, deposit-after-sale, move-out-inspection, forwarding-address, move-out-charges |
| Can't pay | refused-rent, rent-receipt, subsidized-rent-change (national), duty-to-mitigate, early-termination-rights |
| Eviction | default-judgment, eviction-appeal, time-to-move-after-judgment, abandoned-property, right-to-counsel, end-of-tenancy-notice, just-cause-eviction, foreclosure-tenant-protection |
| Repairs | landlord-unpaid-utilities, energy-assistance, heat-requirement, ac-requirement, cooling-device-rights, mold, pests-bed-bugs, lead-paint, smoke-co-detectors, condemnation-relocation, casualty-damage |
| Entry and rent | refuse-entry, landlord-harassment, added-fees, utility-billing, emergency-price-gouging |
| Discrimination and applying | reasonable-accommodation, assistance-animal-rules, source-of-income, fair-housing-complaint, domestic-violence-protections, application-fees, tenant-screening, criminal-history-screening |

Changes to existing concepts:

- New home topic: `late-rent-notice` to `eviction-defense`; `illegal-lockout` and `court-eviction-only` to `locked-out`; `utility-shutoff-protection` to `utility-shutoff`; `constructive-eviction` to `breaking-lease`; `fair-housing` to `discrimination`.
- New question: `lock-change-rules` becomes the renter's own lock rights (rekey on move-in, lock change after abuse), since lockouts moved. `eviction-record` covers sealing.
- Slugs do not change. `/c/{slug}` URLs are already public, so a rename would need redirects for nothing.

Every concept carries its question (ADR-020 D1), set by migration with the rest of the registry.

### D4. A rules page per place and topic holds the facts no situation needs

New `page_kind` value `rules`: one page per place and home topic, titled "{Topic} Rules in {Place}: What Does the Law Say?" ("Security Deposit Rules in Ohio: What Does the Law Say?"). Same pattern as every guide title, with the words people type first.

- Its own statements are the concept facts that no situation playbook in that place carries (the Ohio cap, receipt, interest).
- It renders its own statements together with the published statements from that place's situation playbooks whose concept homes in that topic. One answer per concept, in concept order, each linking to its concept page and to the situation playbook.
- It is a playbook row, so it goes through the same loop: draft, read, fix, judge, stamps, publish gate, Nazanin publishes. No new review path, no new queue.
- Statements stay on one page each (prod has 0 statements on two pages today). A fact never gets a second row on the rules page when a situation page already carries it.
- Concept pages (`/c/{slug}`) keep projecting published statements, now from rules pages as well. ADR-012's projection rule holds; ADR-026 D4 parked statements come back on the rules page.

Rules pages ride the playbook pipeline; they do not get their own (decided 2026-10-04, see Rejected).

This one page does three jobs: it catches the broad search, it gives concept-only facts a reviewed home, and it is the per-place hub.

The concept tag is the hinge. Concept pages and rules pages are two slices of the same statements: a concept page is one concept across every place (ADR-020), a rules page is one place across every concept in a topic. Each statement is stored once and shows on both.

How a rules page is drafted:

1. Code works out the page. For each concept whose home is the topic, it looks for a statement with that tag on one of the place's situation pages. Found: the rules page shows it, and no agent touches it. Not found and no coverage record: the concept is a gap.
2. The drafter gets only the gap list ("Ohio: deposit-cap, deposit-receipt, deposit-escrow-interest") and does what drafters do today: find the law, quote it, write the statement. Finding none, it says so, and the reviewer files a coverage record (D5).
3. The loop reviews only those new statements.

Agent judgement decides what the law says, never what to show or copy; that is a lookup by tag. A situation page still in draft counts as covering its concepts, so nothing is drafted twice. When a situation page later takes a concept the rules page holds, a check flags it and it moves (ADR-026), and the rules page then shows it from there.

### D5. "No law found" is a coverage record, not a statement

Many states have no law on a question (grace period, cap, interest). A concept page that is silent for those states breaks the promise of its title (ADR-020 D7).

- When an official source says there is no rule, that is a normal statement citing it.
- When none does, the reviewer files a coverage record: place, concept, "searched, no law found", the sources checked, who, when. The concept page and the rules page both render "We did not find a {State} law on this. Last checked {date}." A rules page with a question left blank looks forgotten; the dated line shows we looked. This is a site fact about our search, not a legal claim, so it carries no citation and is never phrased as "{State} has no rule".
- A coverage record is listed by the weekly check with its date, so a new law is a reason to look again.

### D6. National pages

National checklists use the existing `checklist` page kind (already allowed by migration 000004). The national situation playbook (`assistance-animal`) is a normal playbook on `united-states`. Topics gain a `national_only` flag so seeding, coverage and the place picker skip them per state. State add-ons (`assistance-animal-rules`, `application-fees`, `tenant-screening`, `criminal-history-screening`) live on rules pages and are linked from the national page by concept.

### D7. Moves stay human on live pages

Statements now on a live page whose situation moved (lockout statements on `landlord-entry`, unlivable-home statements on `repairs-and-habitability`) are not touched by agents. The new page is drafted first with its own statements; then the removal from the live page is filed as a proposal in the queue for Nazanin. Drafts are moved by agents as ADR-026 D6 allows.

### D8. The topic map is split by area

91 concepts and 20 topics do not fit on one screen. `docs/topic-map.md` keeps the playbook table (situation, covers, not here) for every topic. The concept questions move to one section per area. A drafter or reviewer gets the playbook table plus the concept section for its page's area only, so agent context stays about one screen (Nazanin's caution, ADR-026 D5).

### D9. Gap identification comes before rules pages

Rules pages put whatever the tags say on a page, so the gap and coverage logic has to be right first. Today's logic was built for a different job:

- `ConceptGaps` (ADR-025 page review) measures a page against the page one level up. A state misses a concept only if the national page carries it, which most of the 46 new concepts will not at first.
- `ConceptCoverage` (dashboard) counts a place covered when only the national statement answers it. For a state rules page, a national line is not the state's answer.
- Nothing tells "no law exists" from "nobody looked". Grace period is tagged in 22 states; the other 29 are a mix of both, and a gap list would send drafters back to them every wave.
- A missing tag shows a false gap (and a duplicate gets drafted); a wrong tag shows false coverage (and the wrong fact gets shown). The 2026-09-23 sweep found 25 wrong tags in 62 drafts; published pages have not been swept since.

Before any rules page is built:

1. **Coverage records** (D5) exist, so a place and concept can be marked "searched, no law found".
2. **Gaps per place against every concept in the topic**: each pair is a statement, a coverage record, or a gap. Never "covered by the national page" for a state.
3. **A tag audit of published statements**: every statement that answers a concept carries the right tag, and none carries a wrong one. Same reader rules, filed as proposals; live pages stay Nazanin's.

### D10. Headings tell the renter why a statement is there

A renter in a bad situation needs to know why each part of a page is in front of them. The two page kinds answer that differently.

- **Rules pages: one question heading per concept.** The concept's question, with the place in it ("How much can my landlord charge for a deposit in Ohio?"), over that place's answer. Every statement on a rules page exists to answer a question, so the question is the heading.
- **Playbooks: a few stage headings, never one per statement.** A playbook statement exists because it is a step, so the heading says where the renter is in the process. 3 to 5 per page, short, in the renter's words. A deposit page, for example: "What the law says", "Ask for your deposit back", "If your landlord does not pay", "Going to court". Question headings over each statement were rejected as crowded, and many steps carry no concept to ask.

How stages work:

- Each topic has a fixed stage list, kept in the topic map next to its situation. Drafters and reviewers pick from it; nobody invents a heading.
- Each statement on a playbook carries its stage. A heading renders where the stage changes.
- Stages follow the statement order drafters already use (law, first step, next, if ignored, court). A stage that appears twice, split by another, is an `order` finding in page review (ADR-025).
- Existing drafts get stages from the review agent. Live pages get them through the queue, for Nazanin.

Both kinds of heading also help search: a heading over a short passage is the clearest signal for both a search result snippet and an AI answer.

**Amendment (2026-10-06): order and repetition.** The phase 3 pages read as a pile: breaking-a-lease pages opened with domestic violence, sexual assault and military exits (37 of 51), and lockout pages restated the same rule two or three times. Nazanin: edge cases like sexual assault "never need to be first, those are edge cases, and none help a reader in a common situation." Rules, for drafters, reviewers and the backfill of every existing page:

- **The common case leads.** Within each stage, the rule most readers need comes first; exceptions, carve-outs (hotel stays, lawful evictions, covered units) and narrow cases come last.
- **Breaking a lease has a Special protections stage** for victim exits (domestic violence, sexual assault, stalking, trafficking, crime victims) and military exits, with their notice, papers and rent rules. Stage order: What you may still owe · When you can leave early · How to leave · Special protections · After you move out (migration 000064).
- **One rule, one statement.** Two statements that say the same rule, or where one only points to the other's list, are merged; every condition and citation of both is kept.
- **Warnings stay per statement for now.** The risk and win-and-pays lines repeat on each statement that needs them; showing them once per section is deferred.
- The review screen shows stage headings on a page's view (9ee789d). Live pages carry no stages yet; they get them through the queue.

### D11. Every statement stands alone for search

Search engines rank a passage inside a page, and AI answers retrieve a chunk of one. Most renter questions are answered by a statement, not a title. The standing-alone rule (names its own subject, no pointer words, one claim, its own numbers and source) already serves that. Two additions:

- ~~**The place is named in every statement** ("Ohio law..."), since a passage shown alone loses the page title. A drafter rule and a lint on new saves; live pages are not retrofitted.~~ Withdrawn 2026-10-04, see the amendment below.
- **Each statement has a stable link**, an anchor on its durable statement key, so a search result can jump to it and an AI answer can cite the exact claim.

**Amendment (2026-10-04).** The place rule is withdrawn and the lint removed. After phase 3, pages read like forms: "In Nebraska, ..." opened nearly every statement. Nazanin: "We want people to actually be able to read these." Search results print the page title above a passage, and the title, H1, headings and concept-page place headings carry the place. A statement names a place only when it covers a different area than the page: a city ordinance on a state page, part of a state (Tennessee's 17 counties), or federal law on a state page. The phase 3 drafts were rewritten to match. The same pass dropped "the state handbook says" from statements whose source is a government office stating the law; legal aid, nonprofit, undated and hedged sources are still named in the text.

### D12. Batch order

Two Sonnet drafters at a time, the state-review-loop as it is. Each phase converges before the next starts.

0. **Gap identification (D9):** coverage records, per-place gaps against every concept, the tag audit.
1. **Build:** migrations (topics, flags, concepts and questions, home moves, deletes, stage lists), `rules` page kind and render, question and stage headings, coverage records, topic map split, state-brief titles for the new topics.
2. **National pages:** the 3 checklists and `assistance-animal`. Five pages, one session.
3. **`locked-out` and `breaking-lease`:** most searched, strongest law. Up to 102 pages.
4. **Deposit rules pages:** the gap that started this audit (cap 9 states, interest 4, receipt 1).
5. **`heat-not-working` (heat or AC), `lease-renewal`, `utility-shutoff`, `discrimination`.** Heat or AC first in this phase, after New York, Pennsylvania and Tennessee are checked.
6. **`building-sold`, `move-out-bill`.**
7. **Rules pages for the other areas**, filling concept gaps state by state, with coverage records where no law exists.

Up to 407 new situation pages and 51 rules pages per area before thin skips. That is several waves of the size run so far (about 70 pages each).

## Consequences

- Every renter search shape has a page: the situation ("landlord locked me out Texas"), the broad rules search ("security deposit rules Ohio"), the exact question ("how much can a landlord charge for a deposit").
- Concept-only facts get reviewed and published through the loop that already works, with no second path to maintain.
- Concept pages can promise an answer for every state: a cited rule, or a dated "we did not find a law".
- The number of pages per state roughly triples. Review load grows with it; the editor sees more drafts.
- `is_core` changes meaning from "seeded in every new city" to "standard for every state". Cities follow D2's city rule instead.

## Rejected

- **A move-in checklist per state.** It publishes the cap but does not catch the search, and repeats the same steps 51 times around one changing number.
- **Widening `security-deposits` to all deposit rules.** Breaks one situation per page (ADR-026) and still does not match "how much can a landlord charge".
- **Concept-only statements with their own review and publish path.** A second gate, a second queue view, a second publish button. The rules page reuses the one path.
- **One statement row shared by a situation page and a rules page.** Nothing shares rows today; revisions, stamps and drift would all need to handle it. The rules page renders the situation page's statement instead.
- **Splitting `eviction-defense` into notice and court pages.** Rewrites 61 pages; concept pages already catch "eviction notice Ohio".
- **`holding-deposit` and `refused-rent` as playbooks.** Short steps and thin law in most states; concepts carry them.
- **"{State} has no rule" as a statement without an official source.** Inferring law from silence is the claim the loop exists to stop.

## Settled 2026-10-04

- Rules pages piggyback on the playbook pipeline (D4).
- Live `constructive-eviction` pages stay until `breaking-lease` publishes, then redirect (D2).
- Rules page title: "{Topic} Rules in {Place}: What Does the Law Say?" (D4).
- "We did not find a law" shows on rules pages and concept pages (D5).
- Coverage records, the `national_only` flag, no slug renames, the split topic map: accepted as written.
- Place named in every statement and a stable statement link (D11).

## Before the build

1. Stage lists for the 15 playbook topics (D10): drafted by Claude, read by Nazanin. Done 2026-10-04, `docs/ADR-028-stages-and-questions.md`.
2. Final wording of the 46 new concept questions, read by Nazanin before the migration. Done 2026-10-04, same file; headings follow `site-copy.md` rules 10 and 11 (official terms keep their name with an everyday-words gloss).
3. Right before phase 5: confirm heat and AC law in New York, Pennsylvania and Tennessee, and the Houston and Austin AC ordinances.

## Build notes (2026-10-04)

**A rules page is its own topic (Nazanin, 2026-10-04).** D4 said "new page_kind value rules, one page per place and home topic". A slot holds one page per place, topic and language, and every save, publish, revision and URL path finds a page by that slot, so a rules page on the same topic as its situation page would need all of them changed and could overwrite the live guide. Instead each situation topic that is home to concepts gets a rules topic: `security-deposit-rules` beside `security-deposits`, linked by `topics.rules_for`. Page kind `rules` stays as the layout, and the store keeps the two together on every save. URLs: `/j/ohio/security-deposit-rules`. Nazanin: "rules pages are equivalent to a playbook in code."

Rules topics: rent-payment-rules, eviction-rules, security-deposit-rules, repair-rules, heat-and-ac-rules, landlord-entry-rules, rent-increase-rules, lease-breaking-rules, lease-renewal-rules, lockout-rules, utility-shutoff-rules, building-sale-rules, discrimination-rules, rental-application-rules. move-out-bill homes no concept, so it has none.

**Concept homes for the 46** (D3 grouped them by area; the home is the topic whose rules page asks the question): duty-to-mitigate and early-termination-rights on breaking-lease; end-of-tenancy-notice and just-cause-eviction on lease-renewal; foreclosure-tenant-protection on building-sold; landlord-unpaid-utilities and energy-assistance on utility-shutoff; heat, AC and cooling-device on heat-not-working; added-fees, utility-billing, price gouging on rent-increase; application-fees, tenant-screening, criminal-history-screening on rental-application (state rules on rental-application-rules pages); every other new concept on the topic its area names. Tagging stays open: any concept may be tagged on any page (since 2026-08-22).

**Storage.**
- `concepts.question` and `concepts.position` (rules-page order); `concepts.national_only` keeps subsidized-rent-change off state gap lists.
- `topics.stages`, `topics.rules_for`, `topics.national_only`.
- A statement's stage lives on `playbook_statements.stage`, outside the review hash: moving a step under another heading does not change the claim or reset its stamp. A save that passes no stage keeps the stage the statement's key already had, so approvals, the authoring form and re-drafts never wipe stages.
- `coverage_records` (D5): place, concept, sources searched, who, when. Refused for a place that already has a statement with the concept.

**Gaps (D9 step 2).** `RulesAnswers` works out a rules page by tag lookup: for each concept homed in the situation topic, the place's statement (a situation page before the rules page, the home topic before others, published before draft), else its coverage record. `triage gaps <place>` lists it with drafts counted; the public page counts published only and leaves gaps off.

**Place named (D11).** SaveDraft lints new statements (no key) on playbook and rules pages; kept statements, directories and the nationwide page are exempt. City pages may name the city or its state.

**Not built yet.**
- D9 step 3, the tag audit of published statements, and filing coverage records: both need prod reads and agent reading.
- Stages on existing drafts (review agent) and live pages (queue).
- The weekly check listing coverage records by date (D5): `ListCoverageRecords` exists; not wired into the check run.
- The constructive-eviction redirect: a step after breaking-lease publishes in the US and PA.
- `ConceptCoverage` on the dashboard still counts "covered by the national page" for states; the rules-page gap list does not.

