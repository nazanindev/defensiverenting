# ADR-015 — Spanish content is deferred; the plumbing stays

| | |
|---|---|
| Status | Accepted |
| Date | 2026-09-08 |
| Amends | ADR-007 (routing stays), ADR-008 (tooling stays), ADR-014 D6 (the gap this closes for now) |

## Context

Spanish went end to end in August: URL routing (ADR-007), a lint ruleset and agent translation path (ADR-008), and a pilot pass that saved fourteen Spanish drafts for Austin and New York. None was published; a native-speaker review of the ruleset and of the drafts never happened, because there was nobody to do it and no budget to hire them.

The review queue (ADR-014) made the cost of half-finished Spanish visible on its first run. A Spanish draft is a translation of an English page and cites the same sources with the same quotes. When a source drifts, the checker files one proposal per statement citing the vanished quote, so the same drift arrives twice: once against the English statement, which an editor will act on, and once against the Spanish statement, which nobody can. ADR-014 D6 named this: a change approved on an English statement does not carry to its Spanish counterpart, because nothing links their keys. Until that link exists and someone can read the result, every Spanish page is a liability that grows with each English edit.

Meanwhile the fourteen drafts sit in the dashboard's draft count, carry publish-gate issues nobody will fix, and cost a fetch of every source they cite on every check run.

## Decision

### D1. Content languages are a registry, and it holds English

`store.ContentLanguages` lists the languages in which content is authored, drafted, checked, and queued. It is `["en"]`. The voice rulesets keep Spanish (`voice.Supported` is unchanged) so parked Spanish text still lints if anyone opens it; the registry is about what the site works on, not what it can parse.

### D2. Nothing new starts in a deferred language, and nothing deferred publishes

`ResolveLanguage`, which every authoring entry point already calls, refuses a language outside the registry with a message naming this ADR. That closes the portal's new-page form, the agent's `save_draft_playbook` and `get_playbook`, and `cmd/draft -language es`, in one place. Editing an existing Spanish draft still saves, because refusing to save what already exists loses work for no gain. Publishing one is refused: the issue checker (ADR-013 D2) reports `language-deferred` on any page whose language is outside the registry, so the dashboard badge, the view page, and the publish gate all say the same thing.

### D3. Deferred content leaves the working set

- The dashboard hides pages in a deferred language unless the language filter names that language explicitly. The filter keeps its Spanish option, so the drafts are findable, and the draft count describes what an editor can act on.
- The source checker examines only citations reachable from a page in a registry language. A Spanish draft cites what its English page cites, so nothing checkable is lost, and each source is fetched once.
- The review queue lists, and counts, only proposals whose target page is in a registry language. Proposals already filed against Spanish statements stay in the table, invisible until the language returns.

### D4. What stays

Public routing under `/es/` stays as built. No Spanish page is published, so nothing served changes; if one were, it would keep serving. The translation tooling, prompt text, and lint stay. Nothing is deleted: the drafts, their proposals, and the code path are all one registry entry from active.

### D5. What resuming requires

Adding `"es"` back to the registry is the switch. Before it is flipped, two things must exist: a link between an English statement key and its Spanish counterpart, so an approved English change files a translation proposal instead of leaving the Spanish page stale (ADR-014 D6), and a person who reads Spanish to review both the ruleset and the drafts. The second is the one that costs money, and it is the reason for this ADR.

## Consequences

- The dashboard, the checker, and the queue describe English work only. Fourteen drafts and their proposals leave the counts.
- A Spanish drift proposal never appears, so no editor is asked to decide something they cannot.
- An agent that tries to translate is told the language is deferred and why, rather than producing a draft nobody will read.
- The Spanish drafts age. When work resumes, they are a starting point, not a finished pass: each English page may have changed under them.
