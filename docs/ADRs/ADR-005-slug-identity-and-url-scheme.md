# ADR-005 — Jurisdiction hierarchy, slug identity, and the public URL scheme

| | |
|---|---|
| Status | Proposed |
| Date | 2026-08-01 |
| Revised | 2026-08-09 — supersedes the flat-URL and USPS-suffix decisions of the first draft |

## Context

Public URLs are built from two slug namespaces: jurisdictions (`/j/{slug}`, `/j/{slug}/{topic}`) and topics (`/t/{slug}`). Slugs are the only public identifiers: canonical tags, JSON-LD breadcrumbs, the sitemap, internal cross-links, the search `j` filter, and the drafting MCP toolbelt all key on them. The site is live on renterlaw.org with 19 published playbooks across Boston, Pittsburgh, and Seattle (indexed since 2026-07-05), and 26 pilot drafts awaiting review for five more cities.

On 2026-08-01 we shipped our first slug incident cleanup: early drafting runs created per-city topic slugs (`pittsburgh-cant-pay-rent`, `seattle-*`) instead of reusing shared topics. That fragmented topic hubs and cross-city links, and unwinding it required hand-verified production surgery (`scripts/2026-08-01-merge-city-prefixed-topics.sql`) plus a rejection guardrail in `save_draft_playbook`.

A code audit prompted by that incident found the root cause was still live, and that the same class of failure was waiting in the jurisdiction namespace. The general lesson, restated as an invariant:

> **No string typed by an agent or a form may become a public identifier. Identifiers are either resolved against a registry or constructed by code with an enforced uniqueness rule.**

The verbatim-citation guardrail already applies this philosophy to legal claims. This ADR applies it to URLs.

### What has already shipped (2026-08-08)

Three of the original findings were live defects independent of the URL-shape question, and are fixed:

1. **Both authoring form paths fabricated city-prefixed topic slugs** (`citySlug + "-" + topicKey`). The 2026-08-01 guardrail only covered the MCP agent path, so the authoring UI — including the edit form, where a reviewer spends their time — could recreate the incident. Both paths now share `resolveTopic()`, which uses the shared topic key and rejects a city-prefixed custom slug the same way `internal/drafting` does. Covered by tests in `cmd/authoring/topic_test.go`.
2. **`UpsertTopic` silently renamed shared topics site-wide** via `ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name`. Any save with a different display name renamed that topic on every city page. It is now a no-op update; renaming is an explicit editorial action.
3. **Display names were invented by `slugToTitle`**, producing `"Boston Security Deposits"`. Names now come from a `knownTopics` map that mirrors the form dropdown.

Still open from the original audit, and addressed below: silent-clobber `UpsertJurisdiction`, the absence of any rename mechanism, the redundant `playbooks.slug` column, and the seeds registry keyed by bare city slugs.

### What the production data actually shows

The URL decision was originally made against an assumed inventory. The real one:

| City | Parent | Published |
|---|---|---|
| Boston | **none** | 6 |
| Seattle | **none** | 6 |
| Pittsburgh | pennsylvania | 7 |

States `california`, `illinois`, `pennsylvania`, `texas` have no parent country; only `massachusetts` points at `united-states`. `washington` does not exist despite Seattle being published. `austin`, `chicago`, and `los-angeles` exist as empty rows with no playbooks.

**The jurisdiction hierarchy is half-null.** Any decision that puts the parent state in the URL is blocked on repairing it first.

Topic display names in production are `slugToTitle` output and are visible to renters: `"Cant Pay Rent"` (no apostrophe), `"Notice To Quit"` (capital T). They predate the fix above, which deliberately does not overwrite existing names.

### The scaling problem this ADR actually has to solve

Most tenant law is **state** law. Deposit limits, notice periods, the implied warranty of habitability — those are set by the state. Only a minority of cities (New York, San Francisco, Los Angeles, Chicago, Seattle, Portland, Washington DC) have ordinances that meaningfully change the answer.

