# ADR-012 — Reference pages and the three-intent homepage

| | |
|---|---|
| Status | Accepted |
| Date | 2026-08-21 |

## Context

Renters arrive with three different intents. *Something is happening to me* ("landlord kept my deposit") — served by search and the situation list. *What are my rights here* — served by hubs and the location scope (ADR-006, and deliberately a scope rather than a browse axis). And *what does this word mean* ("what is a notice to quit") — served by nothing: the definitional query class is owned by dictionary-grade content elsewhere that defines the term and then abandons the reader, when the concept registry (ADR-011) already holds both the definition and every covered jurisdiction's specific answer, verified and stamped.

The homepage was designed search-first, but at seven topics the situation list answers most visits faster than typing, and search usage has gone stale. Search's value returns with scale and with a reference layer it can surface — not from repositioning the box.

## Decision

### D1. A concept page is a projection of verified statements, not a new prose layer

`/c/{slug}` renders a concept as: the published national tagged statement as the definition (with its citation chips and trust line), then every published localized statement grouped by place, each linking to its home page at the concept anchor. Nothing on the page is written for the page — it is assembled entirely from statements a human verified and published, so it inherits the citation invariant and the freshness stamps instead of creating a second editorial surface to keep true. A concept with no published tagged statement anywhere has no page; the reference layer never renders an empty shell.

### D2. `/terms` is the crawlable index

Like `/locations` for places (ADR-006 D3): one page listing every concept that has a page, with a one-line blurb derived from the first sentence of its national definition where one exists. In the sitemap for the same reason `/locations` is — it is the page that spreads link equity to the reference layer.

### D3. The homepage serves three intents as three lists

Search plus the situation list stay primary. A "Legal terms, explained" section joins them, listing the terms that have a national definition — a list, not a card grid (the 2026-08-09 rule stands) — with the full index one link away at `/terms`. The section grows only when national pages are published, so its size is governed by editorial output, not by content volume. Location stays a scope; the homepage still browses by situation and now by term, never by place.

### D4. Search knows the registry

A query matching a concept's name or slug surfaces that concept page as a "Legal term" result above the prose matches, gated on the page existing. The registry is a few dozen rows, so this is a plain substring match, not an index — the win is routing "notice to quit" to the page that defines it and lists every answer, instead of to whichever statement's prose ranked.

### D5. English-only first

Reference pages and the terms index register unprefixed only, following ADR-007 D2's scope for site chrome. Spanish reference pages become worthwhile when Spanish statements carry tags, which follows the translation content work — the pages assemble themselves the same way when that day comes.

## Consequences

- The definitional search intent ("what is X") finally lands on a page that answers it *and* hands over the reader's jurisdiction-specific rule — the site's core promise applied at the search-entry level.
- Every state added makes every concept page one row better with no per-page work; at 50 states the pages are the primary-sourced successor to the survey charts the lawyer-article policy retired, and become citable backing for the national pages' distribution claims.
- Publishing a national guide now also lights its concepts up on the homepage — one more reason the review queue matters.
- The concept anchor (ADR-011 D4) gains a second consumer; anchors must stay stable, which the closed registry already guarantees.
