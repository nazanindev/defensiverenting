# ADR-010 — Explanatory 404s for uncovered places and topics

| | |
|---|---|
| Status | Accepted |
| Date | 2026-08-19 |

## Context

Every miss served the stock Go one-liner ("404 page not found"). For this site most misses are not broken links — they are readers asking for a place or topic we have not covered yet, which is the expected state of a site whose guides are human-verified and therefore added one at a time. A reader in an uncovered city got nothing: no explanation, no search box, no pointer at the statewide or nationwide guide whose law applies to them (ADR-009).

## Decision

### D1. One styled 404 template, four stories

`notfound.html` renders with a 404 status always — the body is helpful, the code stays honest so search engines drop the URL. Which story it tells depends on what the handler knows:

- **Plain wrong URL** (router fallback, unknown topic slug): "we can't find that page."
- **Uncovered place** (`/j/{unknown}`, or an unknown segment under a known state): "we don't cover this place yet," linking the known parent's hub when there is one. The unknown slug itself is never echoed into the copy — reflected garbage reads as a broken page even though templates escape it.
- **Covered place, missing topic** (`/j/{state}/{city}/{topic}` where the topic is in the registry): names both, and offers the nearest guide up the ancestor chain (`GetNearestTopicJurisdiction`, same walk as ADR-009 D4) with the one-line legal justification: the ancestor's law applies in the place beneath it.
- **Topic not written yet** (`/t/{topic}` with nothing published in the language): "no guides yet," not "no such topic."

Every variant carries the same explanation block: a person researches every guide and checks every citation before publishing, nothing is published without that check, so coverage grows one place and one topic at a time — with a link to `/contact` to ask for a city. This is the honest answer to "why is my city missing," and it is also the site's editorial promise restated at the exact moment a reader would otherwise wonder if the site is abandoned.

### D2. The missing-topic 404 links the ancestor guide; it does not redirect to it

`/t/{topic}?j={city}` redirects upward (ADR-009 D4) because that URL is a resolver, not a page. `/j/{state}/{city}/{topic}` is the canonical address of a page that may exist later; a redirect cached by browsers or the shared cache would keep stealing that page's traffic after it publishes. So the canonical URL 404s with a link, and starts serving the real page the moment one is published.

### D3. Disambiguating `/j/{state}/{b}`

Two-segment URLs are ambiguous between a state topic and a city. When `b` is neither a known city nor a registry topic (nor a retired alias of one), it is treated as an uncovered place under the state — a reader guessing their city's URL is far more common than one guessing a topic slug that never existed.

### D4. The router fallback is cached like any browse page

chi's NotFound handler runs outside the browse middleware group, so `StaticCache` wraps it explicitly: a 404 for a crawler storm of junk URLs is exactly the response a shared cache should absorb. The `/api/coverage` endpoint keeps its plain-text 404 — its consumer is the scope script, not a person.

## Consequences

- Uncovered-place traffic lands on a page that explains the model, offers the ancestor hubs, and carries a search box — instead of eleven bytes of plain text.
- The "ask us to cover your city" link turns dead ends into expansion signal via the contact form.
- The explanation block's wording lives in one template; if the editorial promise changes, one file changes.
