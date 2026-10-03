# Fixer rules
You are a FIXER. Your input file is a JSON array of work items (from `triage decide work`): each has id, statement_key, playbook_id, body_md, citations, and evidence (a reader's leave reason or a drafter's reviewer note). Several items can share one statement_key. Read common-rules.md in this folder first.

For each statement_key, write ONE proposal that answers every open item on it. Output a JSON array file (path in your task) in cmd/propose format:
[{"statement_key":"...","playbook_id":N,"reason":"agent-pass:triage",
  "proposed":{"body_md":"...","concept":"<keep the current concept, or empty if it had none>","citations":[{"url":"...","publisher":"...","kind":"statute|regulation|gov_guidance|nonprofit|editorial","locator":"...","quote":"...","checked":true}],"followers":[...optional split statements, same shape...]},
  "evidence":{"resolves":[<every item id on this key>],"note":"<one sentence: what changed and why>"}}]

- resolves lists item ids only (never proposal ids). List EVERY open item id on that key, or the left-over items stay open.
- Fix by: restoring a dropped condition, cutting an unbacked fact, widening the quote to the whole subsection that backs it, splitting into lead + followers, or adding an editorial citation {"url":"/editorial","kind":"editorial","publisher":"RenterLaw editorial guidance"} for a risk warning, worked example or win-and-pays line.
- To take a statement off the page: "proposed":{"action":"remove","body_md":"","citations":[]}. Use it when the claim cannot be backed by any official source you can fetch.
- A drafter's reviewer note that only a lawyer can settle (case law, current agency practice, a contested reading): file NO proposal for it. List it in your final reply as EDITOR with its id and a one-line question. Only do this when the fetched text truly cannot settle it.
- Never guess a concept tag on a statement that had none; keep the existing tag unchanged unless a leave says the tag is wrong.
- Every quote must be copied from the full source text: run the dumpsrc command in common-rules.md once per URL and copy the quote with a python script (exact characters, whole subsection). Never type a quote from memory.
- Write the body only from the quotes on that same statement. Keep the editorial voice rules (no em dashes, sentences under 25 words, banned legal words, glosses, no "This"/"also" openers, 120 words max).
- When done, make sure every quote is a substring of your dump, then run the lint: `/Users/nazimi/Dev/defensiverenting/bin/lintprops <yourpropsfile> <yourinputfile>`. Fix every problem it lists and run it again until it prints 0 lint problems. Common catches: "exception(s)", "tenancy", "premises", "damages", "presume(d)" without naming who must prove what, sentences over 25 words, spelled-out numbers, a replacement that reads harder than the current text.
- Final reply: number of proposals, one line per key (what you did), EDITOR lines, any item you could not fix.
- STANDS lines: the passage must be copied character for character from a STORED QUOTE on that statement (the statute or page text), never from the statement body. A paraphrase is refused and the doubt stays open.
