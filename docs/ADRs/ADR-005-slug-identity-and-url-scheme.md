# ADR-005 — Slug identity, uniqueness, and the public URL scheme

| | |
|---|---|
| Status | Proposed |
| Date | 2026-08-01 |

## Context

Public URLs are built from two slug namespaces: jurisdictions (`/j/{slug}`, `/j/{slug}/{topic}`) and topics (`/t/{slug}`). Slugs are the only public identifiers: canonical tags, JSON-LD breadcrumbs, the sitemap, internal cross-links, the search `j` filter, and the drafting MCP toolbelt all key on them. The site is live on renterlaw.org with published playbooks for Boston, Pittsburgh, and Seattle (indexed since 2026-07-05) and 25 pilot drafts in review for five more cities.

On 2026-08-01 we shipped our first slug incident cleanup: early drafting runs created per-city topic slugs (`pittsburgh-cant-pay-rent`, `seattle-*`) instead of reusing shared topics. That fragmented topic hubs and cross-city links, and unwinding it required hand-verified production surgery (`scripts/2026-08-01-merge-city-prefixed-topics.sql`) plus a rejection guardrail in `save_draft_playbook`.

A code audit prompted by that incident found the root cause is still live, and that the same class of failure is waiting in the jurisdiction namespace:

1. **The authoring UI still fabricates city-prefixed topic slugs.** Both the create flow (`cmd/authoring/main.go:722`) and the draft-edit flow (`cmd/authoring/main.go:983`) derive `topicSlug := citySlug + "-" + topicKey` and invent a display name via `slugToTitle`. The 2026-08-01 guardrail only covers the MCP agent path. Every draft started in the authoring tool recreates the incident.
2. **`UpsertJurisdiction` silently hijacks rows on slug collision** (`internal/store/postgres.go:364`, `ON CONFLICT (slug) DO UPDATE SET name, kind, parent_id`). City slugs are `toSlug(cityName)`, so creating Portland, Maine while Portland, Oregon exists does not error: it rewrites the existing row's name and parent state, silently reassigning every Portland OR playbook, statement, and source to Maine. The second Springfield does not collide; it hijacks the first.
3. **`UpsertTopic` silently renames shared topics** (`ON CONFLICT (slug) DO UPDATE SET name`). Any draft, agent or human, that reuses an existing topic slug with a different display name renames that topic on every city page site-wide.
4. **Bare city-name slugs guarantee future collisions.** Portland (OR/ME), Columbus (OH/GA), Kansas City (MO/KS), Springfield (IL/MA/MO/OH), Arlington (TX/VA), Charleston (SC/WV), Richmond (VA/CA), Aurora (IL/CO), Glendale (CA/AZ). At six cities we have dodged this; at a hundred we will not. Bare slugs are also ambiguous to renters and to search engines ("portland tenant rights" — which Portland?).
5. **There is no rename mechanism.** A slug change breaks live URLs with no redirect. The topic cleanup already caused this: `/t/pittsburgh-discrimination` era URLs now 404.
6. **`playbooks.slug` is redundant.** Routing keys on `topics.slug` (`GetPlaybook`); the playbook's own slug column is written but never read, and can silently diverge.
7. **The discover seeds registry is keyed by bare city slugs** (`internal/discover/seeds.go`), so any slug change must be coordinated there.

The general lesson from the incident, restated as an invariant: **no string typed by an agent or a form may become a public identifier. Identifiers are either resolved against a registry or constructed by code with an enforced uniqueness rule.** The verbatim-citation guardrail already applies this philosophy to legal claims; this ADR applies it to URLs.

### Slug touchpoint inventory

Any change to slug rules must cover all of:

| Surface | Location |
|---|---|
| Browse routes `/j/{j}`, `/j/{j}/{t}`, `/t/{t}` | `internal/http/handlers/browse.go` |
| Canonical URLs, JSON-LD Article + breadcrumbs | `browse.go` (`BuildPlaybookPage`, `playbookSchema`) |
| Sitemap | `ListSitemapURLs`, `handlers/meta.go` |
| Search jurisdiction filter (`?j=`) | `handlers/search.go`, hidden input in `playbook.html` |
| MCP toolbelt (`city_slug`, `topic_slug` on every tool) | `internal/drafting/tools.go` |
| Seeds registry keys | `internal/discover/seeds.go` |
| Authoring UI (select by slug, new-city creation, `?j=` params) | `cmd/authoring/main.go` |
| Ingest CLI | `cmd/ingest/main.go` |
| Live indexed URLs (~25) | renterlaw.org sitemap |

## Decision

### D1. City slugs carry the USPS state code: `{toSlug(name)}-{usps}`

`chicago-il`, `boston-ma`, `portland-or`, `portland-me`. States keep full-name slugs (`illinois`, `massachusetts`); the country row stays `united-states`. A small curated override map is allowed at creation time for cases where the mechanical derivation reads badly (e.g. New York City → `new-york-ny`, not `new-york-city-ny`). Within a state, a duplicate city name is an error, not an upsert.