Scaling city-by-city with self-contained playbooks means Austin, Houston, Dallas, and San Antonio each carry a `security-deposits` page restating the same Texas statute. At 100 cities that is roughly 700 playbooks, mostly duplicated, which fails in three ways:

- **Cost** — ~700 drafting runs plus 700 human reviews.
- **Freshness** — Texas amends one statute and four-plus pages need re-verification. `sourcecheck` flags the same legal change once per duplicate.
- **Search** — four near-identical pages competing with each other.

The URL scheme is downstream of fixing this. That is the reasoning the first draft of this ADR was missing.

### Slug touchpoint inventory

Any change to slug rules must cover all of:

| Surface | Location |
|---|---|
| Browse routes | `internal/http/handlers/browse.go` |
| Canonical URLs, JSON-LD Article + breadcrumbs | `browse.go` (`BuildPlaybookPage`, `playbookSchema`) |
| Sitemap | `ListSitemapURLs`, `handlers/meta.go` |
| Search jurisdiction filter (`?j=`) | `handlers/search.go`, hidden input in `playbook.html` |
| MCP toolbelt (`city_slug`, `topic_slug` on every tool) | `internal/drafting/tools.go` |
| Seeds registry keys | `internal/discover/seeds.go` |
| Authoring UI | `cmd/authoring/main.go` |
| Ingest CLI | `cmd/ingest/main.go` |
| Live indexed URLs (19, rising to 45 after the pilot batch publishes) | renterlaw.org sitemap |

## Decision

### D1. State law is written once; city pages carry the delta

The state playbook is the primary document for a topic. A city playbook exists only where a local ordinance changes the answer, and it addresses the difference rather than restating the state rule.

This is what makes 100 cities affordable: most new cities become a thin ordinance page over an existing state page, not a seven-topic drafting run. It also collapses the freshness problem — a state statute change touches one page.

### D2. URLs are hierarchical: `/j/{state}/{city}/{topic}`

```
/j/texas                              state hub
/j/texas/security-deposits            state playbook — the substance
/j/texas/austin                       city hub
/j/texas/austin/security-deposits     city delta
/t/{topic}                            cross-jurisdiction topic hub (unchanged)
```

The hierarchy is real — it is how tenant law is actually layered — and D1 makes the state segment point at primary content rather than a directory stub. A reader arriving from search sees immediately which state's law governs.

`/j/{a}/{b}` is ambiguous in shape between a state playbook and a city hub, and is resolved with one indexed lookup on segment two. This is a real cost and was the first draft's reason for rejecting hierarchy. It is accepted here because the lookup happens only on two-segment paths, adds a sub-millisecond indexed query to a request that already hits the database, and buys D3.

### D3. City slugs are bare and unique within their parent state

`chicago`, `austin`, `portland` — no state suffix. Uniqueness is scoped to the parent, so Portland OR and Portland ME are `/j/oregon/portland` and `/j/maine/portland` with no disambiguation needed. A duplicate city name within one state is an error, not an upsert.

This supersedes the first draft's `{toSlug(name)}-{usps}` scheme. That scheme solved collisions by construction, but at a maintenance cost the hierarchy removes entirely. The two options differ in what they ask a human to maintain forever:

- **Suffixes** require judging "is this city name ambiguous with another US city?" for every new city, forever, using geographic knowledge, where a miss is silent until the second Springfield arrives and forces the rename of an indexed URL.
- **Hierarchy** requires one invariant: no topic slug may equal a city slug, or `/j/{state}/{b}` becomes genuinely ambiguous. That is a curated set of ~15 topics, enforceable by a database constraint and a CI test.

A constraint a test can hold beats a list a human has to remember. At ten cities the difference is invisible; at a hundred it is decisive.

`new-york-city` keeps its slug: New York State is `new-york`, and under D2 the city sits at `/j/new-york/new-york-city`. Washington DC is a city whose parent is itself; it gets an explicit row rather than a special case in code.

### D4. Renames become safe via a permanent alias table

