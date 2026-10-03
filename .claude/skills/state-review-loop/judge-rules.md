# Judge rules
You are a JUDGE. Your input file is a JSON array of pending edits (from `triage decide edit`): each has id, current_body, current_citations, note, resolves, and proposed (body_md, citations, optional followers or action). Read common-rules.md in this folder first.

For each item decide "apply" or "leave":
- apply only if (a) the proposed statement (and each follower) is backed sentence by sentence by ITS OWN stored quotes, (b) it fixes what the note says, (c) it passes the voice rules in common-rules.md, (d) it does not lose a true, backed fact the renter needs that was not the problem.
- For action "remove": apply if the current statement is not backed by its quote and the note's reason holds.
- You may read sources with the dumpsrc command in common-rules.md. Never invent "accepted wording"; judge only against the source text.

Output a JSON array file (path in your task): [{"id":N,"verdict":"apply"|"leave","reason":"..."}]. Every entry needs a non-empty reason. A leave's reason names the exact sentence and what is wrong so the next fixer can act.

Also write a second file (path in your task) of PASS decisions for every item you mark apply, so the new statement is stamped once applied: for the replacement statement use the same statement_key; [{"playbook_id":N,"key":"<statement_key>","verdict":"pass","source_url":"<a non-editorial url from the proposed citations, exactly as written, or /editorial if only editorial>","passage":"<first 12 or so words of that proposed quote, copied exactly; for /editorial write: site guidance>","reason":"..."}]. Followers get new keys you cannot know: do not write pass entries for followers.
Final reply: counts plus one line per leave.
