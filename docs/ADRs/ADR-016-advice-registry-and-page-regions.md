# ADR-016: Practical advice is a registry pages reference, and the page has a fixed set of regions

| | |
|---|---|
| Status | Proposed (amended 2026-10-09) |
| Date | 2026-09-08 |
| Amends | ADR-003 (a second kind of statement-shaped content that is not a citation), ADR-011 (registry pattern reused), ADR-014 D2 (proposals lose the editorial path) |

## Context

Drafts keep arriving with statements like "write everything down", "keep a copy of every letter", "take dated photos before you move in". They are good advice. They are not law. They are the same in every jurisdiction. And every one of them carries a citation to whichever legal aid page happened to say it, because the drafting tool leaves the agent no other move: `save_draft_playbook` rejects a statement with no citation, and every citation must be a fetched URL with a verbatim quote. The agent is doing exactly what the guardrail tells it to do. The result is a legal aid pamphlet cited as if it were authority, a checker fetching that pamphlet forever to confirm a sentence nobody would dispute, and the same sentence re-researched and re-worded on every page it appears on.

The site already has the concept of advice that is not law. Migration 000001 seeded an editorial source at `/editorial`; `/editorial` explains to readers what the grey chip means; the human edit form has an "editorial guidance" checkbox; `propose_statement` accepts a citation of kind `editorial`. Only the drafting tool lacks the path. Opening it alone is the wrong fix. An agent that can mark its own statement editorial will use that to route a legal claim around the verbatim check, and the whole trust model rests on that check.

The reason editorial content has not scaled is that it has been generated per page. Everything else on the site that recurs across places scales by reference: a concept slug names a claim once and pages tag it (ADR-011); a reference page is a projection of published statements, not a rewrite (ADR-012). Advice should work the same way. There are perhaps thirty pieces of practical advice a renter ever needs to hear. They should be written once, by a person, and pointed at.

A second problem shares this ADR because the fix lands on the same surface. The playbook page has grown by accretion. A statement item can now carry a body, one chip per citation, a trust line, a "rule depends on where you live" link, and a "see the full guides" link. Around the list sit a badge, a byline, a lede, a place picker on national pages, a help bar, a disclaimer, a report link, sibling topics, other cities, and the footer. Each was added for a reason, and no ADR says what the page is made of or what each part is for. Adding an advice block to that page without first writing down its regions would be one more accretion.

## Decision

### D1. Advice is a registry, written by people, extended by migration

```
advice
  id         BIGSERIAL
  slug       TEXT UNIQUE      e.g. write-everything-down
  body_md    TEXT             one plain-language paragraph, editorial voice
  language   TEXT DEFAULT 'en'
  topic_id   BIGINT NULL      the topic it most belongs to, for the authoring picker; NULL = any
  retired_at TIMESTAMPTZ NULL
```

The registry is written only by migration, like concepts and topics (ADR-005 D5, ADR-011 D1). Its entries are site voice: they lint against the editorial-voice ruleset in a test, the same as `ui.go` strings. Retiring an entry keeps the row so existing references resolve; a retired entry stops rendering everywhere at once.

An entry has no jurisdiction, no citation, no quote, and no checked date. That is the definition: advice is what stays true when the law changes. If an entry ever needs a citation to be true, it is not advice, it is a claim, and it moves to a statement on the pages where it holds.

Seed set: the recurring editorial-only statements already published, deduplicated. The first migration lists them and the pages they came from. Expect fifteen to thirty entries.

### D2. Pages reference advice; the agent references, never authors

```
playbook_advice
  playbook_id  BIGINT REFERENCES playbooks ON DELETE CASCADE
  advice_id    BIGINT REFERENCES advice
  position     INT
  PRIMARY KEY (playbook_id, advice_id)
```

- `save_draft_playbook` gains `advice: []string` of registry slugs. An unknown or retired slug is rejected with the current list. The agent's system prompt gets one rule: a practical habit that no statute creates is not a statement; reference it from the advice list, and if nothing there fits, drop it and say so in the draft's author note. The tool description says the same.
- The drafting tool keeps rejecting a statement with no citation, and still has no editorial citation kind. Nothing the agent writes can escape the verbatim check.
- `propose_statement` stops accepting kind `editorial`. A proposal is agent-filed text; the same rule applies. The human edit form keeps its editorial checkbox, because a person is the editorial authority, and gains an advice picker beside the statement list.
- `get_playbook` returns the page's advice slugs so a revision keeps them.
- The portal's edit form shows advice as its own short list under the statements with add and remove, not as statements. The preview renders it exactly as the public page does.

