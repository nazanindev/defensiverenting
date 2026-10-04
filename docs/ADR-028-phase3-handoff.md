# ADR-028 phase 3 handoff (written 2026-10-04)

Start a new session with: "Read docs/ADR-028-phase3-handoff.md and start phase 3."

## Where things stand

- **Phase 1 (build) is live.** Migration 000048: 15 standard topics, rules topics (`*-rules`, one per situation topic), stages, concept questions, coverage records. Read the Build notes at the end of `docs/ADRs/ADR-028-standard-set-and-rules-pages.md`.
- **Phase 0 (gaps) is done.** The tag check filed 729 tag changes, and Nazanin bulk applied them 2026-10-04. `triage gaps <state>` now gives accurate gap lists.
- **Phase 2 (national pages) is done.** Move-in and move-out checklists are ready to publish. Rental application (playbook 572) and assistance animal (573) are held by page flags 7 and 8, an editor question: HUD's 2016 criminal-records guidance and 2020 assistance animal notice now redirect to the HUD homepage, so they may be withdrawn.

## Phase 3: locked-out and breaking-lease for every state

Up to 102 pages: 51 places (50 states and DC) times 2 topics. The thin-page rule decides: a state with too little law gets no page.

1. **Read first:** `.claude/skills/state-review-loop/SKILL.md`, `state-brief.md`, and `docs/topic-map.md` (the playbook rows for `locked-out` and `breaking-lease`, and the "Eviction, lockouts and renewal" and "Can't pay and leaving early" concept sections).
2. **Stages** (each statement gets one, copied exactly):
   - locked-out: Only a court can make you leave · Get back into your home · Get your things back · What a court can make your landlord do
   - breaking-lease: When you can leave early · What you may still owe · How to leave · After you move out
3. **Titles** are in `state-brief.md`: "Landlord Locked Me Out in {State}: What Can I Do?" and "Breaking a Lease Early in {State}: What Are My Options?".
4. **Draft** with 2 Sonnet subagents at a time, never more (Nazanin's rule). Each gets "read state-brief.md and follow it" plus its state and topic, and the state's URL notes from `state-brief.md`. Do the states with free official code sites first. GA, MS, IN, TN and AR follow `no-free-code-states.md`.
5. **Run the loop** per the skill: read, fix, judge, stands. Then repeat until `decide pass`, `decide work` and `decide edit` are empty for the wave except editor questions.
6. **Waves:** about 20 pages per wave, then converge before the next, as earlier waves did.

## What to watch

- **Live statements to move.** Lockout statements already on live `landlord-entry` pages, and unlivable-home statements on live `repairs-and-habitability` pages, stay where they are (D7). After the new page exists, file the removal from the live page as a queue proposal for Nazanin. Never edit a live page.
- **Constructive eviction.** The live US and PA `constructive-eviction` pages stay until `breaking-lease` publishes in those places. Then they are retired and redirected (Build notes, not built yet).
- **Place named.** Every new statement must name its state; the save refuses one that does not.
- **Tag concepts:** `court-eviction-only`, `illegal-lockout` (locked-out); `early-termination-rights`, `duty-to-mitigate`, `constructive-eviction` (breaking-lease). Statements citing only site guidance never count as a concept answer.

## Tooling notes for this machine

- **Prod commands:** `bin/prod triage|propose ...` uses `bin/triage` and `bin/propose` in the main checkout. Rebuild them from main first (`go build -o bin/triage ./cmd/triage && go build -o bin/propose ./cmd/propose`), because older builds lack `gaps`, `nolaw` and `retag`.
- **The fly tunnel on :15432 must be up.**
- **Rebuild `bin/mcp`** from main and reconnect with `/mcp` if the drafting tool was built before ADR-028. The drafting tool needs that build for stages and the place rule.
- **Migration numbers:** this branch used 000048. The ADR-029 session took 060 to 063. Check `db/migrations` before adding one.
- **Concurrent sessions** commit to main: work in a worktree, pull before pushing.

## Deferred, not phase 3

- The voice rules restructure and the CI copy check (see `project-site-copy-voice` memory).
- Listing coverage records in the weekly check.
- Stages on existing drafts and live pages.
