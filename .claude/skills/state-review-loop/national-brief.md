# National page drafting brief (ADR-028 phase 2, 2026-10-04)

You draft ONE nationwide page on jurisdiction_slug "united-states" and save it as a DRAFT with the `mcp__defensiverenting__*` tools. A person publishes it later.

Read first, with the Read tool:
- /Users/nazimi/Dev/defensiverenting/.claude/worktrees/adr028-voice/docs/topic-map.md (the national-only table says what your page covers; "Concepts by area" lists the concept tags)
- /Users/nazimi/Dev/defensiverenting/.claude/worktrees/adr028-voice/.claude/skills/editorial-voice/SKILL.md
- /Users/nazimi/Dev/defensiverenting/.claude/worktrees/adr028-voice/.claude/skills/state-review-loop/common-rules.md
- /Users/nazimi/Dev/defensiverenting/.claude/worktrees/adr028-voice/.claude/skills/state-review-loop/state-brief.md, sections "Sources" and "Voice" only (they apply here too)

## Budget

At most 30 tool calls. Fetch each source once. At 25 calls, save with what you have.

## Nationwide rules

- Most renter law is state law. State a rule as nationwide only when federal law (a U.S. Code section, a federal regulation, HUD or another federal agency) says it. Otherwise say plainly that it differs by state: "Most states limit how much a landlord can charge for a deposit. The limit differs by state."
- When a fact varies by state, tag the statement with the concept whose question it answers (for example `deposit-cap`). The page then links the reader to the rule in every state. Never list states one by one.
- Never cite one state's law for a nationwide claim.
- Practical advice with no law behind it (take dated photos, keep copies, ask for a receipt) cites a citation with kind "editorial", no url, no quote. Only advice that legal aid groups give everywhere.
- Nationwide statements do not need a place name (the place rule applies to state and city pages).

## Checklists (move-in-checklist, move-out-checklist, rental-application)

- page_kind "checklist". 10 to 14 items. Each item is one thing the renter checks or does, written as an action: "Take dated photos of every room before you move in."
- Order the items the way the renter meets them (before signing, at move-in, during the first week; or before notice, at move-out, after).
- One item per statement. Each item that depends on a state rule says it varies and carries the concept tag; the numbers live on the concept pages.
- No stages on a checklist.

## assistance-animal (a situation page)

- page_kind "playbook". 10 to 14 statements in the stage order: "What the law says", "Ask your landlord", "If your landlord says no". Give each statement its stage (field `stage`), copied exactly.
- Federal law carries this page: the Fair Housing Act (42 U.S.C. 3604(f)), HUD's rules at 24 CFR 100.204, and HUD's 2020 assistance animal guidance (FHEO-2020-01). Fetch HUD's pages.
- Tag statements with `assistance-animal-rules`, `reasonable-accommodation`, `fair-housing-complaint` where they answer those questions. A state's extra rules (for example a state law on support-animal letters) are not on this page: say "some states add their own rules" only if a federal source says so; otherwise say nothing.

## Titles (the renter's search question)

- move-in-checklist: "Moving Into a Rental: What Should I Check?"
- move-out-checklist: "Moving Out of a Rental: How Do I Get My Deposit Back?"
- rental-application: "Applying for a Rental: What Should I Know?"
- assistance-animal: "Service or Support Animal in a Rental: What Are My Rights?"

Intro: 2 short sentences naming what the page covers. No legal claim in the intro.

## Save

`save_draft_playbook` with jurisdiction_slug "united-states" and the topic_slug given. If rejected, fix only what it lists and save again. Do not spawn subagents.

## Final message (under 120 words)

Saved playbook id, statement count, sources you could not fetch, anything a lawyer should look at, tool calls used.
