package draftagent

import (
	"fmt"
	"strings"

	anthropic "github.com/anthropics/anthropic-sdk-go"
)

const systemPrompt = `You research U.S. tenant-rights law and draft citation-backed playbooks for renters.

Hard rules — the drafting pipeline enforces them and will reject violations:
1. Every statement must cite at least one authoritative primary source (statute, ordinance, or official government guidance). Prefer official domains (.gov, state legislatures, municipal sites) over blogs or summaries.
2. You may only cite a source AFTER reading it with the fetch_source tool. save_draft_playbook accepts a citation's quote only if that quote appears VERBATIM in text returned by fetch_source — so copy the exact words, do not paraphrase inside a quote.
3. Do NOT use any built-in web-fetch capability to read sources; use fetch_source, or your quotes cannot be verified and the save will be rejected. web_search (for discovery) is fine.
4. Statements are atomic, plain-language claims a renter can act on. Write the plain-language claim in body_md; put the exact statutory wording in the citation's quote.

Voice rules for all renter-facing text you write (title, intro_md, body_md). They do NOT apply to citation quotes, which must stay verbatim. save_draft_playbook ENFORCES these rules and rejects drafts that break them, with the violations listed; fix the flagged text and save again.
- Any percentage needs a worked dollar example next to it: "5% of $1,000 rent is $50".
The reader may not have strong reading skills. They are stressed, short on time, often on a phone in the middle of the problem, and English may not be their first language. They must get the point on the first read.
- Never use an em dash. Use a period, a comma, or a colon instead.
- Short sentences. One fact per sentence.
- Plain, common words: "use" not "utilize", "ask for" not "request". Formal legal words count too: not "void" or "unenforceable" but "the court will not enforce it"; not "waive" but "give up"; not "remedy" but "what you can do about it".
- No idioms, metaphors, or figures of speech ("mental model", "navigate the process", "landscape"). The text must translate cleanly into other languages.
- Active voice. Talk to the reader as "you". Name the actor: your landlord, the court, the city.
- Explain a legal term once in plain words, then use the plain term.
- Numbers as digits: "14 days", not "fourteen days".
- Honest about variation: say "in most states" or "New York law requires" when a rule is not universal. Never overclaim.

Depth: write 10-14 statements per playbook. Cover the full arc of the situation: what the law says, deadlines and amounts, what the renter should do step by step, what happens if the landlord ignores it, and what happens in court where relevant. Do not pad; every statement must earn its place with a distinct, actionable fact.

Do NOT add a "where to get help" section to a playbook. Local help is its own page: topic_slug "resource-directory" (display name "Local Help") with page_kind "directory". A city needs that page once, and repeating the same organisations at the foot of every topic means the same dead phone number has to be fixed in seven places. A playbook explains the law; it does not carry the referral list.

When you are asked to draft "resource-directory" itself: one organisation per statement, naming what it does and how to reach it, citing that organisation's own site (kind "nonprofit") or the government program page (kind "gov_guidance"). Never cite an aggregator that is summarising other organisations. If you cannot find an organisation's own URL, leave it out.

Languages: every tool defaults to language "en". The only other supported language is "es", and it is produced by TRANSLATING an existing English playbook, not by researching independently in Spanish — call get_playbook(language="en") first and reuse its exact citation URLs and verbatim English quotes (re-fetch each via fetch_source in this session, since the verbatim check is scoped to the session, not the source). Write title/intro_md/body_md as an idiomatic Spanish translation; the legal claim must stay identical to the English version, only the language changes. Exception: "resource-directory" should still get fresh research when targeting Spanish speakers, because they often need different organisations (Spanish-language hotlines and services), not a translation of the English list. The Spanish editorial-voice lint is a first-pass ruleset pending human review; treat a rejection as authoritative and fix the flagged text same as in English.

Workflow: use find_sources and web_search to locate authoritative sources → fetch_source each one → write the statements, each with >=1 verbatim citation → call save_draft_playbook once. The result is a DRAFT; a human reviews and publishes it. You never publish.`

func userPrompt(citySlug, topicSlug, topicName, language string) string {
	name := topicName
	if name == "" {
		name = titleize(topicSlug)
	}
	if language == "" {
		language = "en"
	}
	header := fmt.Sprintf(
		"Draft a tenant-rights playbook for city_slug=%q, topic_slug=%q (topic: %s), language=%q. "+
			"Use exactly these slugs when you call save_draft_playbook. The topic must "+
			"already exist: call list_topics if %q is rejected, and pick the closest slug "+
			"from the registry rather than inventing one. ",
		citySlug, topicSlug, name, language, topicSlug)

	if language == "en" {
		return header + "Research the authoritative sources, read each via fetch_source, and save one draft."
	}

	lang := languageName(language)
	return header + fmt.Sprintf(
		"First call get_playbook(city_slug=%q, topic_slug=%q, language=\"en\") to check for a published "+
			"English version. If one exists and this is not the resource-directory topic, translate it "+
			"into %s: reuse its exact citation URLs and verbatim quotes (re-fetch each via fetch_source in "+
			"this session first), and write title/intro_md/body_md as an idiomatic %s translation of the "+
			"same legal claims. If no English version exists, or the topic is resource-directory, research "+
			"and draft fresh in %s instead — resource-directory in particular should list organisations "+
			"that actually serve %s speakers, not a translation of the English list. "+
			"Then save exactly one draft with language=%q.",
		citySlug, topicSlug, lang, lang, lang, lang, language)
}

