# ADR-036 — The answer is the unit; a page is a topic shape filled in with answers

| | |
|---|---|
| Status | Accepted 2026-11-12. Trimmed the same day after two independent reviews. The eviction table is filed (see Findings). The shape as data is shipped for two topics. D8 is decided and not built. |
| Date | 2026-11-12 |
| Amends | ADR-035 (the topic spec is a four-part shape; one tip per statement is withdrawn), ADR-034 (a rule value's condition comes from a fixed list), ADR-028 (coverage has three states) |

## Context

The goal: the unit of expansion is a topic spec. Given one, the pipeline produces a high quality page for all 50 states.

The page as the unit of data stands in the way of that. The same question is answered again on every page that needs it. Exact-question search, 50-state tables, an assistant that cites answers, the weekly source check and translation all need one answer per place and question. Only reading a guide needs a page, and a page can be put together from answers.

The schema already points this way. Rule values (ADR-034) and "searched, no law found" records (ADR-028 D5) are keyed by one place and one question.

On the state situation pages on prod (2026-11-12): 4,718 statements, 77% tagged with a question; 1,782 pairs of one state and one question answered; 246 of those answered on two or more topics. A read of the 246 found no contradiction. About 100 are a true shared answer, 65 are content on the wrong page, and 70 are a tag used loosely.

## Decision

### D1. The answer

An answer is one place and one question. On a page it is the small group of statements tagged with that question. It is not one statement: a rule, what the renter can get and an exception are often three.

### D2. A base case and "if" cases

- The **base case** is the ordinary situation most people search for. "How long does my landlord have to return my deposit?" means: I moved out in the normal way.
- An **"if" case** is the same question in a named situation: if you left early, if you were evicted.
- Situations are a short fixed list, set per question on the topic. They are never named per place.
- "If" cases are the condition on a rule value (ADR-034). No second structure. Each case has its own quote.
- An "if" case never holds another "if" case.
- Some answers have no honest base, such as a rule tiered by lease length. Those are rows: "depends on X".
- A page shows the base case, and the "if" cases that match its own situation. The breaking-a-lease page shows the deposit deadline for a renter who left early.

### D3. Three states for a place and a question

Answered. Searched, no law found. Not checked. "Not checked" is stored or computed, never left to look like "no law".

### D4. The shape

A topic's shape is the same in every state. The state shows up in the answers. It has four parts. Each part has its own audit check and its own fix pass, so a change to one part reruns only that pass, only on the pages that now fail it.

| Part | Holds | A change costs |
|---|---|---|
| Title and scope | Reader, title, how the intro opens, what is on the page, what goes to another topic | Almost nothing |
| Tips and warning notes | Which the topic uses, and which sit on every page | Cheap: an agent reads the page only |
| Size and order | Caps, section order, what leads, which extras may stay, an example statement | Medium: cuts and merges |
| Must-answer questions | The questions, each under a section, how it is answered (D7), and which "if" case is the topic's own | Expensive: research |

Tips:

- A tip's rule lives on the tip and describes the statement it sits under ("tells the renter to pay the landlord"). It never names a section of one topic.
- A statement can carry more than one tip.
- A tip never sits under a statement that already says it. It may sit under an earlier one.

About a quarter of statements answer no question: steps, local how-to. They belong to the page.

Eviction's four parts and breaking a lease's four parts were approved on 2026-11-12. Breaking a lease was loaded second to test the storage. It needed two additions: a line for how the intro opens, and the "list" answer of D7.

### D5. Pages keep their statements

No statement is shared between pages, and no page stores a link to another page's statement. Two pages that answer the same question are held together by a check:

- The numbers on both must match the rule value.
- Different wording is flagged for a read.
- A live page and its draft revision are one page, not an overlap.

A page save rewrites its statement rows. Only the statement key survives a save. Anything that ever links statements points at a key.

### D6. Numbers say their start

A stored number says from what event it runs and whether the days are calendar or business days.

A quote behind a stored number sits on one page of a PDF source. A quote that crosses a page break fails the source check when the page header moves.

### D7. A number everywhere, or only where the law sets one

Some questions are not answered in days in every place. "Can I pay and stop an eviction?" is answered by an event in most places: before the judge rules, before the officer carries out the order. "Can I move out because my home is not safe?" has a number of days to fix the problem in some states and "a reasonable time" in others. A must-answer question is marked as one of:

- **days**: every place has a stored number. A place with words and no number is a gap.
- **days where set**: every place has words. A stored number exists only where the law sets one. A place with words and no number is answered.
- **list**: one statement for each protection the place has.
- No mark: words only.

### D8. A wording is a child of its answer (2026-11-13)

A shape that only instructs and checks is not enough. The answer has to be a stored thing that pages draw from.

- An answer is stored once for a place and a question: its quotes, its number if it has one, and one plain wording of what the law says.
- Each page that answers the question keeps its own statement, in its own words. That statement is a child of the answer. Pages still share no rows (D5), and a child never points at another page.
- A child is checked against its answer. The answer is checked against the law. The law is read once.
- When an answer changes, every child is flagged that day.
- Two children of one answer cannot disagree without it showing.
- A new page starts from the answers its shape names. It does not research them again.

This reverses one line of the trim. The reviews objected to one row shown on two pages. A child is its own row that knows its parent, so that objection does not apply.

Not decided: where the first wording of an answer comes from for the pairs that exist today, and what a child stores to show which version of its answer it was read against.

## Findings from the eviction table

Filed 2026-11-12: 173 numbers for six questions across 61 places, each an exact copy of a quote already stored on a page.

| Question | Situations | Places with a number |
|---|---|---|
| Notice when rent is late | Base only | 43 |
| Notice before court | A lease violation you can fix (base); one you cannot fix; serious harm or an illegal act | 29 |
| Pay to stop the case | After the court papers; after the judgment | 5, and a cutoff named in words in 24 more |
| Time to answer court papers | To file a written answer; until the court date | 23 |
| Time to move out | After the judgment; after the officer's notice | 29 |
| Appeal deadline | Base only | 32 |

- A short fixed list of situations per question holds. No place needed a situation of its own.
- Two questions have no single base. Time to answer and time to move out each split into two steps that differ by state. Both are stored as situations.
- About half the cells have no number. Every one of those pages passed review, because each statement is backed by its quote. The gap shows only when 50 states sit in one column. The clearest case: 26 places say an appeal is possible and give no deadline.
- A cell with no number is not always a fault. In many states the date to answer is printed on the court papers.
- The day kind (calendar, business, court days) is stated in only a few quotes.
- One page read against its rows (California): all three stored numbers match the page. 8 of 11 statements answer a must-answer question, 2 are extras the shape allows, 1 belongs to the page. The table found three holes on a reviewed page: no base case for notice before court, no appeal deadline, no pay cutoff after the case is filed.
- Putting a page together from answers changes nothing a reader sees today, because the statements of an answer exist only on the page itself. The table is the output that matters.

## Order of work

Nothing moves until Nazanin says go.

1. Done. A 50-state table of eviction's must-answer questions.
2. Done as a read. One page read against its rows.
3. The shape as data: the four parts stored per topic, and the table as a command that reads the shape and the stored numbers. Shipped for eviction and breaking a lease.
4. Answers as stored rows with children (D8), proven first on the questions two topics share.
5. Fill the gaps the table shows, one question at a time, on draft pages. The appeal deadline is first.
6. Shapes for the other topics, one at a time.
7. Content on the wrong page and loose tags are fixed, so the check of a child against its answer is not noise.

## Deferred

- **City pages reading state answers.** City guides stay whole pages in their own words. When this is taken up: a state can forbid a city rule, so "adds" and "replaces" are not the only cases, and federal law is often an "if" case for some housing, not a parent every state inherits.
- **Shared statements, and publishing an answer instead of a page.**

## Open questions

- The fixed list of situations for each question on topics other than eviction.
- How a cell with no number is told apart: a real gap, or a place where the date is set case by case.
- When a live page and its draft word an answer differently, which one a table reads.
