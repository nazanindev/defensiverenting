# Rules page drafting brief (ADR-028, 2026-10-04)

A rules page answers one state's rules for one topic: one statement per concept, each shown under the concept's question with the state added ("How much can my landlord charge for a deposit in Ohio?"). It is a playbook in every other way: draft, read, fix, judge, stamps, publish gate. Read state-brief.md first. Every rule there applies, except where this file says otherwise.

## What you draft: the gaps, and nothing else

You get a gap list from `triage gaps <state> <rules-topic>`. Each entry is a concept with its question and a state: `answered` (another page already says it: leave it alone), `no-law` (we looked and found no law: leave it alone), or `gap` (draft it).

- Draft one statement per `gap` concept, tagged with that concept (field `concept`). The save refuses any other concept, a second statement on the same concept, and any concept another page in the state already answers.
- The statement answers the question directly, in the first sentence, without the state name (the heading carries it): "The law does not limit how much a landlord can charge for a deposit." Then the conditions and numbers its quote supports.
- No stages on a rules page. The question is the heading.
- `page_kind` is `rules` (the store sets it anyway). Title: "{Topic name} in {State}: What Does the Law Say?". Intro: 2 short sentences naming what the page answers, no legal claim.
- No steps, no "what to do". The situation page carries the steps; the rules page links to it.

## When there is no law

- If an official source says in words that the state has no rule (for example a state guide: "Ohio has no limit on deposits"), that is a normal statement citing it.
- If you search the official places and find nothing, do not write a statement. List it in your final message under "No law found", with the concept and every official URL you searched. The reviewer files a coverage record with `triage nolaw`, and the page shows "We did not find an Ohio law on this. Last checked {date}."
- Never infer a rule from silence. "No Ohio law requires a receipt" needs a source that says so.

## Budget and shape

- At most 30 tool calls. Fetch each source once.
- A page with 2 answers is fine. Never pad.

## Your final message (under 150 words)

- saved playbook id and statement count
- No law found: concept, URLs searched (one line each)
- anything a lawyer should look at
- tool calls used
