# Review sheet: statewide drafting wave 1, 2026-09-08

Five states, 7 core topics each, drafted by session subagents over the MCP toolbelt
straight into the production DB as status=draft. Nothing published by this run. Every
citation quote passed the verbatim guardrail; every page passed the voice lint. This
sheet collects what the drafting agents flagged for the human pass, worst first per
page. Fetched text and save payloads for every page are under the session scratchpad
`scratchpad/agents/<state>-<topic>/` if you want to see exactly what a quote came from.

Order run: California, New York, Texas, Illinois, Washington.
Pennsylvania was drafted 2026-08-30 (see review-pa-us-run-2026-08-30.md) and is already
in prod: 8 drafts plus a published repairs page.

Prompt change shipped first: the draftagent system prompt gained a statewide-page rule
(commit bf5982a); the same rule is in the subagent brief for this run.

## California

### Systemic
- find_sources has no california seeds (nor any state); agents used WebSearch discovery.
- dca.ca.gov "California Tenants" guide is gone (404); the courts.ca.gov PDF mirror 403s;
  archive.org rate-limited (429) during the run. Pages cite OAG and selfhelp.courts.ca.gov
  instead, which cover the same ground.
- The OAG Know Your Rights PDFs extract with glyph artifacts mid-sentence, so some
  otherwise-good lines were unquotable and were dropped or swapped.

### Per-page flags

**california / security-deposits** (14 stmts, 36 citations) — Negative claim "no statewide
law makes your landlord pay interest on your deposit" rests on § 1950.5's silence plus the
OAG line that some cities require interest; no source states the absence. Small-claims
statement's "give up the extra amount or file in civil court" line comes from the courts
page's contractor/bad-check sections, not the deposit section; cited quotes only cover the
$12,500 limit. Dates verified in statute: cap since 2024-07-01; move-in photos for
tenancies from 2025-07-01; move-out photos since 2025-04-01; AB 414 amendment effective
2026-01-01. "Small landlord is a person, or an LLC owned only by people" simplifies
"natural person" (also family-trust settlors/beneficiaries). Bad-faith gloss "(on purpose,
without a real reason)" is editorial. Penalty example ($2,000 deposit + up to $4,000 =
$6,000) omits actual damages. Left out: six-month advance-rent carve-out, service-member
higher deposit, § 1946.7 domestic-violence early termination (consider adding), the § 1161
eviction exception to the inspection right.

**california / resource-directory** (14 entries, 32 citations) — Inferred: "Every county
court has one" (self-help centers; page only has a county locator, soften to "most"); the
LawHelpCA "enter your county and your problem" line describes the site, not a quote; the
CRLA scope line is drawn from subpage headings; the AG "does not handle individual tenant
cases" is inferred from its pointer to LawHelpCA. Quoted-verbatim negatives: CRD does not
help with repairs/deposits/evictions; DRC does not represent in eviction cases.
Time-sensitive: DRC intake hours and "online form temporarily unavailable"; LAFLA hours;
DRE guide "2026 Edition"; CRD 1-year filing deadline. Scope: BayLegal and LAFLA are
regional examples; cut if you want strictly statewide. hud.gov/findacounselor fetched this
time (the earlier HUD block seems lifted). Left out: no statewide fair-housing-council org
page exists (CRD covers discrimination); LAAC is a membership body, not a help line.

