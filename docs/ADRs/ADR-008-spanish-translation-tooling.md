# ADR-008 — Spanish translation tooling: no dedicated MT service, for now

| | |
|---|---|
| Status | Accepted |
| Date | 2026-08-16 |

## Context

With the drafting toolbelt, authoring form, and routing (ADR-007) all language-aware, the remaining question was tooling: should Spanish translation run through a dedicated machine-translation service — several now exist as MCP servers — instead of, or alongside, Claude translating directly in the drafting agent?

Checked what's actually available (not from memory — searched, since MCP server availability changes):

- **DeepL's official MCP server** ([github.com/DeepL/deepl-mcp-server](https://github.com/DeepL/deepl-mcp-server)) — translate, rephrase, language detection, formality control, and glossary/style-rule lookups.
- **Lara Translate MCP server** — purpose-built for LLM/agent workflows, one of 36 servers in Scale AI's MCP Atlas benchmark.
- **Google Cloud Translate** and **Microsoft/Azure Translator** — also have MCP wrappers, more raw-API-shaped than the two above.

## Decision

### No dedicated MT service is integrated now

The bottleneck in this pipeline was never translation fluency — Claude is already strong at Spanish. The actual constraints are project-specific and no MT API knows about them: citation quotes must stay byte-exact and untranslated, and the Spanish prose must pass the same `voice.Lint` plain-language rules the English text does (short sentences, no jargon, digits not spelled numbers, a dollar example per percentage). Feeding a statement to DeepL or Lara produces grammatically correct Spanish that may well fail that lint — Claude then has to rewrite it to pass the same gate it already passes when translating directly. The MT call becomes an extra hop, not a replacement for the step that actually matters.

### Two levers are worth adding later, once there's a signal that justifies them

1. **Independent cross-check, not primary translation.** Run a dedicated MT service in parallel with Claude's translation and flag divergence for human review — the same adversarial-verification instinct behind the verbatim-citation guardrail. Catches meaning drift on legal content, the actual risk category this project already optimizes for.
2. **Glossary/terminology consistency at scale.** DeepL's MCP server supports glossary lookups. Once dozens of topics are translated across cities, Claude alone may pick different Spanish synonyms for the same term ("security deposit") run to run; a glossary tool enforces one fixed term site-wide.

Neither is needed today: Spanish city/topic coverage is still zero, so there is no real terminology to drift and nothing yet to cross-check against.

## Consequences

- Translation stays entirely inside the existing agent loop (`get_playbook` → translate → re-`fetch_source` → `save_draft_playbook`), with no new external dependency, API key, or MCP server to operate.
- Revisit trigger: once a meaningful number of Spanish pages exist, check for (a) inconsistent terminology across pages for the same legal concept, or (b) a desire for a second opinion on translation accuracy independent of the drafting agent. Either is the point to add DeepL (glossary) or a cross-check pass (Lara or DeepL as a second translator), not before.
