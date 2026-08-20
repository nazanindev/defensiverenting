# ADR-009 — National guides and upward location resolution

| | |
|---|---|
| Status | Accepted |
| Date | 2026-08-19 |

## Context

City guides rank for location-free queries ("can my landlord enter without permission"), so a reader in Phoenix lands on a Boston page. The fix is country-wide guides that catch the no-location query and route the reader to their own jurisdiction.

The data model needed nothing: a playbook keys on (jurisdiction, topic, language), `jurisdictions.kind` has had `country` since migration 000001, `united-states` is seeded, the router already resolves `/j/united-states/{topic}` through the two-segment fall-through, and the authoring form picker already offers a National optgroup (added when the city-only picker left "Renting Basics" unsaveable). What was missing was discovery — the sitemap, `/locations`, and the topic-hub buckets only knew about cities — and the drafting pipeline's vocabulary, which was city-only end to end.

ADR-005 asked whether `renting-fundamentals` stays a `united-states` playbook or becomes a per-state template. This ADR answers: national playbooks are first-class pages, and `united-states` is a normal jurisdiction that content hangs off.

## Decision

### D1. A national guide is an ordinary playbook scoped to the country jurisdiction

No new page type, no special routing. `/j/united-states/{topic}` was already servable; it is now also discoverable. Because landlord-tenant law is mostly state law, a national page cannot state one rule the way a city page can — the drafting system prompt now carries a nationwide rule: claim a rule nationwide only with federal or nationwide-survey backing, otherwise state the variation plainly and point the reader at their own guide. The verbatim-citation guardrail applies unchanged.

### D2. Hub discovery comes from a kind-agnostic inventory

`ListPublishedHubJurisdictions` returns every jurisdiction — country, state, or city — with at least one published playbook of its own, ordered country → state → city. The sitemap's hub list and `/locations` read it. The published-only rule is unchanged in spirit (an empty hub must not be submitted for indexing); it just stops being spelled "city". A state whose only content is its covered cities is deliberately absent: its heading in the grouped city index already links its hub.

### D3. Browse surfaces label the three kinds apart

The topic hub buckets jurisdictions by kind: cities grouped by state, then "Statewide guides", then "Nationwide guide". Without the third bucket a national guide filed under "Statewide", which is wrong on its face. `/locations` gets a "Nationwide" section above the state index for the same reason: grouped like a city, "United States" renders under an empty state heading. Playbook and hub chrome that assumed a place name ("More tenant rights in United States") takes a `kind == "country"` branch instead.

### D4. `?j=` resolves to the nearest guide up the ancestor chain

`/t/{topic}?j={slug}` used to redirect only when that exact jurisdiction had the topic, else degrade to the picker. It now walks up — the city's own guide when it has one, else the state's, else the national guide (`GetNearestTopicJurisdiction`, one recursive query mirroring the search scoping CTE) — and only degrades to the picker when nothing up the chain covers the topic. This is the same upward-only rule search scoping has always used, applied to navigation: a located reader gets the most specific page that exists, and the national guide becomes the floor under every location instead of a page only location-less visitors see.

### D5. The homepage situation list filters by resolved coverage, fetched per location

The known gap from the homepage-location-scope work (2026-08-09) — "Common situations in Boston" could list a topic Boston lacks — is closed by `/api/coverage?j={slug}`: the topic slugs that resolve for that location under D4's walk (`ListTopicsByJurisdictionRecursive`). The scope script fetches it when a location is chosen and hides situations that would dead-end on the picker. The invariant tying D4 and D5 together: a situation is visible for a location if and only if clicking it lands on a real guide.

Why an endpoint and not the alternatives that were already ruled out: personalizing the cached HTML server-side leaks one reader's city to the next through the shared cache, and shipping a full location×topic map with the page grows it with every city. One small JSON response per chosen location, served under the same public 300s cache (keyed by the `?j=` query string), costs neither. Any fetch failure leaves the list unfiltered, which is exactly the pre-D5 behavior. English-only, like the pages that consume it (ADR-007 D2).

### D6. The drafting pipeline speaks "jurisdiction", not "city"

The tool contract renames `city_slug` → `jurisdiction_slug` across `find_sources`, `save_draft_playbook`, `list_topics`, and `get_playbook`; `list_jurisdictions` now returns every authorable jurisdiction with its kind instead of cities only, which is what actually blocked the batch worker from targeting `united-states` (the store accepted it all along). All clients are schema-driven (the batch worker's tool defs and the MCP front-end both derive from the same structs), so there is no wire-compat shim. `cmd/draft` takes `-jurisdiction`, keeping `-city` as an alias.

### D7. Search scope pickers stay city-only, deliberately

Scoping is recursive upward only, so a selectable "United States" option would search national pages *exclusively* — it would exclude the reader's own city, the opposite of what the label promises. The empty option ("Every location") already means everywhere, and a city scope already includes state and national content via the ancestor expansion. This re-affirms the 2026-08-09 decision; nothing about national pages changes it.

## Consequences

- Publishing a national playbook makes it appear in the sitemap, `/locations`, its topic hubs, city-scoped search results, and (via D4) under every located reader whose city lacks the topic. No further wiring per page.
- The national page's "in your state or city" section (the existing other-jurisdictions cross-link, relabeled for country pages) is the router that hands no-location search traffic to located guides.
- State hubs with statewide playbooks enter the sitemap for the first time, as a side effect of D2.
- A topic covered only nationally now redirects located readers to the national guide rather than showing them the picker; if a state or city guide is published later, the same URL starts landing on it with no change anywhere.