**california / eviction-defense** (14 stmts, 40 citations) — Stmt 14 record masking:
"becomes public only if your landlord wins" merges § 1161.2(a)(1)(E) and (F) as amended
by AB 2304 (eff. 2025-01-01); confirm phrasing. The "7 years" screening line rests only
on courts guidance. Stmt 13 (stay of execution: 1 court day, 24 hours, up to 40 days) is
guidance-only, no statute. Stmt 9 defenses list mostly guidance plus § 1942.4.
Time-sensitive: 10-court-day answer deadline (AB 2347, eff. 2025-01-01); § 1161 is the
SB 611 text operative 2025-02-01; § 1946.2 is the AB 1529 text eff. 2026-01-01 with a
2030-01-01 sunset (stated in stmt 4); filing fee "$240 to $450" and jury fee "$150" from
guidance. Simplifications: stmt 4 names 3 of 9 TPA exemptions and compresses the
single-family owner test to "owned by an individual"; stmt 6 lists 4 no-fault reasons
without the remodel/owner-move-in conditions. Negative claim: "a text or a phone call is
not a legal notice" inferred from § 1162's exclusive list. Left out: substituted-service
20-day and Safe-at-Home 15-day answer deadlines, § 1179 relief from forfeiture, § 1174
pay-in window, personal-property reclaim, CARES Act 30-day notice, local just-cause
ordinances (pointer only).

**california / rent-increase** (14 stmts, 33 citations) — Time-sensitive: stmt 2 CPI caps
(8.7% LA/Orange, 8.8% SF area, 8.6% other counties for 2026-08-01 to 2027-07-31) read from
the AG chart on 2026-09-08; refresh every August; Riverside and San Diego figures omitted.
Chart quotes are terse table fragments. Stmt 9 LA example (3% for 2026-07-01 to
2027-06-30) is from the AG local chart only, not checked against LAHD. Stmt 5 "if your
landlord never gave you that notice, the cap still protects you" is inferred from
§ 1947.12(d)(5)'s "both of the following" structure. Stmt 8 vacancy-decontrol change-of-
terms carve-out cites Costa-Hawkins § 1954.53(a)(1), which strictly governs locally
controlled units; soften or drop the last sentence. Stmt 10 "write to your landlord" is
editorial advice. Stmt 13 retaliation simplifies § 1942.5(a) triggers. Stmt 12 UD-105 box
numbers (3i(4), 3i(5)) may renumber. Sunset 2030-01-01 stated in stmt 14. No fixed-term
lease statement (no fetched primary source; courts rent-increase page 404s). Left out:
mobilehome rules, § 1947.13, Section 8 nuance, emergency price-gouging caps.

**california / landlord-entry** (14 stmts, 24 citations) — Stmt 4 "business hours = M-F 8 am
to 5 pm" rests on one Santa Clara court PDF; the statute does not define the term (body says
so). Stmt 9 "rules apply even if your lease says otherwise" rests only on that PDF; § 1954
has no express anti-waiver clause; soften or drop. Stmt 10 log/photos/witness advice is
editorial; the LA County page backing it is dated 2013. Stmt 11: § 1940.2's $2,000 penalty
needs "significant and intentional" entries aimed at making you leave; the $6,000 example
is agent arithmetic. Stmt 12 "decides the same day or by mail" is not in the quoted text.
Stmt 14 uses § 1942.5(d) general clause; the 180-day presumption was left out on purpose.
Time-sensitive: small claims cap $12,500 (SB 71, 2024). Left out: § 1954(a)(5)-(6)
submeter/balcony entries.

**california / cant-pay-rent** (14 stmts, 40 citations) — Stmt 13 "state rent relief is
closed" rests on HCD filing ERAP under "Programs: Archived" in past tense; no page says
"closed"; re-check at publish. Stmt 1 "no statewide grace period" is a negative claim
backed only by DRE guidance about lease practice. Stmt 2 "no statute caps late fees" is
inferred from § 1671(d) plus DRE; the "courts have ruled" line rests on a DRE footnote
naming Orozco v. Casimiro, not the opinion (static.case.law lacks Cal.App.Supp). Stmts 4,
7, 8, 9 (part payments, pay-and-stay, defective notice, acceptance after deadline) rest on
court self-help and DRE guidance only; statute agrees but is not quoted. Time-sensitive:
TPA sunset 2030-01-01 (stmt 10); CalWORKs Homeless Assistance amounts, form CW-42 rev
9/23 (stmt 13). Stmt 14 (211, tagged help-lines) may read as a help section; drop if so.
Left out: § 1162 service methods, the 2025 Social Security hardship defense (belongs on
eviction-defense), bounced-check fees, CARES Act 30-day notice.

