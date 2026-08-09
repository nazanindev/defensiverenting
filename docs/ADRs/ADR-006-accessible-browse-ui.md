# ADR-006 — Accessibility and the browse UI

| | |
|---|---|
| Status | Accepted |
| Date | 2026-08-09 |
| Shipped in | `fa48000` |

## Context

Someone reaches this site because something has gone wrong where they live. They are stressed, often on a phone, often on a slow connection, sometimes reading in a second language, sometimes with the text zoomed. ADR-002 already chose server-rendered HTML partly on that basis. This ADR records the decisions that make the browse surfaces themselves usable for that reader.

Two problems forced the pass.

**The homepage did not scale.** It rendered one card per city. Coverage and page weight grew together: against a 48-city dataset it emitted 65 boxes before any content. With 3 cities live and 25 drafts in review, this was weeks from becoming the primary complaint.

**The middle of the URL tree was empty.** ADR-005 D2 established `/j/{state}/{city}/{topic}`, but `/j/{state}` listed nothing and never linked to the cities beneath it. `/j/massachusetts` said "No playbooks available for Massachusetts yet" while Boston sat directly under it. That is *why* the homepage had to enumerate every city: nothing else did.

An audit of the stylesheet at the same time found the type system had never actually been set:

| | before |
|---|---|
| `body` `font-size` | not set (browser default, 16px) |
| `body` `line-height` | not set (browser default, ~1.2) |
| smallest text on the site | `.78rem` — 12.5px citation chips |
| muted text (`--ink-soft`) contrast | **4.70:1** — passes AA by 0.2 |
| focus rings | removed by `outline: none` on inputs, selects, buttons |

The muted text is the notable one. `rgba(30,18,21,0.60)` on the paper background computes to 4.70:1, which clears the 4.5:1 AA floor and was being used for 12.5px text on a legal-information site.

## Decision

### D1. Location is a scope on the search, not a browse axis

Cities are `<option>` values on a `<select name="j">` attached to the search box, grouped into `<optgroup>` by state. The homepage never enumerates cities.

This is the decision the rest depends on. A selector is O(1) in page weight regardless of coverage; a card grid is O(n). It also matches how the reader thinks: they have a situation, and their city is context for it, not a separate thing to browse.

Cities are the only selectable options. Search scoping walks the hierarchy *upward* (Boston → Massachusetts → federal), so scoping to a state would search state law only and silently exclude the city the reader lives in. `<optgroup>` labels are non-selectable natively, which is exactly the required behaviour.

### D2. Every browse surface is a list, not a card grid

`.card-grid` and its supporting classes were deleted. Topics render as `.situation-list` rows; places render as `.city-list` / `.state-index`.

Cards gave every item equal visual weight and a fixed footprint, which is what made the wall a wall. Lists stay scannable as the vocabulary grows and read top-to-bottom, which is how someone matching their own situation actually reads. The same grid was also appended to the bottom of every published guide as "*Topic* in other cities" — one box per city under each page.

### D3. `/j/{state}` lists its cities; `/locations` is the crawlable directory

The state hub now lists the cities beneath it, closing the D2-shaped hole in ADR-005's tree. `/locations` carries the full state-grouped city directory the homepage no longer does, and is in `sitemap.xml` for that reason — it is now the page that spreads link equity to city hubs.

### D4. The type scale is anchored to a percentage root, with a 15px floor

`html { font-size: 106.25% }` — a percentage, never a px value. This scales the whole rem-based system up about 6% while still inheriting whatever base size the reader set in their browser. A px value there silently overrides that preference and fails WCAG 1.4.4.

`body` is now `1rem / 1.6`. Nothing on the site renders below `.88rem` (15px). Guide text — the most-read content — is `1.15rem / 1.75` (19.5px).

### D5. Headings are sentence case, not letter-spaced capitals

`.section-title` was `text-transform: uppercase` with tracking. All-caps removes the word-shape cues readers use to recognise words without decoding them, and is measurably slower to read. That is the wrong trade for someone reading in a second language or under stress. Hierarchy is carried by size, weight, and typeface instead.

### D6. The whole palette clears AAA (7:1), not AA

`--ink-soft` moved from `0.60` to `0.72` alpha. Measured against the `#fdf8f8` paper background:

| | ratio | |
|---|---|---|
| body text (`--ink`) | 17.32:1 | AAA |
| muted text — **was** `0.60` | 4.70:1 | AA |
| muted text — **now** `0.72` | **7.11:1** | AAA |
| links (`--accent`) | 7.43:1 | AAA |
| citation chips (5 variants) | 8.05–12.75:1 | AAA |
| disclaimer box | 9.21:1 | AAA |

AAA is the target rather than AA because the content is legal information that people act on, and because a meaningful share of readers are on phone screens in daylight. The ratios are computed, not eyeballed; see *Verification* below.

