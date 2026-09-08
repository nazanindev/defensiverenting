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