**california / repairs-and-habitability** (14 stmts, 44 citations) — Stmt 12 (§ 1942.4)
statute says 35 days; the Santa Clara court handout says 30; statute followed, handout not
quoted for that number. Stmt 10 "set the withheld rent aside in a separate account" is
practical advice with no source. Stmt 6 "your notice date starts both clocks" is a
synthesis of § 1942(b) and § 1942.5(a)(1). Stmt 9 "$4,000 max in a year" is agent
arithmetic from "twice" plus "one month's rent". Stmt 3 time-sensitive: stove/refrigerator
rule (AB 628) effective 2026-01-01 for leases entered or renewed from that date; exempt
housing types not listed. Stmt 11 compresses CCP § 1174.2's 5-day window (longer if served
by mail). Stmt 13 omits the once-per-12-months limit in § 1942.5(b). Statements use
Markdown bullets for lists; check rendering in the playbook layout. Left out: § 1942.3
60-day presumption, § 1941.3 locks, attorney fees, local ordinances.

## New York

### Systemic
- find_sources has no new-york seeds. nysenate.gov fetched for some sections live and for
  others only via archive.org snapshot; check snapshot dates on any statute quote.
- The AG "Changes in NYS Rent Law" PDF carries hidden control characters around
  ligatures, so some quotes fail the verbatim check; agents worked around it.

### Per-page flags

**new-york / security-deposits** (13 stmts, 30 citations) — Statute vs guidance: the AG
HTML guide still says the 14-day return and inspection rules apply only to non-regulated
units; GOL § 7-107 as amended (nysenate.gov revision dated 2025-11-21) and HCR Fact Sheet
#9 extend them to rent-stabilized tenants from 2025-11-15. Stmt 2 follows statute + HCR;
confirm the amendment is in force. Stmt 2 uses topic_ref "rent-stabilization" (a
registered topic with no page yet); drop if that page will not exist. Time-sensitive:
small-claims limits ($10,000 NYC; $5,000 Nassau/western Suffolk and city courts; $3,000
town/village). Soft advice: "give your new address in writing" (stmt 8), "first write to
your landlord" (stmt 13). Stmt 4 presents the 1% admin fee inside the 6-plus-unit rule;
strictly it is conditioned on the deposit actually being in an interest-bearing account.
Left out: § 7-108(1-a) exclusions (rent-controlled, seasonal, senior/assisted living,
owner-occupied co-ops), interest payment options, NYC rules (pointer).

**new-york / resource-directory** (14 entries, 34 citations) — Stmt 4 ORA coverage: the
quote lists Nassau, Rockland, Westchester, Ulster; "New York City" is inferred from the
page's borough office list. Stmt 6 DHR protected-trait list is editorial gloss, not
quoted. Stmt 7 Emergency Assistance: "shelter arrears / utility arrears" is on the page
but not in the quote; "(in New York City, HRA)" is not on the page. Time-sensitive: HEAP
1-person income limit $3,473 is the 2025-2026 figure from an archive snapshot; DHR 3-year
filing deadline applies to acts from 2024-02-15. LSNYC hours (9:30 to 4) taken from its
own site; LawHelpNY says 10 to 4. TPU entry is URL-only (no phone on page). HEAP and DPS
both tagged utility-shutoff-protection; HEAP may fit rent-assistance-programs better.
HUD counselor phone rests on the CFPB page. Fetched but unused: lasnny.org (swap in for
LSNYC if upstate coverage preferred); housingjusticeforall.org excluded (campaigns, no
concrete help).

