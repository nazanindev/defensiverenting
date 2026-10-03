# State page drafting brief (wave 5, 2026-10-02)

You draft ONE statewide renter guide page and save it as a DRAFT in the RenterLaw database with the `mcp__defensiverenting__*` tools. A person publishes it later. You never publish.

Before you start, read these files with the Read tool:
- /Users/nazimi/Dev/defensiverenting/docs/topic-map.md (what belongs on your page and which concept tag each statement gets)
- /Users/nazimi/Dev/defensiverenting/.claude/skills/editorial-voice/SKILL.md (how every renter-facing sentence is written)
- /Users/nazimi/Dev/defensiverenting/.claude/skills/state-review-loop/common-rules.md (the rules reviewers hold every statement to; draft so each statement passes them: names its own subject, every number in its own quote, win-and-pays line and worked examples cite editorial, no 'contact legal aid' advice sentences, every condition kept)

## Budget: HARD LIMIT

- At most 35 tool calls in total, counting every search, fetch and save attempt. Plan your sources first, fetch each one once, and stop researching when you have enough for the page.
- When you are at 30 calls, stop researching and save with what you have. A shorter page that saves beats a longer page that never saves.

## Page shape: set before you draft

- 10 to 14 statements. About 700 to 900 words in total. Not more.
- One claim per statement. Aim for about 80 words; the save rejects over 120, above reading grade 10, or hard words of 4+ syllables (an official term may stay only with a plain gloss in parentheses right after it). Keep it short by splitting or cutting a fact, never by using harder words. A rule, its exception and what the renter can do about it are separate statements.
- Order the statements the way the renter acts: what the law says, what to do first, then next, what happens if the landlord ignores it, what happens in court.
- Only what fits this page's situation (see the topic map "Covers" and "Not here" columns). If a true fact belongs on another page, leave it out.
- Tag each statement with the concept slug from the topic map whose question it answers (field `concept`). Leave step-by-step procedure untagged when no concept fits. Never invent a slug.
- No "where to get help" list on a playbook. Local help is its own page (resource-directory).

## Sources

- Every statement cites at least 1 source you read with `fetch_source` in THIS session.
- Long sources come back in parts. When `truncated` is true, call again with `offset` = `next_offset` only if the section you need is further on. Prefer a per-section URL over a whole chapter when the site has one. The save rejects any quote that is not verbatim in the fetched text.
- Order of preference: the state legislature's official statute site, then state regulations, then the state attorney general or housing agency, then court self-help pages, then state legal aid.
- Never cite Nolo, Justia, FindLaw, Avvo, law firm blogs, Zillow, Apartments.com, landlord blogs, or web.archive.org. You may read them to learn what a rule is called, then cite the official source.
- Text that came through a headless render is still the live page: you may cite it. Only a snapshot (web.archive.org) is not citable.
- If `fetch_source` returns text with `via` set to a snapshot, do not cite it. Find another source or drop the claim.
- The citation's url must be exactly the url you fetched (keep or drop "www." exactly as fetched).
- For a statute or regulation, quote the WHOLE subsection the locator names, from its marker to its end. For guidance, quote the paragraph.
- THE BODY MUST BE BACKED BY ITS OWN QUOTE. A reviewer will read each statement next to the quote stored on it and nothing else. Every fact in the body (every number, deadline, amount, condition, actor) must be in that statement's own quote. Write each sentence from a quote you already have. If you cannot back a fact from a quote, cut the fact.
- When a rule applies only to some places (certain cities, counties, sizes of building), say which places in the same statement. Never "some cities".
- When the state has no statute on a point, write "No [state] law sets..." only if an official source says so, and cite it. Never infer a rule from silence. If you find no rule, say nothing on that point.
- Never write "Some cities add stricter rules" or any local-rules line unless a fetched source says so for this state.
- Lessons from earlier states, which reviewers caught:
  - A statute that sets notice to END a tenancy is not a rent increase notice. Never write "notice before raising the rent" unless a source says so.
  - Keep every condition the quote attaches ("unless otherwise agreed in writing", "during winter", "for repairs", "beyond the landlord's control"). A rule that applies only in one branch of a statute must say which branch.
  - Each statement is shown alone on other pages. Never use words that point to another statement: "This", "These rules", "In that case", "Either way", "instead", "still", "also", "the same", "the problem", "your notice", "those reasons". Name the rule, the problem or the notice in the statement itself.
  - "Wrongfully" means "without a legal right", never "unfairly".
  - Keep qualifiers exactly: "materially", "reasonable", "lawfully served", "in good faith". Dropping one is the most common reviewer catch.
  - When the quote lists several items (reasons, triggers, facilities), name them all or say "for example" or "among other reasons". Never present 2 of 7 as the whole list.
  - Do not add small extras the quote does not say ("even for a short time", "free", "even if you owe rent", examples).
  - A doubled or tripled award must match exactly what the quote multiplies (the amount withheld is not the whole deposit).
