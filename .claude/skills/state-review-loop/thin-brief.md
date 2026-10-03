# Thin page second pass (2026-10-02)

You review draft pages that were held as thin. Nazanin's rule: "dont beef them up just cause. if they satisfy the playbook then we can publish them."

So for each page, answer one question: **does this page answer its situation question as far as this state's law and official sources allow?** More words are not the goal. A short page is fine when the state has little law on the topic.

Read first:
- /Users/nazimi/Dev/defensiverenting/docs/topic-map.md (the page's situation question, its "Covers" column, and its concepts)
- /Users/nazimi/Dev/defensiverenting/.claude/skills/editorial-voice/SKILL.md
- /Users/nazimi/Dev/defensiverenting/.claude/skills/state-review-loop/common-rules.md
- /Users/nazimi/Dev/defensiverenting/.claude/skills/state-review-loop/state-brief.md (sources, per-state url notes, voice, lessons)
- /Users/nazimi/Dev/defensiverenting/.claude/skills/state-review-loop/fixer-rules.md (the proposal format and the lint loop)

Steps for each page:
1. Read the page with `cd /Users/nazimi/Dev/defensiverenting && bin/prod triage page <playbook id>` (read-only; get_playbook cannot see drafts).
2. For each item in the topic map "Covers" column and each concept of that topic, decide: covered on the page, or not covered. For each gap, search the state's official sources (statute site, regulations, AG or housing agency, court self-help, state legal aid) for a rule that answers it. Use fetch_source; you may run `/Users/nazimi/Dev/defensiverenting/bin/dumpsrc <url> > <scratchpad>/src-<prefix>-<name>.txt` for the live text (you have permission). Never cite snapshots, Nolo, Justia, FindLaw, law firms.
3. A gap is real only if an official source states a rule or program that answers it. Silence is not a rule: never write "No [state] law sets X" unless an official source says so.
4. For each real gap, write a new statement as a FOLLOWER on the existing statement it belongs after. Proposal shape (fixer-rules format): statement_key and playbook_id of that existing statement, reason "agent-pass:triage", proposed.body_md and citations UNCHANGED from the current statement (copy them exactly, add "checked": true), proposed.concept unchanged, proposed.followers = [your new statements], evidence.resolves = [], evidence.note = what gap the followers fill. One proposal per anchor statement.
5. Also fix any existing statement that breaks common-rules (pointer words, dropped conditions), same format.
6. Run lintprops on your props file until 0 problems. Do not file anything.

Budget: at most 40 tool calls per page.

Final reply, per page:
- VERDICT: "satisfies the playbook" or "gaps filled" or "cannot satisfy" (say why: what the renter needs that no official source gives).
- Coverage list: each Covers item / concept, covered or not, and the source or the reason.
- Proposals written (count, one line each).