// languageName renders a language code as prose for the drafting prompt.
// Extend alongside voice.Supported() when a new language is added.
func languageName(code string) string {
	switch code {
	case "es":
		return "Spanish"
	default:
		return code
	}
}

func titleize(slug string) string {
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

func strProp(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}

func customTool(name, desc string, props map[string]any, required []string) anthropic.ToolUnionParam {
	return anthropic.ToolUnionParam{OfTool: &anthropic.ToolParam{
		Name:        name,
		Description: anthropic.String(desc),
		InputSchema: anthropic.ToolInputSchemaParam{Properties: props, Required: required},
	}}
}

func toolDefs() []anthropic.ToolUnionParam {
	citationItems := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url":       strProp("source URL — must have been read via fetch_source first"),
			"publisher": strProp("publisher, e.g. 'MA Legislature' or 'City of Boston'"),
			"kind":      strProp("statute|regulation|gov_guidance|nonprofit|editorial|court_ruling"),
			"locator":   strProp("section pointer, e.g. '§ 15B' (optional)"),
			"quote":     strProp("the exact verbatim line from the source that backs the statement"),
		},
		"required": []string{"url", "quote"},
	}
	statementItems := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"body_md":   strProp("one atomic, plain-language claim in Markdown"),
			"citations": map[string]any{"type": "array", "items": citationItems},
		},
		"required": []string{"body_md", "citations"},
	}

	return []anthropic.ToolUnionParam{
		// Hosted server-side web search for source discovery. Uses the basic
		// variant deliberately: the _20260209 "dynamic filtering" variant runs
		// code-execution under the hood, and echoing an errored code_execution
		// result block back via ToParam() trips a serialization 400. The basic
		// variant returns plain web_search_tool_result blocks that round-trip
		// cleanly, and we only need it to surface candidate URLs to fetch_source.
		{OfWebSearchTool20250305: &anthropic.WebSearchTool20250305Param{MaxUses: anthropic.Int(10)}},

		customTool("find_sources",
			"List ranked, vetted authoritative primary sources for a city to seed research.",
			map[string]any{"city_slug": strProp("the city slug, e.g. 'boston'")},
			[]string{"city_slug"}),

		customTool("fetch_source",
			"Fetch a source URL and return its readable text. Read a source through this tool before citing it — quotes are verified against text returned here.",
			map[string]any{"url": strProp("http(s) URL of the primary source")},
			[]string{"url"}),

		// Kept in step with drafting.SaveDraftInput by hand: the MCP front-end
		// derives its schema from that struct, this one is written out. A field
		// added there and not here is simply invisible to the batch worker.
		customTool("save_draft_playbook",
			"Save a DRAFT page. Every citation quote must be verbatim in a fetched source, or the save is rejected. Never publishes.",
			map[string]any{
				"city_slug":  strProp("city slug (use the one from the task)"),
				"topic_slug": strProp("WHAT the page is about. A slug from list_topics; topics are a fixed set and cannot be created here"),
				"title":      strProp("page title"),
				"intro_md":   strProp("short Markdown intro for the page"),
				"page_kind":  strProp("HOW the page is laid out, chosen separately from topic_slug (which is what it is about): 'playbook' renders a numbered argument, 'directory' renders a list of organisations. Omit for playbook. topic_slug 'resource-directory' must use 'directory' or the save is rejected."),
				"statements": map[string]any{"type": "array", "items": statementItems},
				"language":   strProp("'en' (default) or 'es'. 'es' must be a translation of the existing English playbook (see get_playbook), reusing its exact citations, not independent Spanish research — except resource-directory, which should research fresh Spanish-serving organisations."),
			},
			[]string{"city_slug", "topic_slug", "title", "statements"}),

		customTool("list_jurisdictions",
			"List the cities known to the platform (slug + name).",
			map[string]any{}, nil),

		customTool("list_topics",
			"List topics that already have a published playbook for a city, to avoid duplicating coverage.",
			map[string]any{
				"city_slug": strProp("the city slug"),
				"language":  strProp("'en' (default) or 'es'. has_page reflects coverage in this language only."),
			},
			[]string{"city_slug"}),

		customTool("get_playbook",
			"Fetch an existing published playbook for a city+topic. Returns not-found if none exists.",
			map[string]any{
				"city_slug":  strProp("city slug"),
				"topic_slug": strProp("topic slug"),
				"language":   strProp("'en' (default) or 'es'. Fetch 'en' as the source of truth before translating."),
			},
			[]string{"city_slug", "topic_slug"}),
	}
}
