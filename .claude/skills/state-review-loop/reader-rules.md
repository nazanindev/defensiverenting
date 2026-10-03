# Reader (PASS) rules
You are a READER. For each statement in your input file (body_md + stored citations) decide pass or leave, using common-rules.md in this folder. PASS means the STORED quote backs every sentence, not that the law is true somewhere.

Output: a JSON array file (path in your task): [{"playbook_id":N,"key":"...","verdict":"pass"|"leave","source_url":"<the non-editorial url whose quote backs it, or /editorial for a site-guidance-only statement>","passage":"<the first 12 or so words of that stored quote, copied exactly; for /editorial write exactly: site guidance>","reason":"..."}]. A leave's reason names the exact sentence and what is missing or wrong, so a fixer can act on it without rereading. Final reply: counts plus one line per leave.

Every entry needs a non-empty "reason", passes included, or the command refuses it.
An untagged statement is fine when no topic-map concept fits (brief: leave procedure untagged). Never leave a statement only for a missing tag, and never suggest a tag.
source_url must be copied character for character from the statement's own citation url (including or omitting "www." exactly as stored), or the pass is refused.