**new-york / rent-increase** (14 stmts, 34 citations) — Time-sensitive Good Cause figures
(stmts 8, 9): CPI 3.38% downstate / 3.15% rest, from the HCR notice posted 2026-07-16 and
reposted 2026-08-17; the FMR table is captioned FFY 2025 and one header still reads
"Effective [Month, Day] 2026" (HCR placeholder). Page hard-codes 8.38% / 8.15% and
$7,130; refresh every August. Stmt 7 opted-in locality list is "as of 2026-05-04";
Poughkeepsie and Newburgh listed as Good Cause opt-ins even though their ETPA adoptions
were voided (independent laws, but confirm). Statute vs guidance: § 226-c says "equal to
or greater than 5%"; AG guide says "more than 5%"; statute followed. Stmt 6 combines
§ 226-c non-renewal notice with the AG's month-to-month line (AG cites §§ 232-a/232-b).
Stmts 1 and 2 (no cap; lease locked mid-term) rest only on the AG guide. Stmt 12
paraphrases the § 223-b burden; the (3) money/fees quote is not cited. Stmt 9 "most
opted-in localities set it at 1 unit" read off the HCR table. rent-control tagged on
stmts 1 and 13. Left out: rent control (pre-1947), MCI/IAI, manufactured-home 3% cap,
NYC stabilization detail (pointer).

**new-york / cant-pay-rent** (14 stmts, 32 citations) — Time-sensitive: ERAP closure
dates (2023-01-20; portal 2025-11-17) and the HEAP "2025-2026 emergency benefit opened
2026-01-02" line come from archive snapshots; the ERAP citation URL is a web.archive.org
URL, not otda.ny.gov. Stmt 4: Good Cause rider "until 2034-06-15" comes from the NB
marker on RPAPL 711(2); "a phone call or text is not a rent demand" inferred from
"written demand". Stmt 9 merges 749(2)(a) and 749(3); RPAPL 751 deposit-plus-costs left
out. Stmt 10 "call the police and ask for a court order" is practical advice; the
$1,000-$10,000 figure is the civil penalty, not money to the renter (RPAPL 853 treble
damages not included). Stmt 11 calls a utility shutoff a "violation" without saying it
is a petty offense. Stmt 7 no-late-fees-for-pay-and-stay rests on 702(1)'s rent
definition. Stmt 13 Emergency Assistance + 211 rests on OTDA guidance only. Left out:
bounced-check fee limit, RPAPL 753 one-year stay, RPAPL 732 answer window (on
eviction-defense), NYC and stabilization specifics.

**new-york / eviction-defense** (14 stmts, 39 citations) — No sealing statement: no
official source frames the absence of a sealing statute, so the page covers only RPL
§ 227-f (screening-based denial); add a "no sealing law" sentence only with a source.
Answer deadline: RPAPL § 732's 10 days applies only where Appellate Division rules adopt
it; stated as the statewide nonpayment rule without that caveat; NYC guidance confirms 10
days there. Holdover answer: NYC guidance allows an answer demand 3 days before the
hearing if served 8+ days out; statement says "at the hearing date" only. Good Cause:
exemptions summarised (15 categories in § 214); "very high-rent units" glosses the 245%
FMR threshold; "(2024)" comes from the revision date; the § 214 quote is only the
owner-occupied line. Time-sensitive: § 711(2) rent-demand text and § 226-c are the
"until 2034-06-15" versions. Single-source: "accepted rent after the move-out date"
defense and "landlord presents proof first" rest only on the 2019 nycourts holdover PDF.
Retaliation statement summarises § 223-b(1)(a)-(c) broadly. § 753 stay: statute "may",
statement "can also stay"; deposit requirement from § 753(2) not separately quoted. Left
out: RPL § 227-a, § 232-b, NYC stabilization and marshal rules, mobile-home notices.

