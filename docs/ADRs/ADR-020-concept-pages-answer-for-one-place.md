# ADR-020 — A concept page answers one renter in one place

| | |
|---|---|
| Status | Deferred until every state has published guides. D5 shipped 2026-09-17 (migration 000042). |
| Date | 2026-09-17 |

## Context

ADR-011 tagged statements with concepts. ADR-012 projected them into a reference layer: `/terms` lists every concept with a definition, and `/c/{slug}` shows every published statement carrying that concept, national first, then one per place. The homepage carries the same list under "Legal terms, explained".

The page is a GROUP BY. It takes one tag and lists every statement that carries it, across every place. That is the view an editor wants for comparing states, and the tag system is one of the best parts of the data model. It is not a page a renter can use. A renter has one place and one question. On `/c/deposit-return-deadline` today they get a generic national paragraph, then Austin, Boston, Los Angeles, New York City and on down. Their own city is somewhere in the scroll. The "Find your place" box asks them to type it, though the site already remembers where they rent (homepage scope, ADR-017 accounts).

The index has a second problem. Roughly half the entries are words a renter meets on paper: notice to quit, grace period, quiet enjoyment, mediation, small claims court. The other half are rules the data model tracks, named the way a schema names them: "Deposit penalty damages (2x/3x)", "Court records / evidence", "Entry notice period". No renter searches for those strings. They search "how many days does my landlord have to return my deposit". ADR-009's follow-up (2026-09-08) already retitled every guide as the renter's question for the same reason.

The definitions themselves are shorter than the statements but still lean on words the editorial voice bans or glosses: "escrow", "itemized", "redemption", "HUD approved", "screening reports".

## Why this is deferred

Search Console shows question-shaped queries like "how much can a landlord charge for a security deposit" are where clicks come from. A page with that title has to answer for a searcher from any of 50 states. On 2026-09-17 `/c/deposit-cap` has rows for 5 states. Wave 1 drafted state guides for California, New York, Texas, Illinois, Washington, and Pennsylvania, but only the Texas deposit statement is tagged and published; the rest wait in review.

A question-titled page that answers 5 states and shrugs at 45 is worse than the current page for the reader Google sends, because the title promises an answer. So the rule-page work below waits until every state has published guides on the topic the rule belongs to. The trigger is data, not design. Until then, D5 (plain definitions) ships alone, and the concept page keeps its current shape.

What to do in the meantime, in order of leverage:

1. Review and publish the wave 1 state drafts, and tag their rule statements with the matching concepts.
2. Run the next state waves (two agents at a time on Sonnet, per the state-guides plan) until every state has the 7 core topics.
3. Then return here.

## Decision

### D1. A concept has two independent properties, not a kind

A concept may be a **word** a renter meets on a lease, a notice, or court papers. A concept may have **one answer per place**: a number of days, a cap, a permission. Many are both: grace period is a word on the lease and a number that differs by state. So the registry carries two fields rather than a kind enum:

- `glossary BOOLEAN NOT NULL`: true lists the concept under Legal terms by its name.
- `question TEXT NOT NULL DEFAULT ''`: non-empty means the page takes the answer shape below, with the question as its title.

Every concept is at least one of the two. When a concept has a question and is also in the glossary, the name renders small above the question, the way a city badge sits above a guide title.

The classification is registry metadata, set by migration like the definitions (000024), never by the drafting agent.

Proposed classification, to be settled by Nazanin and Cameron before the migration ships:

| Slug | Glossary | Question |
|---|---|---|
| deposit-return-deadline | | How long does my landlord have to return my deposit? |
| deposit-cap | | How much can a landlord charge for a security deposit? |
| deposit-escrow-interest | | Where must my landlord keep my deposit, and does it earn interest? |
| deposit-damages | | What does my landlord owe me for wrongly keeping my deposit? |
| deposit-receipt | | Do I get a receipt for my deposit? |
| deduction-itemization | | Must my landlord list what was taken out of my deposit? |
| entry-notice-period | | How much notice must my landlord give before coming in? |
| entry-allowed-reasons | | When is my landlord allowed to come in? |
| emergency-entry | | Can my landlord come in without notice in an emergency? |
| entry-penalties | | What can I do if my landlord comes in without the right to? |
| increase-notice-period | | How much notice do I get before a rent increase? |
| mid-lease-protection | | Can my rent go up during my lease? |
| partial-payments | | What happens if I pay part of my rent? |
| utility-shutoff-protection | | When can a utility company shut off my service? |
| rent-debt-collection | | What happens to rent I still owe after I move out? |
| grace-period | yes | How many days do I have after rent is due? |
| notice-to-quit | yes | How much notice do I get before I have to move out? |
| late-rent-notice | yes | How much warning do I get before an eviction for late rent? |
| pay-and-stay | yes | Can I stop an eviction by paying what I owe? |
| rent-withholding | yes | Can I stop paying rent until repairs are made? |
| repair-and-deduct | yes | Can I pay for a repair and take it off my rent? |
| eviction-record | yes | Can I seal an eviction record? |
| rent-control | yes | Is there a limit on rent increases where I live? |
| lock-change-rules | yes | Can I change my locks? Can my landlord? |
| late-fees | yes | How much can my landlord charge for late rent? |
| answer-the-case | yes | |
| quiet-enjoyment | yes | |
| constructive-eviction | yes | |
| mediation | yes | |
| small-claims-court | yes | |
| retaliation-protection | yes | |
| fair-housing | yes | |
| habitability-standard | yes | |
| illegal-lockout | yes | |
| court-eviction-only | yes | |
| free-legal-help | yes | |
| help-lines | yes | |
| housing-counseling | yes | |
| complaint-line | yes | |
| code-inspection | yes | |
| records-and-evidence | yes | |
| repair-request-in-writing | yes | |
| rent-assistance-programs | yes | |
| federal-housing-assistance | yes | |
| eviction-court-process | yes | |

