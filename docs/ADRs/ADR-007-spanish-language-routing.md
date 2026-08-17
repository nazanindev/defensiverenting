# ADR-007 — Spanish-language URL routing

| | |
|---|---|
| Status | Accepted |
| Date | 2026-08-16 |

## Context

The drafting toolbelt and the authoring form (this same date, see git history) can now produce and edit `language: "es"` playbooks end to end. The store layer was already fully ready for this — `playbooks.language`/`statements.language` carry no CHECK constraint, and migration 000012 already maps `es` to the `spanish` full-text-search config. What was missing was routing: every browse handler hardcoded `"en"`, so a published Spanish playbook has no public URL.

The goal driving this decision, stated directly: **someone searching their question in Spanish on Google should be able to find and land on a helpful page.** That goal, not general-purpose i18n completeness, is what the rest of this ADR optimizes for.

### Why a query parameter is not on the table

`internal/http/middleware/cache.go` serves every browse route `Cache-Control: public, max-age=300`. A shared cache generally keys on path, not query string, so `?lang=es` risks silently serving the cached English page to a Spanish request — the same class of problem the homepage-location-scope decision (2026-08-09) already ruled out for personalizing browse HTML server-side. Query-param language variants are also weaker for search-engine indexing than path-based ones. A subdomain (`es.renterlaw.org`) is real infrastructure — DNS, TLS, and `CanonicalHost` currently assumes one host — disproportionate for a two-language site today.

## Decision

### D1. Bare English stays unprefixed; Spanish gets a leading `/es`

`/j/{state}/{city}/{topic}` and `/t/{topic}` are unchanged. Their Spanish counterparts are `/es/j/{state}/{city}/{topic}` and `/es/t/{topic}`, registered as literal routes (not a `{lang}` wildcard) so an unsupported code 404s rather than silently resolving. This mirrors the `resolveLanguage("") == "en"` default already used everywhere else in this codebase (`drafting.ResolveLanguage`, `voice.Supported`).

### D2. Only jurisdiction/topic content routes get a language dimension

`/j/*` and `/t/*` (the pages that actually carry translated legal content) get `/es` variants. `/`, `/search`, `/locations`, and the static pages (`/about`, `/support`, `/editorial`, `/report`, `/contact`, the author bio) stay English-only.

This is not laziness — it is what D0's stated goal actually needs. A Spanish Google query lands on a specific playbook page, never the homepage; translating site chrome would be a distinct, unscoped site-i18n project that does nothing for search discoverability of the content itself.

### D3. A missing translation 404s; it is never faked by rendering English at an `/es/` URL

`/es/j/boston/eviction-defense` before that topic has a Spanish draft returns a plain 404. Rendering the English text at that URL instead would be near-duplicate content at two URLs, which search engines are likely to canonicalize back to the English page rather than index separately — it would not add a Spanish-rankable page, and risks diluting the English page's own ranking. Every internal link this site renders to a specific `/es/...` page is built only after confirming that page exists (the same query that lists sibling topics/cities already filters by language, so this falls out for free — see D5), so a reader never reaches this 404 by clicking a link on the site, only by guessing or bookmarking a URL.

### D4. `Jurisdiction.Path()`/`TopicPath()` stay language-agnostic; a thin prefix wraps them

`store.LangPrefix(lang)` returns `""` for English and `"/"+lang` otherwise. `Jurisdiction.PathIn(lang)` / `TopicPathIn(lang, topicSlug)` compose it with the existing, already-centralized path builders rather than duplicating their state/city logic. Every URL the site emits for language-scoped content — canonical tags, JSON-LD, sitemap entries, redirect targets, template links — goes through these two functions, continuing the guarantee `Jurisdiction.Path()`'s own doc comment already makes: the URL shape lives in exactly one place.

### D5. Sibling/cross-link queries are parameterized by the *page's* language, not hardcoded "en"

`ListTopicsByJurisdiction`, `ListJurisdictionsByTopic`, and `GetPlaybook` already accept a `language` argument at the store layer (added when the toolbelt was made language-aware); browse handlers were the last hardcoded caller. Threading the page's own resolved language through them means every sibling-topic link, cross-city link, and Local Help link a playbook page renders is automatically scoped to pages that exist in that language — no separate existence check needed beyond what D3 already requires for the language-toggle link itself (see D6).

### D6. A language-toggle link and `hreflang` alternates appear only when the other version actually exists

A playbook page checks once (`GetPlaybook` for the other language) whether a translation exists, and if so renders both a visible toggle link ("Ver esta página en español" / "Read this page in English" — written in the *target* language, the standard convention) and `<link rel="alternate" hreflang="...">` tags, plus `hreflang="x-default"` pointing at the English version when one exists. This is what lets a search engine show the right language variant to the right searcher — the actual mechanism behind D0's goal, not the routes existing in isolation.

### D7. Sitemap emits both languages

`ListSitemapURLs` drops its `language = 'en'` filter and returns each entry's language, so `sitemap.xml` lists every published playbook in whichever language(s) it exists — both, some, or (for now, most) just English. `ListPublishedTopics` is called for both `en` and `es` to emit `/t/{slug}` and `/es/t/{slug}` topic-hub entries.

## Consequences

- Coverage grows by translating more topics (the toolbelt/drafting-agent work), not by anything at the routing layer — routing correctly reflects whatever content exists and no more.
- `<html lang="...">` on `jurisdiction.html`/`playbook.html`/`topichub.html` becomes dynamic, driven by the page's resolved language; every other public template keeps a static `lang="en"`, which was already correct for them.
- Two extra chi routes are registered per content-route pattern (one per supported language via `voice.Supported()`), not one per route forever — adding a third language later is a one-line change to that list plus a `voice` ruleset, not a routing rewrite.
- `search.html`/`index.html`/`locations.html` remain English-only. A Spanish speaker's specific question is still findable by search-engine indexing of the translated playbook page directly; they do not need to discover it by browsing the (English) homepage first.

## Rejected

- **Query-param language** (`?lang=es`) — cache-correctness risk given the existing shared-cache setup, weaker for search indexing. See "Why a query parameter is not on the table" above.
- **Subdomain** (`es.renterlaw.org`) — real infrastructure lift, disproportionate for two languages.
- **Render English at the `/es/` URL with a "not yet translated" banner** — near-duplicate content at two URLs works against the stated SEO goal rather than for it (D3).
- **Translating site chrome now** — a different, unscoped project; would not move the needle on D0's actual goal (D2).