- A statement that gives the site's own safety advice with no law behind it, a worked dollar example, a risk warning, or the line "You get this money only if you win your case and your landlord pays." cites a citation with kind "editorial" and no URL or quote, in addition to any law citation.

## Voice (the save runs a lint and rejects violations; fix what it lists and save again)

The editorial-voice skill has the full rules. The ones that trip drafters most:
- No em dashes. Sentences under 25 words. Numbers as digits.
- Banned legal words: void, unenforceable, waive, remedy, pursuant, provision, terminate, premises, dwelling, shall, exempt, exemption, exception. Say what happens in plain words.
- Official terms (judgment, mediation, writ, grace period, normal wear and tear, rental assistance, presume/presumption) need a plain-words explanation in the SAME statement.
- Any percentage or multiplied amount gets a worked dollar example.
- One deadline per statement, and say what starts the clock.
- Risky steps (ending the lease, moving out and stopping rent, withholding rent, repair and deduct) must say the risk in the same statement. Approved wording: "If a court later disagrees, you can owe the rent and face eviction. Get legal help first."
- Police: never tell the renter to call the police. Offer it only as their choice, next to another route (legal aid, photos, write it down), unless the sentence is about immediate danger.
- Utilities: first mention says which: "utilities (water, electric, gas)".

## Title and intro

- Title is the renter's search question, with the state as the place. Use the existing state pages' pattern ({State} for DC is "Washington, DC"):
  - cant-pay-rent: "Can't Pay Rent in {State}: What Are My Options?"
  - eviction-defense: "Facing Eviction in {State}: What Can I Do?"
  - landlord-entry: "Landlord Entering Without Notice in {State}: What Are My Rights?"
  - rent-increase: "Rent Increases in {State}: What Are My Rights?"
  - repairs-and-habitability: "Landlord Won't Make Repairs in {State}: What Can I Do?"
  - resource-directory: "Where Can I Get Rent Assistance or Eviction Help in {State}?"
  - security-deposits: "Security Deposit Not Returned in {State}: What Can I Do?"
- Intro: 2 or 3 short sentences saying what the page covers. Same voice rules. The intro makes NO legal claim (no "Montana has no rule on X", no numbers, no deadlines): it only names the topics the page covers.

## resource-directory only

- page_kind "directory". 8 to 12 entries, one organisation or government program per statement: who it helps and how to reach it (phone or website).
- Cite that organisation's own site (kind "nonprofit") or the government program page (kind "gov_guidance"). Never an aggregator. If you cannot fetch an organisation's own page, leave it out.
- Cover: statewide legal aid (and its intake line), the state's court self-help center, the attorney general's consumer line if it takes renter complaints, 211, and any state rent-assistance or eviction-diversion program that is open now. Say "check if it is open" only if the source says funds run out.
- "free", "apply online", "find your local office" each need a quote that says it.

## Reviewer notes

Use `reviewer_note` on a statement only for a real doubt about that claim: a reading inferred from silence, a simplified legal test, a figure that will go stale, a source you could not open. Never in the body.

## Save

Call `save_draft_playbook` with jurisdiction_slug and topic_slug exactly as given. If the save is rejected, fix only what it lists and save again. Stop after a successful save. Do not spawn subagents.

## Your final message (under 150 words, no statute text)

- saved playbook id, statement count, word count estimate
- sources you wanted but could not fetch
- anything a lawyer should look at (one line each)
- tool calls used

## Official code sites for this wave (all tested 2026-10-02)

Section ids on some sites are not guessable. To find one, use WebSearch with allowed_domains set to the official host. That counts as one tool call.