**new-york / repairs-and-habitability** (14 stmts, 52 citations) — Heat statement rests on
a Westchester County health page restating Property Maintenance Code 602.3 (no primary
code text fetched: ICC 403s, DOS PDFs fail or truncate); the 2025 PMCNYS took effect
2025-12-31 and its draft moved the 65°F exception into 602.2; confirm the Sept 15 to May
31 rule survived. MDL scope written as "cities with 325,000 or more people (New York
City)"; Buffalo is now under 325,000; decide whether to name it. Stmt 2 says "apartment
building" without the 3-or-more-families definition. RPAPL 7-D "since January 2024"
dating from LawNY plus the nysenate revision date; Nassau and Suffolk are excluded and no
alternative route given. Negative claims "no set right to stop paying rent" and "no
statewide repair-and-deduct statute" rest on AG and legal-aid framing. Utilities
statement: RPL 235-a(1) quote covers water; electric/gas deduction comes from the Public
Service Law cross-reference not in the quote. Park West fetched at static.case.law/ny-2d.
Left out: MDL § 302-a rent-impairing violations (NYC-only), HPD violation classes (NYC
page).

**new-york / landlord-entry** (14 stmts, 32 citations) — Stmt 1 negative claim "no
statewide law sets a notice period" is framed by the AG guide's reasonable-notice line
plus Cornell's Tenants Advocacy Program; no source says "no statute". The AG guide is
cited from a City of Beacon-hosted copy because ag.ny.gov's page truncates at 60k before
the privacy section (nyc.gov also hosts the same PDF). Stmt 3 (what "reasonable" means)
rests only on Cornell Law's clinic page; confirm it counts as legal aid. Stmt 12 "outside
the city, the local court can do the same" inferred from RPAPL § 768(1)(b). Stmt 7 ties
quiet enjoyment to § 235-b, which is a gloss (quiet enjoyment is common law). Stmt 8
Barash is a 1970 commercial-lease case. Stmts 5 and 14 are NYC-only rules (5-day showing
rule, 24-hour/1-week rule) stated as such; NYC ABCs dated 2024-01. Small claims limits
from a 2025-01 archived page. AG PDF quotes embed a cp1252 apostrophe (U+0092) verbatim;
HCR Fact Sheet #14 quotes trimmed around ligature control chars. Left out: RPL § 235-d,
§ 227-e, tenant-changes-own-lock (single soft source).

## Texas

### Systemic
- **statutes.capitol.texas.gov serves whole chapters**, and every section anchor and
  GetStatute.aspx URL returns the full Chapter 92 page, which fetch_source truncates at
  60,000 characters inside § 92.016, before Subchapter C. The archive.org snapshot
  truncates identically. So Texas statute quotes largely rest on the texas.public.law
  mirror (publisher field says "mirror" so it is visible). Toolbelt follow-up: raise or
  anchor-window the 60k cap so the official site can be cited; then repoint.
- Texas AG tenant-rights page 404s; the State Law Library guides (guides.sll.texas.gov)
  and TJCTC self-represented pages stand in.

### Per-page flags

**texas / security-deposits** (14 stmts, 30 citations) — Statute quotes rest on the
public.law mirror except § 92.001(4); spot-check against the official site. Stmt 1 "no
cap" is a negative claim backed only by two nonprofit pages; the "unless public or
subsidized housing" carve-out is from texaslawhelp only. Stmt 7 "later of move-out or
forwarding address" combines §§ 92.103 and 92.107 as texaslawhelp reads them; the statute
does not say "whichever is later". Stmt 9 second citation quotes only the "tenant owes
rent" prong. Stmt 13 evidence list is editorial. Stmt 2 fee-in-lieu "usually not
refunded" simplifies the statute's insurance-only condition. Time-sensitive: fee-in-lieu
effective 2021-09-01; justice court limit $20,000. Left out: § 92.1031 (never moved in),
§ 92.106 records duty, reletting fees.