Migration of what exists: an audit script lists every editorial-only statement on published pages. Those that match a seed entry are replaced by a reference in the same migration. Those that are page-specific stay as editorial statements. Agent-drafted advice that today cites a legal aid page is left alone; a later pass files their removal and the matching reference through the ordinary revision path.

### D3. The boundary between advice and law is decided by the statute, and the reviewer keeps it

"Send the repair request in writing" is advice in most places and a legal trigger in Texas. The rule for both the agent and the reviewer: if a statute makes the act matter, it is a statement with a citation to that statute. The generic habit is a reference. The agent will misfile some. The reviewer sees advice and statements side by side in the editor and the preview, and moving one across the line is one action. No lint tries to detect advice-shaped text; the last thing this site needs is a second classifier arguing with the first.

### D4. Advice renders as one region, after the law, with no per-item apparatus

Advice never interleaves with statements. It is one block, `Before you act` (a `ui.go` string), rendered after the statement list and before the report link. It is a plain list. Each item is the body and nothing else: no chip, no trust line, no anchor, no onward link, no report link. One line beneath the list says why these carry no source and links to `/editorial`. That page is rewritten to describe the registry rather than the chip.

By page kind:

- default, faq: the block as described.
- checklist: the block renders in checklist style, so a reader ticks advice the way they tick the rest.
- directory: no advice block. A directory has no law on it and advice would be the only thing with no organisation attached.
- national and state pages: same as the city page. The block is identical everywhere by construction, which is the point.

The editorial chip stays for the page-specific editorial statements humans still write. It is no longer the way recurring advice appears.

Search does not index advice. Thirty identical paragraphs on two hundred pages would make every query match every page.

### D5. The playbook page is a fixed inventory of regions, each with one job

This is the part the advice block forced. The page is the following regions, in order, and nothing else. Adding a region, or adding a second element to a statement item, amends this table.

| Region | Job | Shown when |
|---|---|---|
| Sidebar | Place and topic label, back to all topics, language switch | Always |
| Breadcrumb | Where this page sits in the site | Always |
| Header: badge, title, byline, lede | What place, what question, who checked it and when, one paragraph of framing | Always; byline date only when set |
| Place picker | Route a national reader to their own place before they read a rule that may not be theirs | National pages with covered places |
| Help bar | One link to local help for a reader in crisis | When the place has a resource directory |
| Disclaimer | Not legal advice | Always |
| Statement list | The law, one claim per item | When the page has statements |
| Advice block | Practical habits, one list, no apparatus | When the page references advice (D4) |
| Report link | One place to say something is wrong | Always |
| Sibling topics | Other questions in this place | When any exist |
| Other places | This question elsewhere | City and state pages, when any exist |
| Footer | Site-wide | Always |

A statement item is: body, chips, at most one trust line, at most one onward link. Today it can carry two onward links, the concept specifics link and the topic reference link. They are mutually exclusive at the data layer (ADR-011 D7) so this is already true; the rule makes it a constraint rather than a coincidence. The trust line appears only when fully earned (ADR-003, render layer) and that does not change.

Two rules for future additions. Nothing appears twice on one page: the help bar is the local-help link, so no statement links to the directory, and the report link is the page's feedback affordance, so no item has its own. And a region exists for a reader's job, not for an inventory of what the data can show: the badge says the place, the sidebar says the place, and the breadcrumb says the place, which is three regions doing one job and the next amendment to this table should remove one.

### D6. Checks, queue, and the checker

Advice has nothing to fetch, so the source checker never sees it. The issue checker (ADR-013 D2) reports `advice-retired` on a page referencing a retired entry, which blocks publish until the reference is removed; a live page referencing a retired entry keeps serving without it, since retirement was a decision to stop showing it. The review queue has no advice item type; an advice edit is a migration, reviewed as code.

## Consequences

