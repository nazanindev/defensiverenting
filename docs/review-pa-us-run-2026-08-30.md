# Review sheet: PA + US drafting run, 2026-08-30

12 drafts saved (9 pennsylvania, 3 united-states), all status=draft in the local DB,
nothing published. Every citation quote passed the verbatim guardrail; every page passed
the voice lint. This sheet collects what the drafting agents flagged for the human pass,
worst first per page. Fetched source text and save plans are under the session scratchpad
in `scratchpad/agents/<agent>/` if you want to see exactly what a quote came from.

## Systemic (affects several pages)

- **CourtListener is unfetchable by the toolbelt.** Their opinion pages return an empty
  HTTP 202 bot challenge; the render tier gets a CloudFront 403; the REST API needs auth.
  Three agents hit it. Consequences: Pugh v. Holmes is cited from Harvard's Caselaw Access
  Project scans (static.case.law) on the uninhabitable-conditions and landlord-entry pages
  (same official Pennsylvania State Reports text, reporter cite in locator), and was
  dropped entirely on the repairs, heat, and eviction pages, which lean on the AG guide
  (its footnote naming Pugh is quoted, so the case still shows). Follow-up: either teach
  the fetcher to pass the challenge or point the citation-research skill at static.case.law.
- **The AG Consumer Guide's URL drifts.** The dated wp-content paths 404 intermittently
  since the AG site redesign; pages cite the live undated path, one save went through a
  web.archive.org snapshot of the same URL. The guide itself is v1.1, June 2022. This is
  the poster child for the source-change monitoring roadmap item.
- **Rent Withholding Act scope.** The escrow route exists in cities of the 1st/2nd/2A/3rd
  class, not townships/boroughs. The PA repairs, heat, and uninhabitable pages say
  "Pennsylvania cities" and flag the limit; confirm the phrasing on each. (Also the reason
  the Shaler Highlands reply routed escrow through legal aid.)
- **Negative claims ("no statewide law says...")**: PA rent-increase stmt 1 (no rent
  control, no notice statute), PA cant-pay stmts 1-2 (no grace period, no late-fee cap),
  PA landlord-entry stmt 1 (no entry-notice statute), PA heat final stmt (no A/C
  requirement). All were asked for by the brief and all rest on absence of statute plus AG
  framing; official sources rarely state a negative. Confirm each reads as honest, not
  overclaimed.

## Time-sensitive (re-verify at publish)

- PA cant-pay: ERAP stated closed effective Oct 1, 2025 (cited from pa.gov).
- PA directory: LIHEAP entry says applications only in colder months, backed by "the
  2025-2026 season is now closed" notice; season status will flip.
- US heat: LIHEAP Clearinghouse counts (42 cold-weather / 21 hot-weather states) are
  point-in-time; LA County 82°F rule's complaint-based enforcement starts Jan 1, 2027
  (stmt 11 says "now requires"; consider adding the date).

## Per-page flags

**pennsylvania / uninhabitable-conditions** — stmt 10's city-class reading (Scranton as
2A, third class as the rest) derived from statute + Pugh; check. Dropped for depth, all
available in fetched text if wanted: USTRA retaliation bar (§ 399.11), PHRA/FHA
retaliation, Pugh's move-out remedy.

**pennsylvania / landlord-entry** — stmt 5 (emergency entry) is the softest claim on any
page: inferred from the AG's reasonable-access line, no PA source addresses emergency
entry directly. Quiet enjoyment cites Duff v. Wilson (1871) via static.case.law.

**pennsylvania / security-deposits** — stmt 5 interest payout: statute says yearly on the
lease anniversary; AG guide says end of third and subsequent years. Both cited; pick a
framing. Bond-in-lieu (§ 250.511c) deliberately omitted.

**pennsylvania / rent-increase** — Philadelphia examples framed as "check your city's
guide" pointers; confirm you like a state page naming one city.

**pennsylvania / eviction-defense** — stmt 13 phrasing: Rule 515 B(1) lets the landlord
request the order for possession after day 10 even if an appeal without a deposit was
filed; current wording implies otherwise. Left out for the cap: 30-day DV appeal window
(Rule 1002 B(2)).

**pennsylvania / cant-pay-rent** — nothing beyond the absence claims above; no partial-
payment statement exists because no primary source addressed the effect.