**texas / cant-pay-rent** (13 stmts, 35 citations) — § 92.019 late-fee quotes come from
the enrolled S.B. 1414 (2019) bill text because the live section would not fetch; the
quotes carry bracketed struck-through redline text ("[charge]", "[one]") verbatim; swap
the URL once the section fetches. Stmt 7 pay-and-stay is new law (S.B. 38, eff.
2026-01-01); statute keys on "not late before the month the notice is given";
texaslawhelp reads it as "first time late this lease term"; statute wording used. The old
"landlord need not accept late rent after notice" rule was repealed § 24.005(i) and is not
used. Late-fee caps apply only to leases entered or renewed from 2019-09-01 (not stated).
Utility submeter exception compressed to "notice at least 5 days before". Stmt 5 partial
payments inferred from "any portion / at least part" wording; no statute addresses it
directly. Writ of reentry "the same day" is a gloss on "ex parte". Time-sensitive: Texas
Rent Relief closed summer 2023; CEAP and Help for Texans; two assistance statements
included per the assignment, cut one if it duplicates Local Help. Left out: § 92.019(d)
waiver rule, rent acceleration.

**texas / resource-directory** (14 entries, 33 citations) — TLSC eviction line 855-270-7655
is cited from TDHCA's page (TLSC's own site does not list it; TDHCA calls it a
"pandemic-related" line, may be stale). Disability Rights Texas page carries a "network
disruption, phone lines closed" banner; verify before publish. Lone Star Legal Aid county
list and LANWT/TRLA example cities are inferred from the TexasLawHelp referral directory,
not the orgs' own pages. PUC entry's "does not regulate landlords who resell utilities" and
"wrongful shutoffs" are not on the fetched page; trim. TWC entry omits the "more than three
properties" threshold and uses the agency general line. TexasLawHelp live-chat hours come
from tlsc.org. TDHCA merged into one entry tagged rent-assistance-programs though CEAP is
LIHEAP. AG entry's "does not represent you in court" is editorial. TJCTC "answering an
eviction case" packet inferred from a nav link. Texas Tenants' Union fetched but dropped
(members-only appointments).

**texas / eviction-defense** (14 stmts, 36 citations) — Retaliation statement rests on the
State Law Library page (quotes § 92.331 verbatim) plus texaslawhelp; the statute itself is
past the 60k truncation; "$500 penalty" from the SLL summary of § 92.333. Records
statement "no Texas law lets you seal or erase it" is a negative claim on one texaslawhelp
article (reviewed 2023-07), which says "no way to expunge". CARES Act statement phrased
"may require" because texaslawhelp notes disagreement on whether it still applies;
§ 24.005(c-1) (2026) only delays writ service. Time-sensitive: SB 38 changes (delivery,
notice to pay or vacate, appeal affirmation, summary disposition) apply to cases filed
from 2026-01-01; dated on the notice and pay-and-stay statements, not on summary
disposition or appeal; consider adding. "Many Texas leases set only 1 day" is from
texaslawhelp. Default-judgment motion 5-day deadline from texaslawhelp only (Rule 510 PDF
truncated after 510.15(c)). Lockout statement collapses § 92.0081 notice rules; penalty
math ignores the "less any delinquent rent" offset. Rule 510 cited as kind "regulation"
(Supreme Court of Texas Misc. Docket 25-9096). Left out: § 24.006 attorney's fees,
foreclosure notice, subsidized-housing good cause, supersedeas appeal, writ expiry.

**texas / landlord-entry** (14 stmts, 37 citations) — Stmt 14 criminal trespass: "not
settled" and "depends on what your lease allows" are agent framing, unsourced; cut to a
bare pointer or drop. Stmt 6 Clark v. Sumner (1977, 559 S.W.2d 914): the $1,735 / $1,200
figures are in the opinion but the cited quote is the holding sentence; the entry there
was letting a dog in. Stmts 7, 8, 9, 13: every Subchapter D and § 92.331 claim (rekey in
7 days, 7/3-day compliance, 1 month + $500, 6-month window) rests on AG / State Law
Library / legal-aid restatements because the statute page truncates; sections named in
body. Stmt 13 "a written complaint about entry counts" inferred from the "right under the
lease" clause. Stmt 3 "24 hours is what renters usually ask for" from a texaslawhelp
example. Stmts 2, 3, 5 describe the TAA lease as the common form via texaslawhelp. No
Austin pointer (no Austin entry ordinance found).