This is uniqueness by construction, matches the dominant local-SEO convention (Zillow, Apartments.com, Yelp), and disambiguates for renters and crawlers alike.

### D2. Routes are unchanged; a jurisdiction is always one path segment

`/j/{jurisdiction}` and `/j/{jurisdiction}/{topic}` serve cities and states identically — `/j/illinois` and `/j/illinois/rent-increase` route today with zero code changes because slugs are globally unique across the hierarchy. The truly hierarchical alternative (`/j/illinois/chicago/{topic}`) was rejected: it makes `/j/{a}/{b}` ambiguous between a state playbook and a city index, forcing a DB probe to classify segment two on every request, forever. The `/j/` prefix stays: dropping it would require reserving every current and future top-level page name as a forbidden slug, a permanent tax for a cosmetic gain.

### D3. Renames become safe via a permanent alias table

```sql
CREATE TABLE slug_aliases (
    alias     TEXT NOT NULL,
    namespace TEXT NOT NULL CHECK (namespace IN ('jurisdiction', 'topic')),
    target_id BIGINT NOT NULL,
    PRIMARY KEY (namespace, alias)
);
```

Every slug rename inserts the old slug as an alias. Browse handlers fall back to an alias lookup on miss and reply `301` to the canonical URL. The drafting toolbelt's `GetJurisdictionBySlug` resolves aliases too, so agent sessions in flight (and stale saved prompts) keep working instead of erroring. Code enforces that an alias never shadows a live slug. This mechanism is what makes the D5 migration cheap, and it retroactively fixes the `/t/pittsburgh-discrimination`-style 404s left by the topic cleanup.

### D4. Creation paths are hardened so collisions error instead of clobbering

- `UpsertJurisdiction` is replaced by get-or-create semantics: reuse only when kind, name, and parent match the existing row; any mismatch is an error surfaced to the caller. Blind `ON CONFLICT ... DO UPDATE` on jurisdictions is removed.
- New-city creation (authoring UI, ingest CLI) requires a state, derives the slug with a static USPS map (50 states + DC + territories), and refuses duplicates within a state.
- Both authoring flows drop the `citySlug + "-" + topicKey` derivation. The topic picker submits a shared topic slug directly; a custom topic goes through `toSlug` plus the same city-prefix rejection the MCP path has. `UpsertTopic` stops updating `name` on conflict; renaming a topic becomes an explicit editorial action, not a side effect of saving a draft.
- `playbooks.slug` stops being written and is dropped in a later cleanup migration; `topics.slug` is the single source of truth for the topic URL segment.

### D5. Migration order (each step independently deployable)

1. Ship `slug_aliases` + alias-aware lookups + 301 fallback. No visible change.
2. Ship the USPS map and creation-path hardening (D4).
3. Data migration, hand-verified against prod first (per the precedent set by the topics cleanup script): rename `boston→boston-ma`, `pittsburgh→pittsburgh-pa`, `seattle→seattle-wa`, `chicago→chicago-il`, `austin→austin-tx`, `los-angeles→los-angeles-ca`, `philadelphia→philadelphia-pa`, `new-york-city→new-york-ny`, writing an alias for each old slug. Verify parent states exist and are correct while in there.
4. Re-key `internal/discover/seeds.go` to the new slugs; update the `city_slug` examples in the MCP tool schemas (`"boston"` → `"boston-ma"`).
5. Verify: sitemap emits only new URLs; old URLs 301; topic hubs and cross-city links intact; a draft agent round-trips `list_jurisdictions → save_draft_playbook` on a new slug.

Coordinate step 3 with the renterlaw.org rebrand cutover window so search engines absorb one redirect generation, not two.

### D6. Explicitly rejected

- Hierarchical `/j/{state}/{city}` URLs (routing ambiguity, D2).
- Dropping the `/j/` prefix (reserved-word bookkeeping forever).
- "Suffix only on collision": leaves slug style inconsistent forever and forces renames at the worst time — after the colliding city is indexed.

## Consequences

- ~25 live URLs are renamed once, with permanent 301s. Short-term Search Console churn; long-term stable, self-disambiguating URLs.
- In-review drafts are unaffected (playbooks reference `jurisdiction_id`, not slugs). In-flight agent sessions survive via alias resolution.
- Silent-clobber upserts become loud errors; the authoring UI can no longer recreate the topic-fragmentation incident.
- Findings 1–3 in Context (city-prefixed topics in the authoring UI, jurisdiction hijack, topic rename clobber) are live defects worth fixing even if the slug-format decision here is revised.

## Open questions

1. New York's canonical slug: `new-york-ny` (proposed) or mechanical `new-york-city-ny`.
2. Should state USPS codes alias to the full-name slugs (`/j/il` → 301 → `/j/illinois`)? Cheap once `slug_aliases` exists.
3. When state-level playbooks ship, does `ListPublishedCityJurisdictions`/the homepage grow a states section, or do state hubs stay reachable only via directory pages and cross-links?