- Alaska (slug alaska): akleg.gov returns 403 on large ranges at times: fetch single sections or ranges of 2-5 sections. Use the PRINT url form only (the plain statutes.asp url returns a title list). Landlord and Tenant Act AS 34.03.010-34.03.380: https://www.akleg.gov/basis/statutes.asp?media=print&secStart=34.03.010&secEnd=34.03.380 (about 64k chars, read on with offset). Eviction (forcible entry and detainer) AS 09.45.060-09.45.160: https://www.akleg.gov/basis/statutes.asp?media=print&secStart=09.45.060&secEnd=09.45.160. A single section works the same way (secStart=secEnd).
- Delaware (delaware): Residential Landlord-Tenant Code, Title 25 Part III. Chapter pages: https://delcode.delaware.gov/title25/c053/index.html (landlord obligations, tenant remedies), c055 (tenant obligations, landlord remedies, deposits 5514), c056 (right to counsel in evictions), c057 (summary possession = eviction court), c059. Chapter 51 is split: https://delcode.delaware.gov/title25/c051/sc01/index.html (rights and procedures, incl. 5105-5107) and .../c051/sc02/index.html (definitions). The plain c051/index.html has no text.
- Hawaii (hawaii): some HRS pages show two versions of a section (current and a future or past one): quote ONLY the version in force today and add a reviewer_note naming the change date. Residential Landlord-Tenant Code HRS ch. 521, one page per section: https://www.capitol.hawaii.gov/hrscurrent/Vol12_Ch0501-0588/HRS0521/HRS_0521-0044.htm (change the 4 digits: 0521-0042, 0521-0053 etc). Summary possession HRS ch. 666: https://www.capitol.hawaii.gov/hrscurrent/Vol13_Ch0601-0676/HRS0666/HRS_0666-0001.htm.
- Idaho (idaho): https://legislature.idaho.gov/statutesrules/idstat/Title6/T6CH3/SECT6-321/ pattern (Title 6 ch. 3 landlord and tenant incl. eviction 6-303 to 6-324; Title 55 ch. 2 e.g. Title55/T55CH2/SECT55-208/). Chapter index: .../Title6/T6CH3/.
- Maine (maine): Title 14, one page per section: https://legislature.maine.gov/statutes/14/title14sec6021.html (6001-6016 eviction/FED in ch. 709, 6021-6030 ch. 710, deposits 6031-6038 ch. 710-A). Chapter list: https://legislature.maine.gov/statutes/14/title14ch710sec0.html.
- Montana (montana): Residential Landlord and Tenant Act Title 70 ch. 24, deposits ch. 25. Per section: https://archive.legmt.gov/bills/mca/title_0700/chapter_0240/part_0030/section_0030/0700-0240-0030-0030.html (= 70-24-303; part_00P0 and section_0SS0 encode 70-24-PSS). Chapter index renders empty: use sections.
- Nebraska (nebraska): Uniform Residential Landlord and Tenant Act 76-1401 to 76-1449: https://nebraskalegislature.gov/laws/statutes.php?statute=76-1416.
- New Hampshire (new-hampshire): whole chapters: https://gc.nh.gov/rsa/html/LV/540/540-mrg.htm (eviction, 66k chars), https://gc.nh.gov/rsa/html/LV/540-A/540-A-mrg.htm (prohibited practices incl. lockouts/utility shutoff, security deposits), https://gc.nh.gov/rsa/html/III/48-A/48-A-mrg.htm (housing standards).
- North Dakota (north-dakota): https://ndlegis.gov/cencode/t47c16.pdf (ch. 47-16 leasing, incl. deposits 47-16-07.1, entry 47-16-07.3; 48k chars) and https://ndlegis.gov/cencode/t47c32.pdf (ch. 47-32 eviction). Ch. 33-06 is repealed, never cite it.
- Rhode Island (rhode-island): Residential Landlord and Tenant Act 34-18: https://webserver.rilegislature.gov/Statutes/TITLE34/34-18/34-18-19.htm (per section; index at .../34-18/INDEX.htm).
- South Dakota (south-dakota): per section, renders by headless browser and is short but quotable: https://sdlegislature.gov/Statutes/43-32-6.1 (ch. 43-32 lease of real property), eviction ch. 21-16 (https://sdlegislature.gov/Statutes/21-16-1).
- Utah (utah): ONLY the versioned urls below have text. The plain "57-17-S3.html" form is an empty shell. Current versions in force on 2026-10-02:
  - 57-17-1 https://le.utah.gov/xcode/Title57/Chapter17/C57-17-S1_1800010118000101.html
  - 57-17-2 https://le.utah.gov/xcode/Title57/Chapter17/C57-17-S2_1800010118000101.html
  - 57-17-3 https://le.utah.gov/xcode/Title57/Chapter17/C57-17-S3_2025050720250507.html
  - 57-17-4 https://le.utah.gov/xcode/Title57/Chapter17/C57-17-S4_1800010118000101.html
  - 57-17-5 https://le.utah.gov/xcode/Title57/Chapter17/C57-17-S5_2023050320240701.html
  - 57-22-1 https://le.utah.gov/xcode/Title57/Chapter22/C57-22-S1_1800010118000101.html
  - 57-22-2 https://le.utah.gov/xcode/Title57/Chapter22/C57-22-S2_2017050920170509.html
  - 57-22-3 https://le.utah.gov/xcode/Title57/Chapter22/C57-22-S3_2025050720250507.html
  - 57-22-4 https://le.utah.gov/xcode/Title57/Chapter22/C57-22-S4_2021050520210505.html
  - 57-22-5 https://le.utah.gov/xcode/Title57/Chapter22/C57-22-S5_1800010118000101.html
  - 57-22-5.1 https://le.utah.gov/xcode/Title57/Chapter22/C57-22-S5.1_2025050720250507.html (crime-victim locks, domestic-violence lease exit, public-safety requests; a new version takes effect 2027-01-01: add a reviewer_note). Note: 57-22-4 is the OWNER duties (incl. 24-hour entry notice at (2)); 57-22-5 is the RENTER duties.
  - 57-22-6 https://le.utah.gov/xcode/Title57/Chapter22/C57-22-S6_2023050320240701.html
  - 57-22-7 https://le.utah.gov/xcode/Title57/Chapter22/C57-22-S7_2023050320230503.html
  - 57-21-2 https://le.utah.gov/xcode/Title57/Chapter21/C57-21-S2_2026050620260506.html
  - 57-21-5 https://le.utah.gov/xcode/Title57/Chapter21/C57-21-S5_2026050620260506.html
  - 78B-6-801 https://le.utah.gov/xcode/Title78B/Chapter6/C78B-6-S801_2026050620260506.html
  - 78B-6-802 https://le.utah.gov/xcode/Title78B/Chapter6/C78B-6-S802_2026050620260506.html
  - 78B-6-802.5 https://le.utah.gov/xcode/Title78B/Chapter6/C78B-6-S802.5_1800010118000101.html
  - 78B-6-803 https://le.utah.gov/xcode/Title78B/Chapter6/C78B-6-S803_1800010118000101.html
  - 78B-6-807 https://le.utah.gov/xcode/Title78B/Chapter6/C78B-6-S807_2018050820180508.html
  - 78B-6-808 https://le.utah.gov/xcode/Title78B/Chapter6/C78B-6-S808_1800010118000101.html
  - 78B-6-809 https://le.utah.gov/xcode/Title78B/Chapter6/C78B-6-S809_2016051020160510.html
  - 78B-6-810 https://le.utah.gov/xcode/Title78B/Chapter6/C78B-6-S810_2025050720250507.html
  - 78B-6-811 https://le.utah.gov/xcode/Title78B/Chapter6/C78B-6-S811_2026050620260506.html
  - 78B-6-812 https://le.utah.gov/xcode/Title78B/Chapter6/C78B-6-S812_2026050620260901.html
  - 78B-6-814 https://le.utah.gov/xcode/Title78B/Chapter6/C78B-6-S814_1800010118000101.html
  - 78B-6-815 https://le.utah.gov/xcode/Title78B/Chapter6/C78B-6-S815_2018050820180508.html
  - 78B-6-816 https://le.utah.gov/xcode/Title78B/Chapter6/C78B-6-S816_2017050920170509.html
  If you need a Utah section not listed, say so in your final message instead of guessing a version.
