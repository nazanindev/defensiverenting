# ADR-031 — The first screen of a guide is for the reader Google sent

| | |
|---|---|
| Status | Accepted 2026-10-04 |
| Date | 2026-10-04 |
| Amends | ADR-017 (the saved place now also picks the guide), ADR-020 D3 (place first, on guides too), ADR-028 D10 (stage headings unchanged) |

## Context

The public site has grown one piece at a time, and Nazanin, 2026-10-04: "The user facing site is getting too bloated. I'd want to clean it up first if we're going to add more." She asked to start from who uses the site and how, given that most people arrive from a Google search.

Search Console (read 2026-10-04) shows:

- Every click from search so far landed on a guide, city or state. The homepage, place hubs, topic hubs, All locations and the about pages get almost no search traffic.
- Most clicks come from phones.
- Many queries name no place ("can a landlord just show up without notice", "can I call the police if my landlord enters without permission"). Google answers them with whichever city page fits best. So a reader on the Boston entry guide is often not in Boston.
- Some searchers want a person or money, not the rule ("security deposit assistance", "rent assistance seattle", "eviction help seattle").

So the guide page is the front door, and its most common reader may be in the wrong place. The top of a guide did not serve that reader. It said where you were four times (a sidebar label, "← All topics", a breadcrumb, a city badge) and gave no plain way to say "I rent in Ohio". The only list of other places sat at the foot of the page, 62 links long. Every statement repeated the same "Sources checked" date. The header carried a tagline, a search box, Legal terms and Sign in on every page.

## Decision

### D1. The place line

Under the title, a city or state guide says whose rules it gives: "For Pittsburgh, Pennsylvania." Below it, "Rent somewhere else?" opens the list of places that have this topic. Each link goes to **the same topic** in that place, and picking one saves it as the reader's place (`data-set-location`, ADR-017).

The sidebar, the "← All topics" link, the visible breadcrumb and the city badge are removed. The breadcrumb stays in the JSON-LD for search. The language link, when a translation exists, moves into the place line. Nationwide guides keep their "Where do you rent?" picker and get no place line.

The list of other places at the foot of a guide is removed. "More tenant rights in {place}" stays at the foot, for the reader in the right place with the wrong problem.

### D2. Offer the reader's own place

When the site knows or can guess where the reader rents, the place line adds: "Do you rent in Ohio? See this guide for Ohio". The link goes to the nearest guide for this topic there (`/api/coverage`).

- **Known:** the place saved on this device, or in the reader's account (ADR-017).
- **Guessed:** when nothing is saved, `/api/where` returns a US state from the visitor location headers the CDN adds (`CF-IPCountry`, `CF-Region-Code`). Decided by Nazanin 2026-10-04: worth it if it routes people to the right page.
- A guess is shown as a question and is never stored. Tapping the link saves it, like any other place pick. `/api/where` stores and logs nothing and is never cached.
- A guess of the page's own state is not offered on a city page, since the reader may be in that city.
- The page is shared-cached, so all of this runs in the browser after load. Without a known or guessed place, nothing shows.

**Not live yet:** renterlaw.org is served by Fly directly, not through the Cloudflare proxy, so the headers do not arrive and the guess returns nothing. It turns on when the domain is proxied through Cloudflare with "Add visitor location headers" enabled. The saved-place offer works now.

### D3. Not legal advice comes first; the byline is fine print at the foot

Nazanin, 2026-10-04: "Renters are an at-risk population and they deserve to know what they're getting into." Many reference sites carry one disclaimer in the footer. We put ours first, and make it say something.

- Directly under the title, before the place line, the intro and any rule, on every guide: a solid block in the header's color with an info mark. "**Not legal advice.** This page explains the law in general. It cannot tell you what to do in your case. For help with your case, contact legal aid in Pittsburgh." The link goes to the place's Local Help page. Without one: "contact a legal aid office near you."
- Not in the site header: a strip on every page stops being seen, takes phone space on pages where it means nothing, and sits apart from the rules it warns about. Not above the title: the reader first needs to know they are on the right page, and a warning before anything else reads like an error.
- The rows below it (place line, the reader's own place) share one look: thin lines between them, like the Local Help row. Nazanin: what is not important can be small and at the bottom.
- The byline moves to the foot of the guide, as fine print: "Published by Nazanin · September 24, 2026 · sources checked October 3, 2026". The date is the oldest confirmation across the page's statements, and shown only when every statement has one. That is the same fully-earned-or-absent rule the per-statement line kept.
- The per-statement "✓ Sources checked" line is removed from guides, FAQs and checklists. Rules pages keep it on each answer, since those answers are shown from other guides. Local Help keeps "Details checked" on each organization.

### D4. Local help is a row under the intro

"Need help now? Local Help in Pittsburgh →", a row in the same style right under the intro, where the "rent assistance" searcher sees it before the rules. It replaces the box.

### D5. The header is the logo and the reader's place

On every page. The tagline goes. The header search goes until vector search exists. Then we decide where search lives. The search boxes on the homepage, hubs and 404 page stay as they are until then. Legal terms and Sign in move to the footer.

## Consequences

- The top of a guide answers "is this my place?" and offers one tap when it is not. The page loses four navigation markers and a 62-link list.
- A guide is one column on desktop. Reading width stays near 820px.
- Statements read without a date under each. A statement whose source failed the weekly check now shows nothing on the page itself, and the page's date disappears. Marking the one failing statement is not built (see Later).
- Readers who used the header search must use the homepage or hub search until vector search ships.

## Later

- Mark a statement whose source failed this week's check, so the page says which one, not only that the date is gone.
- Turn on the Cloudflare proxy and location headers so D2's guess works.
- The rest of the cleanup, from the same Search Console reading: city guides that repeat their state guide, very long legal-term pages, and hubs and topic hubs that no search reaches.

## Rejected

- **A real-borders US map for picking a place.** Small states are hard to tap on a phone, and a list does the job.
- **Showing the guess as a statement ("You are in Ohio").** IP location is often wrong. A question lets the reader ignore it.
- **Removing every search box now.** The homepage and hub boxes are the only search until vector search ships.