- The agent stops citing pamphlets for habits. Drafts get shorter and the checker fetches less. What it does cite is law, which is what the verbatim check was built to protect.
- Advice is written once and edited once. A change to "write everything down" reaches every page on the next deploy, with the migration as its record.
- The page gets a written shape. The next feature that wants a spot on it has to say which region it lives in, or argue for a new row in D5's table.
- A reviewer gains one classification to make per draft: is this a statement or a reference. It replaces the work of reading the same advice re-worded on every page.
- Advice entries are English only until ADR-015 lifts. The table has a language column so the Spanish pass, when it comes, translates the registry once rather than every page.
- Page-specific editorial statements still exist and still need a person to write them. This ADR does not make those scale. It makes them rare.

## Rejected

- **Let the agent mark its own statements editorial.** Opens a path around the verbatim guardrail for the content most in need of it, and reproduces per-page generation, the thing that did not scale.
- **A lint that detects advice-shaped text.** Fragile, and it would recreate this ADR's problem with a different label as the agent learns to phrase around it.
- **Drop practical advice from the site.** Renters need it, and the legal aid pages the agent was citing exist because someone had to say it.
- **Interleave advice with statements at a per-page position.** Contextual placement is sometimes better, but it makes advice look like a numbered claim, and it means the advice block differs on every page. One block, one look, one job.
- **Author advice in the portal instead of by migration.** Thirty rows edited a few times a year do not justify a form, and a migration is the only path already reviewed and deployed as code.

## Amendment, 2026-10-09: advice is backed, leads the stressful pages, and carries the warnings

Four weeks of review changed the problem this ADR was written for. Agents could not write advice, so the loop wrote it as editorial statements, one copy per statement. On 2026-10-07 the corpus held "If a court later disagrees" 424 times, "Get legal help first" 501 times, the win-and-pays line 370 times, and the same two collections statements on 52 pages. Lints require the warning inside each statement (the risky-step and owe-nothing rules), so the copies are enforced. The pages got long, and Nazanin's read of the breaking-lease pages was that they are too much for a renter in a stressful situation. The most useful content on those pages is not law at all: get safe, put it in writing, keep proof, make sure your landlord knows you have it. In her words: "Your landlord might sue if they want the money. Make sure they know you have proof."

This amendment keeps D2 (agents reference, never author), D3 (the statute decides the line) and D6, and changes D1, D4 and how the warnings are enforced.

### A1. Every entry is backed (replaces D1's "no citation")

An advice entry carries one or more citations, each a source URL and a verbatim quote, checked by the same quote check as statements. Advice stays jurisdiction-free: if it is only true in some places, it is a claim and belongs in a statement on those pages.

- **Who can back it.** Only sources of kind `gov_guidance` (federal or state government pages, like HUD and the CFPB) or `nonprofit` (legal aid organizations and housing nonprofits). Enforced in code at save time, not by review. Statutes do not back advice: a habit a statute makes matter is a statement (D3). `discover.ReferenceOnly` sites (law firm blogs, Nolo, Justia, FindLaw, Lexis) are refused as they are everywhere.
- **No backing, no publish.** An entry without a backing quote cannot be referenced by a page that publishes. The one exception is the site's own disclaimer, which is the site speaking and says so.
- **The checker watches it.** The source checker re-checks advice quotes like statement quotes. Today `store/monitor.go` re-checks only `statute` and `regulation` sources; it gains `gov_guidance` and `nonprofit` for advice citations. A drifted quote on an entry flags the entry once, not every page that references it.
- **Lawyer review, later.** When the project has its own lawyer, they review entries one at a time. A reviewed entry carries a stamp with the reviewer's name and date, and renders a different chip from a source-backed one. This is not a lawyer article as a source (the lawyer article policy stands): it is the project's lawyer approving the project's text. An entry may be lawyer-reviewed and source-backed at once; lawyer review never replaces the quote.

```
advice_citations
  advice_id   BIGINT REFERENCES advice ON DELETE CASCADE
  source_id   BIGINT REFERENCES sources   -- kind must be gov_guidance or nonprofit
  quote       TEXT NOT NULL               -- verbatim, checked
  position    INT

advice  (+ columns)
  lawyer_reviewed_by  TEXT NULL
  lawyer_reviewed_at  TIMESTAMPTZ NULL
```