**pennsylvania / repairs-and-habitability, heat-not-working** — heat page cites 66
Pa.C.S. §§ 1523-1531 (Chapter 15, no sunset) and 52 Pa. Code §§ 56.100/56.111; the
Chapter 14 lead (§ 1406) was deliberately not cited: it sunset Dec 31, 2024.

**pennsylvania / resource-directory** — PUC citation is backed by an archive.org snapshot
(their site refused connections mid-run; quoted text matched the live page fetched
earlier). PHRC protected-class list is compressed from an overview paragraph. Dropped as
unbackable: 211 "helps with rent/utility bills specifically", PUC "informal complaint
stops a shutoff", AG "renters use it against landlords".

**united-states / uninhabitable-conditions** — stmts 1, 2, 8, 10, 11 lean on Cornell LII
(Wex, kind nonprofit) for "most states / varies" survey claims; stmts 5, 7, 8 use the
California AG's sheet for universally-phrased practical steps; stmt 3's mold sentence is
slightly broader than the EPA line ("people with asthma" vs "people with asthma who are
allergic to mold").

**united-states / repairs-and-habitability, heat-not-working** — "ask for an inspection"
(stmt 5 on both) is a fair reading but not verbatim in EPA/Blueprint sources. The White
House Renters Bill of Rights Blueprint is archived aspirational guidance and statements
mirror that ("federal guidance says owners should..."); confirm you're comfortable citing
bidenwhitehouse.archives.gov. CRS IF12823 fetched via archive snapshot (congress.gov
403s). Dropped as unbackable nationwide: "most states don't require A/C" (replaced with
the CRS extreme-heat line + "check your state's guide"), state-by-state withholding/
repair-and-deduct surveys, "many states ban retaliation for code complaints" (scoped to
the federal FHA ban).

## 2026-08-31 follow-up: constructive-eviction + uninhabitable retirement

The uninhabitable-conditions drafts were deleted (the topic was retired vocabulary from
2026-08-09; the resurrected prod topic row is gone and the alias stands). Their material
was redistributed into four re-promoted drafts; the earlier per-page flags for the two
uninhabitable pages above are now moot, and the repairs flags apply to the MERGED versions.

**pennsylvania / constructive-eviction** (new, 13 stmts) — anchored on the AG guide's own
constructive-eviction passage plus Pugh, Kuriger v. Cramer (heat shutoff as constructive
eviction) and Chelten Ave. v. Mayer (must actually move out), all via static.case.law.
Checks: stmt 9 attributes Pugh's approving block-quote of a Massachusetts case to
"courts", not the PA Supreme Court; stmt 11 (no eviction record unless the landlord files)
is built from two AG lines, read it as a whole; stmt 7's "you can owe rent for months
after you left" cites the AG's early-termination passage, which is adjacent rather than
exact. No day-count for "reasonable time" anywhere: no source gives one.

**united-states / constructive-eviction** (new, 13 stmts) — Cornell LII spine + federal
guidance. Checks: stmt 12 ("only a judge can order you out") rests on a Wex line whose
context says "in most jurisdictions"; the deliberate omission of any claim that
constructive eviction keeps your record clean (no primary source) is correct, don't add it.

**pennsylvania / repairs-and-habitability** (merged, 14 stmts) — absorbed the cities-only
escrow scope, the repair-and-deduct $6 example, and the percentage-abatement worked
example; "you can also sue" now lives only inside the AG quote, not as a body claim.

**united-states / repairs-and-habitability** (merged, 14 stmts) — absorbed the
annoying-vs-unsafe line, EPA lead reporting, stronger inspection phrasing, and state
retaliation. Cap-forced cuts from the old draft: the lease-promised-amenities statement
(weakest backing; the lease-included-A/C ground is still covered on the US heat page) and
the carbon monoxide statement. Restore either by swapping in the authoring tool if wanted.

Both repairs pages now carry a severe-end pointer with topic_ref constructive-eviction.
Migration 000032 (constructive-eviction topic) is in the repo, applied locally; prod got
the topic via promote and the migration will no-op there on the next deploy. Local still
holds the retired uninhabitable topic row because Boston's legacy local page references
it: the 2026-08-09 vocabulary script has never run against the local DB — worth a pass
someday to end the local/prod registry drift that caused this whole detour.

## After review

Publish decisions happen in the authoring tool as usual. Promote to prod is staged and
waiting on the prod DSN; dry run first, and the run reports created/reused/overwritten
before anything applies.
