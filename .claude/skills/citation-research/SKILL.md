---
name: citation-research
description: Boundaries for researching and sourcing citations across find_sources, fetch_source, courtlistener, and brave-search/WebSearch. Use when researching primary sources, hunting case law, verifying a citation, backfilling a missing quote, or working the citation coverage gap. Triggers - research, source, cite, citation, case law, court ruling, courtlistener, verify quote, fetch_source.
---

# Citation research

This governs interactive research and citation work in Claude Code — verifying
citations, backfilling missing quotes, chasing case law, or auditing sources by
hand. It does not apply to the autonomous `cmd/draft` worker: that loop only has
`web_search` + the `defensiverenting` toolbelt today; courtlistener and
brave-search are not wired into it, and that's a separate follow-up, not
something this skill changes.

## The one rule: discovery tools never supply a quote

`mcp__defensiverenting__save_draft_playbook` (and the authoring form's manual
quote check) both enforce the same trust anchor: a citation's `quote` must be a
verbatim, whitespace-normalized substring of text that
`mcp__defensiverenting__fetch_source` actually fetched. Nothing else counts,
no matter how authoritative it looks.

That makes every other research tool **discovery-only** — it finds a URL or a
case, never a quotable fact:

- `mcp__defensiverenting__find_sources` — curated, vetted primary-source seeds
  for a city (statutes, ordinances, gov guidance). Start here for "what does
  the law say" statements.
- `mcp__courtlistener__search` / `search_document` / `get_endpoint_item` /
  `get_endpoint_schema` / `call_endpoint` — case-law discovery: finding an
  opinion, its citation, its court, its docket.
- `mcp__courtlistener__read_document` — returns full opinion text directly.
  **Never treat this text as a citation quote.** It bypasses `fetch_source`
  entirely, so a quote pulled from it either gets rejected outright or, worse,
  happens to match and slips through unverified against the real page.
- `brave-search` (once connected — see below) and Claude Code's own
  `WebSearch` / hosted `web_search` — general web discovery, same rule.

**The only path to a quotable citation:** discover a URL → run it through
`fetch_source` → quote from what that call returns. This holds even when the
URL came from courtlistener or a search result you're confident about.

## When to reach for which

1. **`find_sources` first**, for anything jurisdiction-specific and
   statutory/regulatory. It's pre-vetted for this project; don't re-derive it
   with a general search.
2. **`courtlistener`**, specifically for statements about how a court has
   applied, interpreted, or enforced a law — not as a substitute for a
   statute citation. A playbook's baseline legal claims should still cite the
   statute or ordinance; case law backs statements like "courts have ruled
   that..." or describes a remedy actually won.
3. **`brave-search` / `WebSearch`**, for open discovery when `find_sources`
   has no seed and it isn't a case-law question — e.g. a nonprofit's own page
   for `resource-directory`, or an ordinance `find_sources` doesn't carry yet.
4. Never cite a search snippet or an AI-generated summary of a case as fact.
   Treat every hit from any of these tools as a lead, not a source, until it's
   been through `fetch_source`.

## Citing case law once fetched

Use `kind: "court_ruling"` and put the case citation in `locator` (e.g.
`"410 U.S. 113"`, or the docket number if no reporter citation exists yet).
Fetch the opinion's own canonical page — its CourtListener permalink
(`absolute_url`) or the official court/reporter link if one exists — so the
citation points somewhere a reviewer can independently re-check.

## Settled questions (do not flag these again)

Decided 2026-09-17 from the queue triage pass. Drafting agents apply them
instead of leaving a reviewer note.

- Archived or aspirational executive-branch documents (the White House
  Renters Bill of Rights Blueprint on bidenwhitehouse.archives.gov, and the
  like) are not sources. They are not law and not current agency guidance.
  Cite CFPB, HUD, USAGov, or the state statute instead, and drop any
  sentence that rests only on such a document.
- A university legal clinic's tenant guidance (Cornell's Tenants Advocacy
  Program, a law-school housing clinic) counts as a nonprofit legal-aid
  source, the same as a legal-aid organization's own site. Attribute the
  reading in the body when it is a judgment call ("a legal aid program
  reads reasonable notice as...").
- A city-only rule may stand on a state page when the sentence names the
  city ("In New York City, ..."). Whether the city guide should also carry
  it is drafting work for that page, not a doubt about the statement.
- Tag with the registry as it reads: the rent-control concept covers rent
  stabilization too. A topic_ref must point at a topic that has pages; if
  none exists, leave the tag off and send the reader to the guide by name.
- Where a statement sits on the page (help lines, advice) is a page-region
  question (ADR-016), not a reason to note the statement.

## Setup note

`courtlistener` is already connected project-scoped (http, no key needed).
`brave-search` is not yet configured in this project — it needs adding to
`.claude.json` (typically an API-keyed stdio or hosted MCP entry) before its
tools appear. Until then, fall back to `WebSearch` for open discovery.