### A2. Advice can lead, per stage (amends D4)

D4 put advice in one block after the law. For a page whose reader is in a crisis (breaking a lease, locked out, eviction), the advice is the first answer and the law is how it holds up. So advice attaches to a stage, not only to the page:

- A page's advice references carry an optional stage. An entry with a stage renders at the top of that stage's section, before its statements. An entry with no stage renders in the page-level block as D4 describes.
- The topic decides the default: a topic lists its advice entries per stage, and a new draft inherits them. A page may add or drop references; the reviewer sees the change.
- Rendering stays plain: the body, then one chip naming the source ("From the CFPB") or the lawyer stamp. No trust line, no anchor, no onward link.
- D5's table gains the per-stage advice as part of the statement list region, not a new region.

### A3. Warnings move to advice, and the lints check the page (amends the risky-step and owe-nothing rules)

The risk warning, the lawsuit reminder, the win-and-pays line, and "don't ignore court papers" become advice entries. A statement no longer carries them. The guarantee moves up one level and stays in code:

- A statement that names a risky step (riskyStep) or tells the renter they owe nothing (noOwe) passes the lint when its stage, or the page, references the matching warning entry. Without the reference it fails exactly as today.
- The warning fits the step, as the 2026-10-06 rule does: the move-out entry says you can still owe the rent; the stay-and-withhold entry names eviction.
- The page-level disclaimer for breaking-lease ("Breaking a lease early is legally complicated...") becomes the topic's first entry, rendered at the top of the page, and leaves the intro statement.

### A4. Migration

1. Seed the registry with the repeated sentences above plus the breaking-lease set, each with a backing quote found on gov or nonprofit pages. An entry for which no source says it waits, unpublished, for the lawyer. Expected breaking-lease set: the disclaimer; get safe first; tell your landlord in writing; take dated photos; call the inspector; keep copies; your landlord might sue if they want the money, make sure they know you have proof; do not ignore court papers.
2. Draft pages: the loop removes the warning sentences from statements and adds the references, through ordinary proposals. Statements get shorter; nothing else in them changes.
3. Live pages: the same proposals go to the queue for a person, as all live edits do.
4. The 350 editorial-only statements are sorted three ways: a registry entry (replaced by a reference), page-specific guidance (stays), or a claim with no law behind it (cut).

### A5. Two kinds of content, one set of checks

After this amendment the site has exactly two kinds of content. They differ only where they must; every check that can be shared is shared, so neither kind gets a weaker copy.

| | Statement (the law) | Advice (what to do) |
|---|---|---|
| What it says | What a law requires, in one place | A habit that stays true everywhere |
| Lives on | One page, one place | The registry, referenced by pages |
| Who writes it | Drafting agents, reviewed by a person and the review loop | A person, added by migration and reviewed as code |
| Sources allowed | statute, regulation, court_ruling, gov_guidance | gov_guidance and nonprofit only |
| Quote check | Verbatim, the one checker | The same checker |
| Source monitoring | Yes | Yes (monitor.go gains these kinds) |
| Voice lint | The one lint | The same lint |
| Review | Statement stamps and the queue | Entry stamp; later the lawyer stamp |
| Edits | Proposals in the queue; live pages by a person | A migration; one change reaches every page |
| Place | Implied by the page | Never; if it holds only in some places, it is a statement |

Shared, never forked: the verbatim quote checker, the voice lint, the source monitor, and the reference-only refusal (`discover.ReferenceOnly`).

**No editorial-only statements.** Every statement has at least one citation that is not editorial; editorial alone never backs a statement. The save path and `triage check` enforce it. The editorial citation survives only beside a real quote, for framing such as a worked example of a quoted rule. The disclaimer is the one piece of pure site voice, and it is an advice entry with a written exemption from A1's backing rule, not a statement. The 350 editorial-only statements are sorted in A4 step 4; until each is sorted, existing ones are not blocked, the same as every new lint (ADR-023 amendment).

### A6. Open questions for Nazanin

- Does per-stage advice render with a heading ("What to do") or simply lead the section?
- Pilot topic: breaking-lease first, on one state, before the registry is built out?
- Lawyer review: does a reviewed entry show the lawyer's name to readers, or only "Reviewed by a lawyer"?