### D7. Focus is always visible, and colour is never the only cue

Several controls set `outline: none` for appearance. That is fine with a mouse and leaves the site unusable with a keyboard. A 3px `:focus-visible` ring is restored on every focusable element — `:focus-visible` rather than `:focus` so it does not fire on mouse clicks.

Links inside running prose are underlined, not distinguished by colour alone (WCAG 1.4.1).

### D8. Touch targets clear 44px; line length is capped

List rows are the primary navigation on a phone and sit close together, so at narrow widths every row is at least 44px tall (WCAG 2.5.5, above the 24px 2.5.8 minimum).

Guide text is capped at 70ch and ledes at 62ch. Past roughly 75 characters the eye loses its place on the return sweep.

### D9. Motion is optional

Each statement card runs a 600ms `fadeUp`; on a long guide that is a lot of movement. A `prefers-reduced-motion: reduce` block collapses animations and transitions (WCAG 2.3.3).

### D10. Enhancement is never load-bearing

The location selector submits with the search as the `j` query param the handler already understood. No JavaScript is required for any of it: with JS off the selector still submits, the situation rows still link to the topic hub, and `/locations` is still a complete list of plain links.

JavaScript adds exactly two things — remembering the chosen location, and pointing the situation rows at it.

### D11. Personalisation is client-side because the cache is shared

Browse routes are served `Cache-Control: public, max-age=300`. A cookie-scoped or server-personalised homepage would be stored by a shared cache and served to the next reader, leaking one person's city to another. The remembered location therefore lives in `localStorage` and is applied after load. The cached HTML is identical for everyone.

### D12. Scope resolution happens on the server, at `/t/{topic}?j={city}`

When a location is set, situation rows carry `?j={city}`. The topic hub resolves it: redirect to the city's guide if that city publishes the topic, otherwise render the picker.

The alternative was resolving it in the browser, which requires shipping a city × topic coverage map to every visitor purely to avoid linking at a 404. That map is 64 entries today and 10,000 at 500 cities. Doing it server-side costs one already-loaded list and degrades to a real page in every case. `302`, not `301`: coverage grows, so the mapping must not be cached in browsers permanently.

### D13. Rejected

- **A custom combobox for locations.** A real `<select>` keeps native type-ahead, keyboard behaviour, and the OS picker on mobile. Every one of those would have to be rebuilt, and rebuilt correctly, for zero gain at present scale.
- **Keeping cities on the homepage behind a "show all" toggle.** Still O(n) in page weight, and hides content from crawlers and from anyone without JS.
- **State cards on the homepage, drilling down to cities.** Bounded at 51, but adds a click for every city visitor — the common case — to solve a problem the scope selector solves with none.
- **Bumping every `font-size` by hand.** Fragile and it drifts. One root percentage scales the system and respects reader preference.

## Verification

Contrast ratios are computed from the shipped hex values with the WCAG relative-luminance formula, not sampled by eye. Scaling behaviour was verified against a seeded 48-city / 16-state / 512-playbook dataset rather than the 3 cities currently live, because the failure mode is a scale problem and 3 cities cannot show it.

Result at that scale: homepage boxes 65 → **17**, page weight flat at ~14KB, all 48 cities reachable through 16 optgroups.

Handler tests cover the invariants that are easy to regress silently: `TestIndex_citiesAreScopeOptionsNotCards` fails if a per-city card returns to the homepage, and the topic-hub tests assert that an uncovered or bogus `?j=` falls through to the picker with a 200 rather than redirecting into a 404.

## Consequences

- Adding a city changes the homepage by one `<option>`. It previously added a card.
- `.card-grid` is gone. Reintroducing a card grid on a browse surface contradicts D2.
- Anything that personalises a browse route server-side contradicts D11 and will poison the shared cache. If that becomes necessary, the caching header has to change first.
- The 15px floor and AAA palette are project defaults now. New components inherit them from the tokens; new hard-coded sizes or colours should be checked against D4 and D6.
- `--ink-soft` got darker, so muted text across every page is more prominent than before. That is intended.

## Open questions

- **The situation list is not filtered by the selected location.** `ListPublishedTopics` returns topics published anywhere, so with a location set the list can include a topic that city does not cover. It degrades to the picker rather than a 404 (D12), but the heading still implies coverage that may not exist. Fixing it properly needs either a per-city topic query — which D11 rules out on the cached homepage — or the coverage map D12 rejected. Deferred deliberately.
- **`.city-badge` and `.sidebar-label` remain uppercase.** They are short standalone labels where caps read as a badge convention, so D5 was not applied to them. Worth revisiting if the flag ever reads as inconsistent.
- **Independent audit.** All of the above is measured against WCAG programmatically. None of it substitutes for testing with an actual screen reader, or with renters who use one.
