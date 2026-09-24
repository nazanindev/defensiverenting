# ADR-026 — Playbooks answer situations, concepts answer questions

| | |
|---|---|
| Status | Accepted 2026-09-23 (Nazanin). Ground level only; concept page build-out stays deferred (ADR-020). |
| Date | 2026-09-23 |
| Amends | ADR-011 (concepts), ADR-012 (reference layer), ADR-025 (page review) |

## Context

Pages were mixing two kinds of question. "My landlord didn't return my deposit" is a situation with steps. "How much can my landlord charge for a deposit?" is an informational question. On one Illinois page, a pay-within-5-days notice sat on Can't Pay Rent, but once a notice arrives the renter is in the eviction process. The agents could not tell, because each one saw only its own page.

Nazanin, 2026-09-23: "later we are going to beef up concept pages for SEO to answer exact questions like 'how much can my landlord charge for a security deposit' so playbooks are by situation and actionable, but statements are informative and can be reused on concept pages."

## Decision

**D1. Two surfaces.** A playbook answers one situation, as steps in the order a renter acts, under the renter's search question as its title. A concept answers one informational question; its page is assembled from tagged statements across places.

**D2. Statements are informative and tagged.** Each statement makes one claim (ADR-023) and carries the concept whose question it answers. The concept list in `docs/topic-map.md` gives each concept its question.

**D3. Placement.** A statement sits on a playbook only when a renter in that situation needs it to act. Otherwise it belongs on the playbook whose situation it serves (a move), or it is parked for its concept.

**D4. Parked, never deleted.** Taking a statement off a playbook retires that row with its page; the key, text, tag, quotes and stamp stay stored. The concept build-out will let a statement live on a concept page without a playbook and bring parked statements back from there. Until then, a place drops off a concept page when its last published statement for that concept leaves every page; the 2026-09-23 trims do this for about 33 place and concept pairs, accepted as temporary.

**D5. One shared map.** `docs/topic-map.md` is the only context beyond its own page that a drafting or review agent gets: each playbook's situation, what it covers, where everything else goes, and the question each concept answers. It stays about one screen (Nazanin's caution against giving agents too much context).

**D6. Page review gains "misplaced".** A statement that answers another playbook's situation is a page finding; the fix is a move: removed here, added there if the other page lacks it. Drafts only, as with every ADR-025 action.

## Consequences

- Drafters and page reviewers read the map; misplaced statements get caught before a person reads the page.
- Concept questions exist now, so the concept build-out starts from a list of exact questions with statements already tagged.
- A statement's tag becomes more important than its page. A wrong or missing tag is a real defect.
