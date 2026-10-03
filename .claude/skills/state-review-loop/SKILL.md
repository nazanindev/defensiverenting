---
name: state-review-loop
description: How to draft state guide pages and run the reader / fixer / judge review loop against prod until every statement is stamped. Use when drafting new states or cities, verifying draft pages, fixing queue notes, or doing a thin-page pass. Triggers - draft a state, run the loop, verify drafts, clear the queue, thin pages.
---

# State review loop

The loop that took waves 3 to 5 (50+ places) from draft to verified. Agent judgement runs in this session and its subagents; commands only list and apply.

## Tools

Build once per session: `go build -o bin/lintprops ./cmd/lintprops && go build -o bin/dumpsrc ./cmd/dumpsrc`.

- `bin/dumpsrc <url> > file.txt`: the source text exactly as the quote checker reads it (live only, never a snapshot). Receipt on stderr. Subagents may run it.
- `bin/lintprops <props.json> [<work-items.json>]`: the voice lint `triage check` runs, without a database. Exit 1 on problems.
- `fileprops.sh <props.json>` (this folder): `triage check`, then `propose -by "triage agent"` only on "0 problems". File every proposal through it.
- `bin/prod triage ...`: list and apply against prod over the fly tunnel (:15432).

## Files in this folder

- `state-brief.md`: the drafter brief (budget, page shape, sources, per-state URL quirks, voice, lessons). Add a state's URL notes before drafting it.
- `common-rules.md`: what every statement must meet. Every agent reads it.
- `reader-rules.md`, `fixer-rules.md`, `judge-rules.md`: one role each.
- `thin-brief.md`: second pass on short pages: fill only real gaps, never pad.

## The loop

1. **Draft**: one Sonnet subagent per page, 2 at a time, prompt = "read state-brief.md and follow it" + page + state URL notes. Drafters save with `save_draft_playbook`.
2. **Read**: `bin/prod triage decide pass 2>/dev/null > in.json` (stdout only), filter to the pages, reader subagent writes `out.json`, then `bin/prod triage decide pass out.json -apply`.
3. **Fix**: `bin/prod triage decide work 2>/dev/null > in.json`, fixer subagent writes props, YOU check every body against its quote, then `fileprops.sh props.json`.
4. **Judge**: `bin/prod triage decide edit` lists pending edits; write `[{id, verdict: apply|leave, reason}]`, apply with `-apply`.
5. **Stands**: a drafter doubt the quote answers: `bin/prod triage decide flag f.json -apply` with a passage copied from the STORED quote.
6. Repeat 2 to 5 until `decide pass`, `decide work` and `decide edit` show nothing for those pages except editor questions.

## Rules learned the hard way

- PASS means the stored quote backs every sentence, not that the law is true somewhere.
- Check every fixer body before filing: fixers drop qualifiers ("legitimate", "other residents", "prospective or actual", "materially") and add pointer words ("listed next", "for that reason", "above", "also").
- A judge apply that drops a statutory condition is overridden to leave.
- Re-read the page (`bin/prod triage page <id>`) before adding followers: agents add statements the page already has.
- Never anchor a proposal on a statement with an open note: a new proposal on the same key can supersede it.
- Withdrawing a held proposal also closes its leave note.
- A review-agent note (page hold, refused pass) cannot be closed by `decide flag` or `withdraw`: clear it with a no-change proposal whose `evidence.resolves` lists the note id, then apply.
- Two strikes, then cut the statement. Lawyer-only doubts go to the editor as notes, not chat.
- A thin page is held with `bin/prod triage decide page`; publish-all skips any page with an open note.
- Live pages stay human: agents work drafts only.