```sql
CREATE TABLE slug_aliases (
    alias           TEXT   NOT NULL,
    namespace       TEXT   NOT NULL CHECK (namespace IN ('jurisdiction', 'topic')),
    jurisdiction_id BIGINT REFERENCES jurisdictions(id) ON DELETE CASCADE,
    topic_id        BIGINT REFERENCES topics(id)        ON DELETE CASCADE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (namespace, alias),
    CHECK ((namespace = 'jurisdiction' AND jurisdiction_id IS NOT NULL AND topic_id IS NULL)
        OR (namespace = 'topic'        AND topic_id IS NOT NULL AND jurisdiction_id IS NULL))
);
```

Two nullable foreign keys rather than one polymorphic `target_id`, so the
database guarantees an alias never outlives what it points at — a dangling
alias would redirect to a 404, which is worse than the 404 it prevents.

Aliases store the target's **id**, not its slug, so repeated renames need no
extra bookkeeping and every lookup stays a single hop.

Every rename inserts the old slug as an alias. Browse handlers fall back to an alias lookup on miss and reply `301` to the canonical URL. `GetJurisdictionBySlug` resolves aliases too, so in-flight agent sessions and stale saved prompts keep working. Code enforces that an alias never shadows a live slug.

**This is the load-bearing decision in this ADR.** It is what makes D2 and D3 adoptable at all, it retroactively fixes the `/t/pittsburgh-discrimination`-era 404s left by the topic cleanup, and it is what makes any future slug mistake survivable. It should ship first and independently.

### D5. `topics` becomes a closed registry

Topics are currently defined by two hardcoded Go lists that disagree: `internal/draftagent/topics.go` (`CoreTopics`, 5 entries, used by the AI batch) and `cmd/authoring/main.go` (`knownTopics`, 13 entries, used by the form). Neither is checked against the `topics` table, and `UpsertTopic` creates whatever either one supplies. That is how two vocabularies for the same subjects came to exist.

The drift was not carelessness. `list_topics` — the tool the rejection message tells the agent to call — routes to `ListTopicsByJurisdiction`, which filters to `status = 'published'` **for that city**. Drafting a brand-new city returns an empty list, so the agent has no registry to reuse at exactly the moment it matters.

Therefore:

- The canonical topic set is seeded by migration. `topics` gains `is_core BOOLEAN`; core topics are seeded for every new city, non-core are added where the law justifies them.
- Save paths use `GetTopicBySlug` (which already exists) instead of `UpsertTopic`. An unknown slug is a rejection, not a creation. Creating a topic is a deliberate act, separate from saving a playbook.
- `list_topics` returns the global registry, not the per-city published set.
- Both Go lists are deleted.
- The seed migration also corrects the `slugToTitle`-generated display names now live in production.

**Canonical set.** Core (seeded everywhere): `cant-pay-rent`, `eviction-defense`, `repairs-and-habitability`, `security-deposits`, `landlord-entry`, `rent-increase`, `resource-directory`. Non-core: `heat-not-working`, `rent-stabilization`, `discrimination`, `move-in-checklist`, `move-out-checklist`, `lease-renewal`, `noise-complaints`, `renting-fundamentals`.

`heat-not-working` is deliberately kept rather than merged into `repairs-and-habitability`. It is problem-shaped, it matches how a renter actually searches, and it has three published pages whose URLs stay put as a result.

### D6. Creation paths are hardened so collisions error instead of clobbering

- `UpsertJurisdiction` is replaced by get-or-create: reuse only when kind, name, and parent match; any mismatch is an error. Blind `ON CONFLICT ... DO UPDATE` on jurisdictions is removed. Today, creating Portland, Maine while Portland, Oregon exists silently rewrites the existing row and reassigns every Portland OR playbook.
- New-city creation requires a parent state. Under D2 a city without a parent has no URL at all, so this stops being advisory.
- `playbooks.slug` is dropped. Routing keys on `topics.slug`; the playbook's own slug column is written but never read, and can silently diverge.

