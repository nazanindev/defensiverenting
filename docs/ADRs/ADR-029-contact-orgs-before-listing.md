# ADR-029 — Small local orgs are contacted before we send them renters

| | |
|---|---|
| Status | Accepted 2026-10-04 |
| Date | 2026-10-04 |
| Amends | ADR-011 (Local Help pages stay, some statements hide), ADR-013 (the view page shows what is hidden) |

## Context

Every state and DC now has a Local Help page, and traffic rose sharply when all 50 states went live. Those pages, and some guides, tell renters to call or contact a named org. Some of those orgs exist to take public traffic: courts, attorney general lines, 211, statewide self-help sites. Others are a tenant union, a two-person clinic, or a legal aid office that already turns people away. None of them knows we list them.

If the site grows, we could send a small org more calls than it can answer, and we cannot predict when. That hurts the org, and it hurts the renter who calls and gets no one. Nazanin: "There is no way to predict when we will hit the threshold where we are overloading these orgs. Better safe than sorry."

An audit of prod on 2026-10-04 found 62 live Local Help pages (51 states, 10 cities, 1 national) and about 464 cited orgs. About 292 are public. 172 are direct-service orgs. About 516 live statements on Local Help pages and 114 on guides cite one of those 172.

## Decision

### D1. Every org has a type: public or contact first

- **Public**: government offices, courts, housing agencies, 211, self-help websites (the LawHelp family, Illinois Legal Aid Online, Ohio Legal Help), ABA Free Legal Answers, the Legal Services Corporation, state bars. They exist to take public traffic.
- **Contact first**: orgs that answer renters one by one. Legal aid offices that take cases, tenant unions, clinics, fair housing centers, mediation centers, community action agencies.

A source of kind `gov_guidance`, `statute`, `regulation`, `court_ruling` or `editorial` is always public. Only `nonprofit` sources need a type.

### D2. An org is a row, keyed by its website

A new `help_orgs` table holds one row per website host: name, host, type, and who changed it and when. A `nonprofit` source belongs to the org of its host. Several sources on one site (for example the variants of TexasLawHelp.org) are one org and are contacted once.

Each contact attempt is a row in `help_org_contacts`: date, method (email or call), who was reached, outcome, a note, and who logged it. The outcome is one of **no answer**, **left message**, **maybe**, **yes**, **no**. An org takes several tries, and the log keeps all of them.

An org's status comes from its latest attempt:

| Latest outcome | Status | On the live site |
|---|---|---|
| none yet | not contacted | hidden |
| no answer, left message | contacted | hidden |
| maybe | maybe | hidden |
| yes | OK to list | shown |
| no | leave off | hidden |

Only a yes makes an org show. No reply, however long, is not a yes.

A "maybe" means they want to think, need to ask someone, or asked for more information. It stays hidden. Any attempt can carry a follow-up date that the editor picks. On that date the org comes back to the top of the Orgs page, so nothing is forgotten.

A `nonprofit` source whose host has no row counts as contact first and not contacted. A new org an agent cites stays off the live site until someone looks at it. The safe answer is the default.

The audit's typing is the seed. It is a first pass from names and websites, and the editor corrects it on the Orgs page.

### D3. The live site shows a contact-first org only when it is OK to list

A statement that cites a contact-first org whose status is not **OK to list** is hidden from every public surface: guides, Local Help pages, concept pages, terms, and search. It is not deleted, and its page does not change. When the org is marked OK to list, the statement shows again with no further step.

The rule is the same on guides and on Local Help pages. A guide statement that cites a legal aid site only for a fact also hides. One rule is easier to trust than a judgement about each sentence's wording.

A page with no visible statements left is not shown, and the "Need help now?" bar does not link to it.

Drafts are unaffected. The authoring view page marks each hidden statement and names the org that hides it, so the editor sees what readers do not.

### D4. One Orgs page

The authoring portal gets one Orgs page with one row per org: name, website, places it appears, clicks last month (D6), type, and status. Contact-first orgs come first, ordered by how many renters the places they serve send us, with maybes that are due again on top.

- The type can be switched, for orgs the first pass got wrong.
- One form logs an attempt: email or call, who was reached, outcome, note. The org's status follows from it.
- The org's past attempts show under it.

Nazanin's editor contacts orgs outside the site, by email or phone, using a fixed script and email template (`cmd/authoring/script/org-contact-script.md`, pinned beside the Orgs list and fillable per org). Each says what the site is, how we describe the org, about how many renters we send from their area, and how to ask us to change or remove the listing.

### D5. What changes on the live site, and in what order

The 172 contact-first orgs start as not contacted, so their statements leave the live site when hiding turns on. Local Help pages keep their government offices, 211 and self-help sites. From the 2026-10-04 audit, hiding alone would empty Austin, Chicago and New York City, leave Los Angeles one statement, and leave Kansas, Mississippi, Missouri, Tennessee, Virginia, West Virginia, Wisconsin and the national page three each.

So hiding turns on last (D8 first):

1. Drafts add public help to every Local Help page that hiding would leave short of D8.
2. Nazanin publishes them.
3. Hiding turns on.

A renter on any Local Help page always has somewhere public to go.

### D8. Every Local Help page carries public help

A Local Help page needs at least three statements that cite only public sources (D1): for example 211, the court self-help center, a city or state housing office, or the state's LawHelp site. This is checked like any other publish issue (ADR-013): a Local Help draft with fewer is held from publishing, with the issue `local-help-public`. The count leaves out any statement that cites a contact-first org, because that statement may be hidden.

The rule holds for every Local Help page from now on, not only the ones this ADR touches.

### D6. We count clicks to orgs

A link to a source still goes straight to the source, so a reader always sees where it leads. When it is clicked, the page script reports the click to `/out` beside it, and that adds one to the source's count for the day. A phone number in a statement becomes a tap-to-call link and is reported the same way, against the statement's first source. A tap is counted, not a call; nothing can count calls. A reader without scripts is not counted.

(Amended at build, 2026-10-04: first written as a redirect through `/out/{source id}`. A citation chip that shows our address instead of the statute's would undercut the chip's point.)

The count follows the rule "Did this page help?" already uses (d3aed53): one click per reader per org per day, a daily cap per reader, and a reader known only by a hash of their address with that day's salt, deleted when the day ends. One person clicking a link over and over moves the count by one.

Public orgs stay visible, so their clicks, divided by page views, give a click rate. Applied to a place's page views, that rate estimates what a contact-first org there would receive. It also tells us which pages and orgs readers use, for our own decisions.

### D7. Hide now, measure for a month, then call

The contact-first orgs are hidden once every Local Help page has public help (D5). Click counting starts as soon as it ships. Calls and emails start about a month later, so the editor can give each org a real number for its area. The month also gives time to write the script and template and to correct the first-pass typing.

## Consequences

- No small org gets traffic from us before a person has spoken to it.
- No Local Help page is left without a public place to turn (D8).
- About 630 live statements go dark until the editor works through the list. The orgs that cover the most places should be contacted first.
- Guides that cited a legal aid site for a fact lose that statement until the org is OK to list. An agent may re-cite such a fact to a public source on a draft. Live pages stay human (decided 2026-09-22).
- Agents keep drafting as now. A new direct-service org they cite stays hidden by default, so drafting rules do not need to change.
