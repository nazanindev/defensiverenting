# Citation leads brief

Use this when a review-loop pass should also close coverage gaps. The leads come from another 50-state reference's statute citations, compared against every section our guides cite in the same state (published or draft). They live only on this machine, in `notes/citation-leads/` (gitignored). Read `README.md` there first: one summary table, then one `leads-<our-topic>--<their-topic>.md` file per topic.

## What a lead is

A lead is a statute section, or a recent session law, that the other reference cites for a state and topic and that no guide of ours for that state cites. It is a place to look, never a source.

- Read the official text yourself (`bin/dumpsrc <url>` or `triage fetch`), from the legislature or the state's code publisher. The lead's link is usually that official page.
- Never cite, quote, copy or name the other reference, in a statement, a note, a commit or a doc. Its wording, summaries, tables and selection are its own work under its data license. Section numbers and the law itself are public.
- Every statement drafted from a lead follows `common-rules.md` like any other: a verbatim stored quote from the official source, plain words, one claim.
- Links to lexisnexis.com are leads only. For a Lexis-only state, follow `no-free-code-states.md`.

## Each lead ends one of three ways

1. **Draft a statement**: the section gives a renter a right, a deadline, a limit or a step, and the page does not already say it. Re-read the page first (`bin/prod triage page <id>`); the rule may be there under a different section.
2. **Rules page**: the section answers a concept question but no situation needs it. It belongs on that topic's rules page (ADR-028), through `rules-brief.md` and `triage gaps`.
3. **Does not apply**: landlord-only procedure, court clerk mechanics, definitions with no renter consequence, commercial leases, or a section the page already covers. Drop it. No note is needed.

Live pages stay human: a lead on a live page becomes a proposal for Nazanin, never a direct edit.

## Order

1. **Law-change leads first** (46 across topics, the "Recent law changes" lists). Each is a check that our stored quote and wording reflect the law in force today. Known example: Connecticut § 47a-21(j) was amended by P.A. 26-79, effective 2026-10-01. Our Connecticut deposit guide quotes (j) for the Banking Commissioner complaint route from `/current/`, which may predate the change. Check the quote against the official text and refile if it moved.
2. **Small topics next**, where most leads are real gaps: rent increases (66), late fees (48), entry (30), deposits (93 + 50 interest).
3. **Eviction (689) and lease ending (776) last.** Most are court procedure and timing rules. Expect most to end as "does not apply" or as rules-page material, and keep situation pages to the 8 to 12 statement budget.

Spot-check findings from the deposits pilot (2026-10-04):

- Connecticut's 21-day return deadline was already right on our page, so a law-change lead can already be covered.
- Renter-useful gaps seen: Connecticut § 47a-7c (move-in walk-through; no deduction for a noted condition) and § 47a-4d (move-in fee limits); Texas Prop. Code §§ 92.110, 92.112, 92.113; Michigan MCL 554.602 to 554.604; Delaware 25 Del. C. §§ 5310, 5311, 5514A.
- Arkansas and Tennessee deposit guides cite no statutes on our side, which is expected for those code hosts.

## Where we have no page yet

Leads in the "no page of ours yet" sections (rent increases 13, entry 6, late fees 3) are starting sources for the drafter of that page. Give them to the drafter as URLs to read, in the same discovery-only terms.

## Rerunning after new drafts

The scripts are in `notes/citation-leads/`: `rl_collect_all.py` reads our side through `bin/prod triage page <id>`, and `la_compare_all.py` writes the leads files. The other reference's citations are already saved there as `la_<topic>.json`. Rerun only our side and the compare; refetch theirs only when a topic is added, at one request per second.

Not yet compared, because we have no guides for them: application fees, lease disclosures, pets and assistance animals, mobile home parks.