### D7. Migration order — each step independently deployable

1. **`slug_aliases` + alias-aware lookups + 301 fallback.** No visible change. Ship alone.
2. **Repair the hierarchy.** `boston → massachusetts`, create `washington` and attach `seattle`, attach every state to `united-states`. Verified against production before and after. Prerequisite for step 4.
3. **Topic registry (D5)** — seed migration, creation-path changes, `list_topics` fix, display-name corrections.
4. **URL migration**, hand-verified against production first, per the precedent set by the topics cleanup. Adds the state segment and renames topics **in one pass**, writing an alias for every old URL: `security-deposit-not-returned → security-deposits`, `landlord-entry-without-notice → landlord-entry`, `uninhabitable-conditions → repairs-and-habitability`, `notice-to-quit → eviction-defense`. `cant-pay-rent`, `heat-not-working`, `rent-increase`, `discrimination` keep their topic slugs.
5. **Re-key `internal/discover/seeds.go`** and the `city_slug` examples in the MCP tool schemas.
6. **Verify**: sitemap emits only new URLs; every old URL 301s; topic hubs and cross-city links intact; a draft agent round-trips `list_jurisdictions → save_draft_playbook`.

**Timing.** The pilot batch is reviewed but unpublished. Migrating before it publishes moves 19 URLs; after, 45. Steps 1–4 therefore run in parallel with review, and publication waits for them. Coordinate step 4 with the renterlaw.org rebrand window so search engines absorb one redirect generation, not several.

**Boston and Seattle each have an `eviction-defense` draft sitting alongside a published `notice-to-quit`.** Renaming the latter collides with the former, and `UNIQUE (jurisdiction_id, topic_id, language)` permits only one. Each pair needs an editorial comparison — publish the draft and retire the old page, or discard the draft and rename the live one. This is a content judgment and belongs in the review queue, not in the migration script.

### D8. Rejected

- **Uniform USPS suffixes** (`chicago-il`). Solves collisions by construction, but D2+D3 solve them structurally without permanently lengthening every URL, and the maintenance asymmetry in D3 favours hierarchy.
- **Suffix only on collision.** The first draft rejected this because it forces renames after indexing. That objection was sound when written and is now moot — D4 makes renames safe — but D2 removes the need entirely.
- **Dropping the `/j/` prefix.** Would require reserving every current and future top-level page name as a forbidden slug, forever.
- **Flat URLs with a curated disambiguation list.** Viable, and cheaper today. Rejected on the maintenance asymmetry argued in D3.

## Consequences

- 19 live URLs move once, with permanent 301s. Short-term Search Console churn; long-term a scheme that never needs another structural change.
- The hierarchy must be repaired before step 4 can run at all. This is a latent data-integrity problem being forced into the open, which is a benefit.
- `/j/{a}/{b}` costs one extra indexed lookup, on two-segment paths only.
- A new invariant to enforce: topic slugs and city slugs must be disjoint. Enforced in the database and in CI, not by convention.
- Silent-clobber upserts become loud errors in both the jurisdiction and topic namespaces.
- In-review drafts are unaffected — playbooks reference `jurisdiction_id`, not slugs — and in-flight agent sessions survive via alias resolution.
- D1 changes what gets drafted, not just how it is addressed. The drafting pipeline needs a state-playbook mode, and the review queue will contain state pages as well as city pages.

## Open questions

1. Does a city hub (`/j/texas/austin`) list only that city's delta pages, or does it also surface the state topics it inherits? Inheriting reads better for renters; it needs care to avoid duplicate content.
2. When a city has no local ordinance for a topic, does `/j/texas/austin/security-deposits` 404, or 301 to the state page? A 301 is friendlier and captures long-tail city queries.
3. Washington DC, and any future consolidated city-county, need a jurisdiction that is its own parent. Special-case row or a `kind = 'city_state'`?
4. Does `renting-fundamentals` stay a `united-states` playbook, or become the template every state page starts from?
