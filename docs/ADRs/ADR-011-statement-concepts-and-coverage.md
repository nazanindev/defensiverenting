# ADR-011 — Statement concepts and resolution-aware coverage

| | |
|---|---|
| Status | Accepted |
| Date | 2026-08-21 |

## Context

Coverage is tracked per playbook: the dashboard matrix answers "does Pittsburgh have a rent-increase page", and since ADR-009 the national guides answer it for everywhere else. Below the page, nothing is tracked. Three observations from one editing session (2026-08-21) point at the same missing layer:

1. The Boston security-deposits page states "your landlord cannot punish you for reporting them — this is called retaliation," cited to a Massachusetts statute. The national page states the same claim generically. These are not duplicates: the Boston statement is a *localized instance* of a claim the national page can only make in outline. But the relationship exists only in the author's head — nothing connects them, nothing can ask "which places have their own version of this."
2. National pages accumulate "this varies by state — check your state" statements with no follow-up. ADR-009 D1 requires that phrasing (a nationwide rule may only be claimed with nationwide backing), so the dead ends are by design — yet the localized answers often already exist one hop down, and answering them is the point of the site.
3. There is no queue for the gap in between: no way to ask "which covered places lack a locally-cited statement for a claim the national page makes generically."

Separately, the editorial-source incident (same day: third-party commentary filed under `kind='editorial'` hijacked the editorial checkbox) re-taught the lesson of ADR-005 D5: vocabularies that anything can extend at save time drift until they break something. Whatever connects statements across pages must be a closed registry.

## Decision

### D1. Concepts are a closed registry; a statement carries at most one

A `concepts` table — slug, display name, owning topic — extended only by migration, exactly like the topic registry (ADR-005 D5). Statements get a nullable concept reference. One concept per statement because statements are atomic claims (ADR-003); a statement that seems to need two concepts is two statements. A concept belongs to a topic so the form's picker shows a short relevant list, not the whole vocabulary. Untagged statements stay fully legal: concepts describe claims that recur across jurisdictions ("retaliation-protection", "rent-increase-notice-period"), and a page-specific procedural statement ("bring the form to Room 240") never needs one.

### D2. Both authoring paths tag, under the same closed-registry guardrail

The edit form offers a concept select per statement, filtered to the page's topic. The drafting agent gets an optional `concept` field on `save_draft_playbook`, validated against the registry with the same reject-and-name-the-choices response an unknown topic slug gets — the agent may reuse the vocabulary, never extend it. Future drafts arrive pre-tagged for the author to correct, which is cheaper than tagging from scratch; the existing corpus is backfilled by tagging the national pages first (they are already the generalized instances) and letting city statements be tagged as pages come up for review.

### D3. Coverage is resolution-aware, not flat

For each concept × covered place, the status is **localized** (the place's own page carries a tagged statement with its own citations), **generic** (the claim resolves upward to a tagged ancestor statement, in practice the national page — the floor ADR-009 D4 built), or **missing** (no statement anywhere up the chain). The dashboard queue is the *generic-only* set, not the missing set: a Boston renter always gets some answer via resolution, so the work worth queueing is "this place has only the outline — research the local law." A flat "does this page have concept X" would send authors to paste generic sentences onto city pages, which is the junk this design exists to prevent. The editorial rule that pairs with it: a local page carries a concept's statement only when it says something local — a statute, a stronger remedy, a local process. Otherwise the page stays silent and resolution covers it.

### D4. National pages link tagged statements to the topic hub *(amended 2026-08-21)*

Under a tagged national statement, the page renders one line — "This rule depends on where you live. See the states and cities we cover." — linking to the topic hub (`/t/{topic}`), which already lists every covered jurisdiction bucketed by kind (ADR-009 D3). The link renders only when some jurisdiction besides the national page covers the topic, so it never points at a hub whose only entry is the page the reader is on. Tagged statements still render their concept slug as an HTML anchor, so `/j/pennsylvania/pittsburgh/rent-increase#rent-control` stays a stable deep link for anything that wants one. Same properties as before: content-driven, identical for every reader (no cache-leak shape, contrast ADR-009 D5), internal links only.

*As first accepted, this decision rendered a per-place link row ("Specifics for: Massachusetts · Pennsylvania · …") deep-linked to each localized statement. Amended the day it shipped: the row grows with every covered place, and at even eight cities it competes with the statement it hangs under, while the topic hub already is the page that answers "which places do you cover" — one link that scales beats a row that has to be re-justified at every size.*

### D5. Statement inheritance is considered and deferred

Rendering national statements onto city pages that lack a concept would eliminate the duplication wholesale, and server-side composition makes it mechanically easy. It is deferred because its costs land in exactly the places this project protects: verify-and-publish is page-scoped, and inheritance makes one national edit silently change every city page; byte-identical paragraphs across dozens of city URLs is the duplicate-content pattern that risks the search standing city pages depend on; and Spanish (ADR-007/008) would couple every city translation to the national statement's translation. Revisit when the D3 matrix shows generic-only gaps that D4's links demonstrably fail to serve, or when Spanish makes duplicated statements expensive enough to consolidate. Until then, decided against — explicitly to avoid touching SEO.

### D6. The portal shows a source's citation structure

One URL is one source row (`sources.url` is UNIQUE); citing three sections of one statute page is one source with three citation locators. The portal now surfaces that rollup — per source: how many statements cite it, across which locators and pages — so "the same page under different statutes" reads as the structure it already is rather than as apparent duplicates. Near-duplicate *rows* (the same document typed as slightly different URLs, the failure the import picker's comment names) are a separate cleanup with a merge path, out of scope here.

## Consequences

- Bootstrap order is part of the design: tag the national pages first and the concept vocabulary largely writes itself; the D3 matrix lights up city gaps immediately after.
- Every "check your state" statement becomes either a door (D4, where localized instances exist) or a queue entry (D3, where they don't). The same registry powers both.
- Concept slugs double as stable public anchors, usable anywhere the site needs to point at a specific claim.
- Adding a concept is a migration, so vocabulary growth is reviewed the way topic growth is — deliberately.
- D1–D3 ship with no renter-visible change; D4 is the first public surface and adds only internal links. D5 stays a documented trigger, not a roadmap item.
