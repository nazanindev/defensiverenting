# ADR-030 — Show the rigor to the renter

| | |
|---|---|
| Status | Proposed 2026-10-04 |
| Date | 2026-10-04 |
| Amends | ADR-017 (reader accounts gain law-change emails), ADR-024 (a law change becomes something a reader can see) |

## Context

On 2026-10-04 we reviewed another 50-state landlord-tenant reference, this one written for landlords. Its research method matches ours almost point for point: primary sources only, every fact cited, a weekly source check, "no statute" stated as a finding, question titles, AI doing the reading under a written procedure, and a person accountable for what publishes.

Two things follow.

- **The method is no longer rare.** Anyone with a model and a procedure can build a cited, checked, 50-state dataset in a few months. Our edge is not the substrate.
- **Nobody points that method at the renter.** Tenant-facing law content exists, but it is written in legal grammar, covers one place, or carries no citations. Nazanin, 2026-10-04: "No one in this field is writing for the tenants themselves, even with a one-to-one research methodology."

The review also showed ways of putting the method in front of a reader that we do not use yet. Our checks and reviews mostly happen out of the reader's sight. A statement shows "Sources checked" with a date, and a page shows who reviewed it. A renter cannot see that a law changed, that we fixed a mistake, or when the law changed where they rent.

Decision, Nazanin, 2026-10-04: take the parts that help renters and leave the rest.

## Decision

Each part below still gets a words-first description of the screen and Nazanin's go before it is built.

### D1. Law-change emails

A reader with an account (ADR-017) can get an email when the law changes where they rent. The trigger is a change that was verified and published, not a raw drift finding. If nothing changed, no email goes out. Each email says what the rule was, what it is now, when it takes effect, and cites the source.

This is the first reason to make an account that a renter would feel.

### D2. Updated and Correction marks, and a public log

A page that changed after it was published gets one dated mark beside its review line:

- **Updated**: the law changed and the page changed with it.
- **Correction**: the page had something wrong and we fixed it.

The mark links to a short dated list at the bottom of the page: what changed and the source behind it. The same entries go on one public log page for the whole site.

The events already exist: approved proposals, drift findings, ADR-024 law changes. This decision is about showing them.

### D3. Answer first

A guide opens with the renter's question, a one-line answer, and the two or three numbers that matter (a deadline, an amount, a notice period), each cited. Then the steps.

The numbers come from the same per-state values the comparison pages use, so a guide and a comparison page cannot disagree. Comparison pages themselves are Nazanin's separate work and not part of this ADR.

### D4. Easy to cite

Each guide carries a short "Cite this page" line with the title, the reviewed date, and the URL. The site publishes an `llms.txt` that lists every published guide by place and question, so AI answers can point renters to us.

### D5. Research from our own data, later

Short pieces built from the comparison data, written for renters. For example: the most a landlord can legally ask for to move in, state by state. These wait until the comparison data exists.

## Already in place

- **People behind the site.** The Who we are page (`/authors`) names the reviewing editor with his background and links from every guide's review line. D2 adds a record of fixes next to it, not in place of it.

## Deferred, not in this ADR

- **Letters and deadline calculators.** Both wait for the lawyer consult (user-zero decision). They are not ruled out.
- **Paid links to tenant-rights lawyers.** Maybe (Nazanin, 2026-10-04). Rules on paying for lawyer referrals vary by state, so how a lawyer could pay us is a question for the lawyer consult. A paid link would never sit inside a guide's statements.

## What we do not take

- **Ads and affiliate links for landlord services.** That is a conflict of interest on a renter site.
- **Legal-register writing.** Plainness stays enforced in code (ab48c10).
- **Topic-first navigation.** Location is the search scope and situations are the browse axis (homepage decision 2026-09-23).

## Consequences

- The work we already do (stamps, checks, corrections) becomes visible to the reader. That is the trust argument.
- A visible Correction mark admits a mistake in public. We bet that renters trust a site that shows its fixes more than one that never seems to make any.
- D1 depends on telling a real law change apart from page noise. ADR-024 law-change findings and approved proposals are the source. Raw drift is not.
- D3 and D5 depend on typed per-state values. Until those exist, guides keep their current opening.

## Order

D1 and D2 first: they show what we already do and need no new data. D4 is small and can go any time. D3 follows the comparison data. D5 comes last.