### D2. The question is the title

For a concept with a question, the question is the `<title>`, the `<h1>`, the `og:title`, and the search result title. The name stays the label inside statement bodies and on the portal, where "Deposit return deadline" is the right register.

### D3. The page answers for the reader's place first

The page takes the reader's place from two sources, in this order:

1. `?j={slug}` on the URL, resolved server-side. A distinct URL, so the shared cache keys it separately, the same way the topic hub resolves `?j=` today. Search results and links inside guides pass it.
2. The remembered location (`renterlaw.location` in localStorage, or the account's location via `/api/me`), applied client-side by the same script the homepage uses. The bare `/c/{slug}` page stays shared-cache HTML and never personalises server-side.

Regions, in order:

| Region | Job |
|---|---|
| Title | The question, or the name for a glossary-only concept |
| Lede | The definition, which for a rule reads as the one-line general rule |
| Your place | "Where you live: Massachusetts" with the short answer, the statement, chips, trust line, link to the full guide |
| General rule | The national statement, when one exists |
| Every place | The comparison table (D6) |
| Report link | |

The place is resolved up the chain the way the 404 and the topic hub do it: the city's statement, else the state's, else the state-not-covered message (D7). That chain walk is the third copy of that logic and becomes one function before this ships.

With no place known, the "Your place" region is a picker of all 50 states, not only the covered ones (D7). The existing type-to-filter box goes.

### D4. The homepage and `/terms` list glossary concepts by name

"Legal terms, explained" on the homepage and the `/terms` index list glossary concepts by their name. Question-only concepts are reached from search, from the concept links inside statement bodies, and from the "See the rule in every place we cover" link on national statements. `/terms` gets a second section, "Questions with one answer per place", so nothing is unreachable.

### D5. Definitions are rewritten in the editorial voice (shipped)

Every definition is re-read against the editorial voice skill. Legal and program words are replaced or glossed in place: no "escrow", "itemized", "redemption", "HUD approved", "screening reports", "utilities" without its gloss. Two short sentences at most. Shipped as migration 000042 on 2026-09-17, independent of the rest.

### D6. A rule page leads with the number and ends with a table

The cap, the days, the permission is what the searcher wants, and the statement prose takes three sentences to reach it on some rows. A statement carrying a question concept may carry an optional **short answer** ("1 month's rent", "No cap", "14 days"), written by the reviewer, never the agent. It renders above the statement. Empty means the statement stands alone.

The rule is state law in every row seen so far. The page answers at the state level and lists a city only when its statement adds to the state's. Austin and Texas do not both appear.

After the reader's answer, a table: place, short answer, source chip, one line per covered state. This is the GROUP BY view rendered the way a searcher wants it, and the reason the comparison stays on the page.

The page ships JSON-LD for the question and the per-state answers, and a meta description that carries the general rule with its numbers in it.

### D7. An uncovered state gets a real page, not a shrug

A searcher who picks a state with no row gets: that we do not have that state yet, the general rule with its numbers, how to find the state's own law, and the ask-us-to-cover link. Never a 404 and never an alphabetical list of other people's cities.

## Consequences

- The reference page stops being a comparison table with no answer and becomes an answer with a comparison behind it. The GROUP BY view stays intact as the table, so the editor loses nothing.
- Search results for rule concepts get a title that matches what was typed.
- ADR-012's "pages are projections of published statements" holds. The new columns are registry metadata and one optional reviewer-written field on a statement.
- Nothing in D1 through D4, D6, or D7 is built until the coverage trigger above is met.

## Rejected

- **Shipping the rule page at 5-state coverage.** The title promises an answer the data cannot give for most readers. See "Why this is deferred".
- **A kind enum (word, rule, both).** Two independent properties model the same thing without a third value that is really the other two together.
- **Personalising the bare `/c/{slug}` server-side.** Same reason as ADR-012 and the homepage scope: browse routes are shared-cache, and a renter's city must not leak into the next reader's page.
- **Dropping the other places.** The comparison is the reason the tag system exists, and a renter who is moving wants it. Rendered as a table, not removed.
- **Inline glosses inside guides (tap a term, see the definition without leaving).** Right, and later. It is a guide-page change with its own region rules under ADR-016, not a concept-page change.