- Vermont (vermont): whole chapters: https://legislature.vermont.gov/statutes/fullchapter/09/137 (Residential Rental Agreements, 9 V.S.A. ch. 137, 49k chars) and https://legislature.vermont.gov/statutes/fullchapter/12/169 (ejectment = eviction, 12 V.S.A. ch. 169). Per-section urls come back as archive snapshots: use only the fullchapter urls.
- West Virginia (west-virginia): per section: https://code.wvlegislature.gov/37-6-30/ (landlord and tenant ch. 37-6), https://code.wvlegislature.gov/37-6A-2/ (deposits ch. 37-6A), https://code.wvlegislature.gov/55-3A-1/ (summary eviction ch. 55-3A). The page text starts with a long chapter menu; the section is further down.
- Wyoming (wyoming): one PDF for all of Title 1, https://wyoleg.gov/statutes/compress/title01.pdf (about 700k chars). Forcible entry and detainer (eviction) W.S. 1-21-1001 onward starts near offset 357000; Residential Rental Property W.S. 1-21-1201 to 1-21-1211 starts near offset 369000. Fetch with offset 355000 and read on. Quotes must not span a PDF page-footer line; split into two citations at the break.
- Washington, DC (washington-dc): DC is not a state; title it "Washington, DC". D.C. Code per section: https://code.dccouncil.gov/us/dc/council/code/sections/42-3505.01 (evictions, long), 42-3502.17 (deposits pointer), 42-3502.08 (rent increases), 42-3505.51/.52 (eviction notice), Title 42 ch. 35 (Rental Housing Act: rent stabilization covers many but not all units; every rent-stabilization statement says which units it covers, backed by the quote). The Office of the Tenant Advocate https://ota.dc.gov/ is gov guidance. Never state a federal rule as DC law.