**texas / rent-increase** (13 stmts, 27 citations) — Stmt 2 rent-control ban (Local Gov't
Code § 214.902) is cited only to two State Law Library pages (one mislabels it as
Property Code; body names the right code). §§ 92.331(b)-92.334 cited via SLL, AG, and
texaslawhelp; the enrolled S.B. 630 (2013) bill text covers § 92.331(a) only. Stmt 4
"30 days before a rent change" on month-to-month is texaslawhelp's reading of § 91.001
(which governs ending the tenancy); body says so. Stmt 8 "an increase your lease already
allows" is broader than § 92.332(a)(1). Stmt 13 "tell your housing authority before you
pay more" is advice. Stmt 6 LIHTC good-cause rests on one nonprofit source. Time-sensitive:
justice court $20,000 limit (form dated 2022-05). §§ 92.3515 and 92.0135 were off-topic
and dropped.

**texas / repairs-and-habitability** (14 stmts, 58 citations) — Subchapter B statute
citations point at enrolled bill texts on capitol.texas.gov (SB 1259 2023, SB 1367 2015,
HB 177 2007, SB 1678 1997, SB 630 2013), not the codified page; SB 1678 quotes carry
page-line prefixes ("11-20") verbatim. §§ 92.052(b)-(d), 92.056(e), 92.0561(e)-(k),
92.0563(a)-(d), 92.331(b), 92.054, 92.058, 92.333 rest on guidance only. Justice court
cap: statute ($20,000, SB 1259 eff. 2023-09-01, Gov't Code § 27.031) taken over the 2021
TJCTC packet and AG page which still say $10,000. Stmt 9 carries three waits (none / 3
days / 7 days), backed only by the Austin Tenants Council brochure. Stmt 11 "trial 10 to
21 days after you file" (TJCTC packet, TRCP 509) vs § 92.0563(d) "hearing 6 to 10 days
after service"; pick one. Stmt 1 "no general warranty of habitability" inferred from
the § 92.052 health-or-safety standard. Stmt 7 "behind on rent ends the repair duty" via
SLL guidance. Austin pointers (3-1-1, 68°F heat) from the city page and a brochure with
2022 archive links. Left out: § 92.0562, § 92.055, § 92.334, § 92.062.

## Illinois

### Systemic
- find_sources has no illinois seeds. Old-style ilga.gov URLs (ilcs3.asp, fulltext.asp)
  404 since the 2025 redesign, and the archive.org fallback returned the wrong act; the
  working patterns are `/Legislation/ILCS/Articles?ActID=..&Print=True` and
  `/Documents/legislation/ilcs/documents/{code}.htm`. Seed these.
- The Illinois AG Landlord and Tenant Rights PDF is dated 2024-01 and still names the
  repealed Retaliatory Eviction Act; it predates the 2025 Landlord Retaliation Act.

### Per-page flags

**illinois / rent-increase** (14 stmts, 27 citations) — Stmt 4 "month-to-month = 30 days"
rests on 735 ILCS 5/9-207, a termination statute; "landlord raises rent by ending the
tenancy" is the AG's and ILAO's reading; ILAO's newer page says "often at least one full
rent period". Stmt 6 "no statewide renewal-notice law outside Cook County" is a negative
claim on ILAO alone. Stmt 14 renewal-fee ban (765 ILCS 705/35, P.A. 104-479) has a delayed
effective date of 2027-01-01; consider holding the statement until then. Stmt 1 "no cap"
rests only on the 2024-01 AG PDF. Stmt 12 voucher 60-day rule rests on ILAO; the HUD
quote covers only the 30% income share. Stmt 13 names Chicago, suburban Cook, Evanston,
Oak Park, Mt. Prospect from ILAO. Left out: week-to-week 7-day notice, § 9-211 delivery,
mobile home park 90-day rules.

**illinois / landlord-entry** (14 stmts, 34 citations) — Stmt 1 "no statewide law sets
notice" is framed by ILAO (access by agreement or lease, list of local rules) and the AG
(check municipal ordinances); neither says "no state law"; 765 ILCS 705 confirmed to have
no entry provision. Case law fit: Blue Cross (100 Ill. App. 3d 647) is a commercial lease
case for the quiet-enjoyment rule; Home Rentals and Applegate are constructive-eviction
cases about conditions, not entry. Stmt 12 "Cook County" supplied for "counties over
3,000,000". Stmt 2 "such as 24 hours" is editorial. Stmts 2, 5, 6 rest on ILAO only.
Time-sensitive: Retaliation Act effective 2025-01-01 (sec. 20 amended 2025-08-15). The
Retaliation Act does not list entry as a retaliatory act and the page does not claim it.
Safe Homes Act and 765 ILCS 705 fetched via archive.org snapshot. Left out: IDHR's Safe
Homes summary-of-rights lease page (from 2026-01-01), the suburban Cook unlawful-entry
defense, sec. 15(c) rekey liability (stated, only (b), (e), (f) quoted).

**illinois / security-deposits** (14 stmts, 36 citations) — Statute vs guidance on
coverage: the AG sheet and older ILAO text say the Return Act covers buildings of 5+
units; the current 765 ILCS 710/1 text (P.A. 103-224, eff. 2024-01-01, fetched via
archive snapshot) has no unit threshold and ILAO's newer text agrees. Stmt 3 follows the
statute ("since 2024-01-01 every landlord"); confirm against P.A. 103-224 before publish.
Stmt 1 "no statewide law limits the amount" rests on AG + ILAO framing. Time-sensitive:
interest rate 0.01% for 2017 to 2024 leases from ILAO's table (2023-11); 2025+ rates not
listed; small-claims $10,000 from 2020 court PDFs. Stmts 11, 12, 14 rest on ILAO only.
Stmt 14 "usually cancel it" paraphrases "in almost all circumstances"; stmt 12's 45-day
demand-letter framing is ILAO practice. One ILAO quote trimmed around an inline glossary
insert. Left out: Chicago-only 30-day limitation note, 705/16, 705/5(b).


## Run stopped 2026-09-08

Nazanin stopped the run for usage cost during Illinois. Illinois eviction-defense,
repairs-and-habitability, and resource-directory agents were killed; cant-pay-rent
reported a save just before the kill. Check the prod queue for which Illinois cells exist.
Washington was never started. Remaining cells for wave 1: the Illinois gaps plus all 7
Washington topics.

## Resumed pass (2 agents at a time, Sonnet, 20-call cap, 12 statements)

**illinois / eviction-defense** (12 stmts, 15 citations, all statute) — Stmt 6
habitability defense cites § 9-106's general "raise any defense" clause; the warranty
itself is case law (Jack Spring), not quoted. Stmt 11 Chicago pointer reuses § 9-101 to
frame the absence of a statewide lockout penalty; no ordinance cited. § 9-118 and § 9-110
fetched and discarded (narrow special cases). No utility-shutoff statement (not verified
within budget). ilga.gov ActID URL for 765 ILCS 721 returned HTTP 500; per-section
documents URLs worked. Working payload.json predates the lint fixes.

**illinois / repairs-and-habitability** (12 stmts, 16 citations) — Repair-and-deduct
(765 ILCS 742: $500 or half a month, 14-day notice, exclusions) and utility-shutoff
protection (765 ILCS 735) rest entirely on ILAO guidance: every ilga.gov
"Articles?ActID=" full-text URL returned HTTP 500 this pass; only the per-section
documents pattern worked (used for 721/95). "No statewide habitability statute" sourced
to Glasoe v. Trinkle's own language. Retaliation "within 1 year" presumption is stated
but not quoted (only the prohibition and the 2025-01-01 date are cited). $900 rent →
$450 cap example is agent math. Case-law page locators read off static.case.law
page-break markers; spot-check. Title changed to "Rental Repairs in Illinois" (lint
rejects "Habitability" in titles).
