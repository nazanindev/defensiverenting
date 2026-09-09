package drafting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/nazanindev/defensiverenting/internal/discover"
	"github.com/nazanindev/defensiverenting/internal/store"
	"github.com/nazanindev/defensiverenting/internal/voice"
)

// ---- find_sources ----------------------------------------------------------

type FindSourcesInput struct {
	JurisdictionSlug string `json:"jurisdiction_slug" jsonschema:"the target jurisdiction's slug: a city like \"boston\", a state like \"massachusetts\", or \"united-states\" for a nationwide page"`
}

type Candidate struct {
	URL        string  `json:"url"`
	Publisher  string  `json:"publisher"`
	Title      string  `json:"title"`
	KindGuess  string  `json:"kind_guess" jsonschema:"statute|regulation|gov_guidance|nonprofit|editorial|court_ruling"`
	Rationale  string  `json:"rationale"`
	Confidence float64 `json:"confidence" jsonschema:"0..1, higher is more authoritative/relevant"`
	Via        string  `json:"via" jsonschema:"how it was found: registry|search"`
}

type FindSourcesOutput struct {
	Candidates []Candidate `json:"candidates"`
}

// FindSources returns ranked, vetted candidate primary sources for a
// jurisdiction: a city, a state, or the country.
func (tb *Toolbelt) FindSources(_ context.Context, in FindSourcesInput) (FindSourcesOutput, error) {
	slug := strings.TrimSpace(in.JurisdictionSlug)
	if slug == "" {
		return FindSourcesOutput{}, reject("jurisdiction_slug is required")
	}
	cands := discover.Run(slug, discover.DefaultProviders()...)
	out := FindSourcesOutput{Candidates: make([]Candidate, 0, len(cands))}
	for _, c := range cands {
		out.Candidates = append(out.Candidates, Candidate{
			URL: c.URL, Publisher: c.Publisher, Title: c.Title,
			KindGuess: c.KindGuess, Rationale: c.Rationale,
			Confidence: c.Confidence, Via: c.Via,
		})
	}
	return out, nil
}

// ---- fetch_source ----------------------------------------------------------

type FetchSourceInput struct {
	URL string `json:"url" jsonschema:"the http(s) URL of the source to fetch and cache. Reference-only sites (Nolo, Justia, law firm blogs) MAY be fetched to orient your research and compare your own copy, but citations to them are rejected: cite the primary law they summarize."`
}

type FetchSourceOutput struct {
	URL       string `json:"url"`
	Host      string `json:"host"`
	Text      string `json:"text" jsonschema:"readable text of the source; cite verbatim lines from this"`
	Truncated bool   `json:"truncated" jsonschema:"true if text was cut for length (full text is still cached for citation checks)"`
	Via       string `json:"via,omitempty" jsonschema:"how the text was obtained when a fallback was needed, e.g. \"web.archive.org snapshot\"; tell the human reviewer when set"`
}

// FetchSource fetches a URL, caches its extracted text for the verbatim check,
// and returns the (possibly truncated) text.
func (tb *Toolbelt) FetchSource(_ context.Context, in FetchSourceInput) (FetchSourceOutput, error) {
	raw := strings.TrimSpace(in.URL)
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return FetchSourceOutput{}, reject("url must be a valid http(s) URL, got %q", raw)
	}
	f, err := tb.fetch(raw)
	if err != nil {
		return FetchSourceOutput{}, reject("could not fetch %s: %v", raw, err)
	}
	tb.cache.put(raw, f.Text)

	returned, truncated := f.Text, false
	if r := []rune(f.Text); len(r) > maxReturnRune {
		returned, truncated = string(r[:maxReturnRune]), true
	}
	return FetchSourceOutput{URL: raw, Host: u.Host, Text: returned, Truncated: truncated, Via: f.Via}, nil
}

// ---- save_draft_playbook ---------------------------------------------------

type CitationInput struct {
	URL       string `json:"url" jsonschema:"source URL, must have been fetched via fetch_source"`
	Publisher string `json:"publisher"`
	Kind      string `json:"kind" jsonschema:"statute|regulation|gov_guidance|nonprofit|editorial|court_ruling"`
	Locator   string `json:"locator" jsonschema:"section pointer, e.g. \"§ 15B\" (optional)"`
	Quote     string `json:"quote" jsonschema:"the verbatim line from the source that backs the statement"`
}

type StatementInput struct {
	BodyMD string `json:"body_md" jsonschema:"one atomic, plain-language claim in Markdown"`
	// Key ties a revised statement to the claim it replaces (ADR-014 D1), so
	// a revision of a live page keeps the identity of every statement it
	// kept or reworded. Unchanged bodies inherit it anyway; the field matters
	// when the wording changed.
	Key     string `json:"key,omitempty" jsonschema:"when revising an existing page, the key of the statement this one replaces, copied from get_playbook. Omit for a statement that is new to the page."`
	Concept string `json:"concept,omitempty" jsonschema:"optional concept slug from the closed registry, tagging a claim that recurs across jurisdictions (e.g. retaliation-protection, deposit-return-deadline). Any registry slug is accepted on any page — tag the statement making the claim wherever it lives. Leave page-specific procedure untagged. Never invent slugs."`
	// TopicRef marks a statement that summarizes a whole subject the site
	// covers as its own pages, rather than making one claim.
	TopicRef  string          `json:"topic_ref,omitempty" jsonschema:"optional topic slug from list_topics, for a statement that is a one-paragraph summary of an entire subject (e.g. a fundamentals statement about safe housing points at repairs-and-habitability). Mutually exclusive with concept. Never set it to this page's own topic."`
	Citations []CitationInput `json:"citations" jsonschema:"at least one citation quoting a fetched source"`
}

type SaveDraftInput struct {
	// JurisdictionSlug scopes the page: a city, a state, or "united-states"
	// for a nationwide page. Tenant law layers, and a playbook may hang off
	// any level (see store.ListAuthorableJurisdictions).
	JurisdictionSlug string `json:"jurisdiction_slug"`
	// No topic_name: topics are a closed registry, so the display name comes
	// from the topics table and is never supplied by the caller.
	TopicSlug string `json:"topic_slug" jsonschema:"a slug from list_topics, e.g. \"security-deposits\""`
	Title     string `json:"title"`
	IntroMD   string `json:"intro_md" jsonschema:"short Markdown intro for the playbook"`
	// PageKind selects the layout. A directory renders each statement as an
	// entry in a list of local help rather than as a step in an argument, so
	// "where to get help" belongs on its own directory page under the
	// resource-directory topic instead of repeated at the foot of every
	// playbook.
	PageKind   string           `json:"page_kind,omitempty" jsonschema:"playbook|directory|faq|checklist. Omit for playbook. Use \"directory\" for a page listing local organisations and services, where each statement is one organisation and its citation is that organisation's own page."`
	Statements []StatementInput `json:"statements"`
	// Language keys this playbook against jurisdiction+topic+language, so a
	// translation is a separate row beside the English one rather than a
	// replacement for it. Citation quotes still stay verbatim in whatever
	// language the fetched source is actually written in (usually English
	// statute text) regardless of this field: it governs title/intro_md/
	// body_md only. See the citation-research skill and the drafting system
	// prompt for how a translation should be produced.
	Language string `json:"language,omitempty" jsonschema:"language code for this draft's renter-facing text: \"en\" (default) or \"es\". Use \"es\" only to translate an existing English playbook (fetch it with get_playbook first and reuse its exact citations) or to draft resource-directory content aimed at Spanish speakers — never invent Spanish legal claims independently of the English research."`
}

type SaveDraftOutput struct {
	JurisdictionSlug string `json:"jurisdiction_slug"`
	TopicSlug        string `json:"topic_slug"`
	PageKind         string `json:"page_kind" jsonschema:"the layout the draft was saved with; check it matches what you intended"`
	Language         string `json:"language" jsonschema:"the language code the draft was saved under"`
	StatementCount   int    `json:"statement_count"`
	CitationCount    int    `json:"citation_count"`
	Status           string `json:"status"`
	Message          string `json:"message"`
}

// SaveDraft validates every citation quote against the cached fetched source
// text, then writes a status="draft" playbook. Guardrail failures are returned
// as *RejectionError so the caller can hand them back to the model to fix.
func (tb *Toolbelt) SaveDraft(ctx context.Context, in SaveDraftInput) (SaveDraftOutput, error) {
	if strings.TrimSpace(in.JurisdictionSlug) == "" || strings.TrimSpace(in.TopicSlug) == "" || strings.TrimSpace(in.Title) == "" {
		return SaveDraftOutput{}, reject("jurisdiction_slug, topic_slug and title are required")
	}
	if len(in.Statements) == 0 {
		return SaveDraftOutput{}, reject("a playbook needs at least one statement")
	}

	jur, err := tb.db.GetJurisdictionBySlug(ctx, in.JurisdictionSlug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return SaveDraftOutput{}, reject("no jurisdiction with slug %q — call list_jurisdictions to see valid slugs", in.JurisdictionSlug)
		}
		return SaveDraftOutput{}, err
	}

	pageKind, err := resolvePageKind(in.PageKind)
	if err != nil {
		return SaveDraftOutput{}, err
	}
	if err := checkTopicLayout(in.TopicSlug, pageKind); err != nil {
		return SaveDraftOutput{}, err
	}
	lang, err := ResolveLanguage(in.Language)
	if err != nil {
		return SaveDraftOutput{}, err
	}

	// Guardrail: topics are shared across jurisdictions; a place-prefixed
	// topic slug fragments cross-city hubs and produces
	// /j/{city}/{city}-{topic} URLs.
	if strings.HasPrefix(in.TopicSlug, in.JurisdictionSlug+"-") {
		return SaveDraftOutput{}, reject("topic_slug %q is prefixed with the jurisdiction slug — topics are shared across every place. Use %q instead (call list_topics to see existing shared topics).",
			in.TopicSlug, strings.TrimPrefix(in.TopicSlug, in.JurisdictionSlug+"-"))
	}

	// A published page in this slot is not an error: the draft becomes a
	// proposed revision that sits beside it. The live page is untouched and
	// stays live, and nothing reaches the public until a human publishes the
	// revision, which retires the old page rather than deleting it.
	//
	// This used to be refused outright, back when one row could exist per slot
	// and writing a draft would have overwritten live legal content. See
	// migration 000015.
	revises := false
	if _, err := tb.db.GetPlaybook(ctx, in.JurisdictionSlug, in.TopicSlug, lang); err == nil {
		revises = true
	} else if !errors.Is(err, store.ErrNotFound) {
		return SaveDraftOutput{}, err
	}

	// Guardrail: renter-facing text must pass the editorial-voice lint for
	// the language it's written in. Citation quotes are exempt: they must
	// stay verbatim source text regardless of the draft's language.
	texts := map[string]string{"title": in.Title, "intro_md": in.IntroMD}
	for si, st := range in.Statements {
		texts[fmt.Sprintf("statement %d body_md", si+1)] = st.BodyMD
	}
	if violations := voice.LintAll(lang, texts); len(violations) > 0 {
		return SaveDraftOutput{}, reject("draft rejected by the editorial-voice lint. Rewrite the flagged text in plain language and save again (do NOT change citation quotes):\n- %s", strings.Join(violations, "\n- "))
	}

	// Guardrail: every citation quote must be verbatim in the cached fetched text.
	citationCount := 0
	for si, st := range in.Statements {
		if strings.TrimSpace(st.BodyMD) == "" {
			return SaveDraftOutput{}, reject("statement %d has empty body_md", si+1)
		}
		if len(st.Citations) == 0 {
			return SaveDraftOutput{}, reject("statement %d (%q) has no citations — every statement must cite a source", si+1, truncate(st.BodyMD, 60))
		}
		for ci, c := range st.Citations {
			if strings.TrimSpace(c.URL) == "" || strings.TrimSpace(c.Quote) == "" {
				return SaveDraftOutput{}, reject("statement %d citation %d needs both a url and a quote", si+1, ci+1)
			}
			// Reference-only sites (lawyer marketing, content mills) may be
			// fetched to orient, never cited. Rejected here with the fix
			// spelled out, before UpsertSource refuses the row less helpfully.
			if discover.ReferenceOnly(c.URL) {
				return SaveDraftOutput{}, reject("statement %d citation %d cites %s, which is reference-only: lawyer marketing and content-mill sites are never sources. Reading it to orient was fine. Now find the statute, regulation, or official guidance it summarizes, fetch_source that, and cite it instead.", si+1, ci+1, c.URL)
			}
			cached, ok := tb.cache.get(strings.TrimSpace(c.URL))
			if !ok {
				return SaveDraftOutput{}, reject("statement %d citation %d cites %s but that URL was never fetched — call fetch_source(%q) first", si+1, ci+1, c.URL, c.URL)
			}
			if !QuoteAppearsIn(cached, c.Quote) {
				return SaveDraftOutput{}, reject("statement %d citation %d quote is NOT present verbatim in %s — quote exact text returned by fetch_source. Offending quote: %q", si+1, ci+1, c.URL, truncate(c.Quote, 120))
			}
			citationCount++
		}
	}

	// Upsert each distinct source and map URL -> source id.
	jID := jur.ID
	srcID := map[string]int64{}
	for _, st := range in.Statements {
		for _, c := range st.Citations {
			u := strings.TrimSpace(c.URL)
			if _, done := srcID[u]; done {
				continue
			}
			src, err := tb.db.UpsertSource(ctx, store.UpsertSourceParams{
				URL:            u,
				Publisher:      c.Publisher,
				JurisdictionID: &jID,
				Kind:           defaultKind(c.Kind),
			})
			if err != nil {
				return SaveDraftOutput{}, err
			}
			srcID[u] = src.ID
		}
	}

	// Topics are a closed registry: a draft may reuse one, never invent one.
	// Adding a topic is an editorial decision made in a migration, not a side
	// effect of an agent saving a page. See docs/ADRs/ADR-005 D5.
	topic, err := tb.db.GetTopicBySlug(ctx, in.TopicSlug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return SaveDraftOutput{}, reject("no topic with slug %q. Call list_topics for the full set and pick the closest match. Topics are shared across every city and are not created by drafting.", in.TopicSlug)
		}
		return SaveDraftOutput{}, err
	}

	// Guardrail: a concept tag must come from the closed registry and belong to
	// this page's topic or the cross-cutting renting-fundamentals set (ADR-011
	// D2). Same shape as the topic check above: the agent may reuse the
	// vocabulary, never extend it, and the rejection names the valid choices.
	if err := tb.validateConcepts(ctx, in.TopicSlug, in.Statements); err != nil {
		return SaveDraftOutput{}, err
	}

	stmts := make([]store.IngestStatementParams, 0, len(in.Statements))
	for _, st := range in.Statements {
		cites := make([]store.IngestCitationParams, 0, len(st.Citations))
		for _, c := range st.Citations {
			cites = append(cites, store.IngestCitationParams{
				SourceID: srcID[strings.TrimSpace(c.URL)],
				Locator:  c.Locator,
				Quote:    c.Quote,
				// The guardrail above matched this quote against the text
				// fetch_source returned in this session, so the citation is
				// checked as of this save.
				CheckedNow: true,
				CheckedBy:  store.ActorDraftingAgent,
			})
		}
		stmts = append(stmts, store.IngestStatementParams{
			BodyMD:       st.BodyMD,
			Key:          st.Key,
			Language:     lang,
			ConceptSlug:  strings.TrimSpace(st.Concept),
			TopicRefSlug: strings.TrimSpace(st.TopicRef),
			Sources:      cites,
		})
	}

	if err := tb.db.IngestPlaybook(ctx, store.IngestPlaybookParams{
		JurisdictionID: jID,
		TopicID:        topic.ID,
		Language:       lang,
		Slug:           in.TopicSlug,
		Title:          in.Title,
		IntroMD:        in.IntroMD,
		PageKind:       pageKind,
		Status:         "draft",
		Statements:     stmts,
		UpdatedBy:      store.ActorDraftingAgent,
	}); err != nil {
		return SaveDraftOutput{}, err
	}

	return SaveDraftOutput{
		JurisdictionSlug: in.JurisdictionSlug,
		TopicSlug:        in.TopicSlug,
		PageKind:         pageKind,
		Language:         lang,
		StatementCount:   len(in.Statements),
		CitationCount:    citationCount,
		Status:           "draft",
		Message:          savedMessage(revises),
	}, nil
}

// ---- read helpers ----------------------------------------------------------

type ListJurisdictionsOutput struct {
	Jurisdictions []JurisdictionOut `json:"jurisdictions"`
}

type JurisdictionOut struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
	Kind string `json:"kind" jsonschema:"country|state|city"`
}

// ListJurisdictions returns every jurisdiction a page can target, not just
// cities: tenant law layers, and a playbook may hang off any level. The
// city-only listing meant the drafting agent was never shown "united-states"
// as a valid target even though the store accepted it.
func (tb *Toolbelt) ListJurisdictions(ctx context.Context) (ListJurisdictionsOutput, error) {
	js, err := tb.db.ListAuthorableJurisdictions(ctx)
	if err != nil {
		return ListJurisdictionsOutput{}, err
	}
	out := ListJurisdictionsOutput{Jurisdictions: make([]JurisdictionOut, 0, len(js))}
	for _, j := range js {
		out.Jurisdictions = append(out.Jurisdictions, JurisdictionOut{Slug: j.Slug, Name: j.Name, Kind: j.Kind})
	}
	return out, nil
}

type ListTopicsInput struct {
	JurisdictionSlug string `json:"jurisdiction_slug"`
	Language         string `json:"language,omitempty" jsonschema:"language to check coverage for: \"en\" (default) or \"es\". has_page reflects this language, not any other."`
}

type ListTopicsOutput struct {
	Topics []TopicOut `json:"topics"`
}

type TopicOut struct {
	Slug    string `json:"slug"`
	Name    string `json:"name"`
	IsCore  bool   `json:"is_core" jsonschema:"true for the topics every city should cover"`
	HasPage bool   `json:"has_page" jsonschema:"true when this city already has a published playbook for the topic in the requested language"`
}

// ListTopics returns the whole topic registry, marking which ones this city
// already covers.
//
// It used to return only topics already published in the given city, which is
// empty for every new city — exactly when the agent needs the vocabulary most.
// With nothing to reuse, drafting runs invented slugs, which is how a second
// topic vocabulary came to exist alongside the first. See docs/ADRs/ADR-005 D5.
func (tb *Toolbelt) ListTopics(ctx context.Context, in ListTopicsInput) (ListTopicsOutput, error) {
	jur, err := tb.db.GetJurisdictionBySlug(ctx, in.JurisdictionSlug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ListTopicsOutput{}, reject("no jurisdiction with slug %q", in.JurisdictionSlug)
		}
		return ListTopicsOutput{}, err
	}
	lang, err := ResolveLanguage(in.Language)
	if err != nil {
		return ListTopicsOutput{}, err
	}
	registry, err := tb.db.ListTopicRegistry(ctx)
	if err != nil {
		return ListTopicsOutput{}, err
	}
	covered, err := tb.db.ListTopicsByJurisdiction(ctx, jur.ID, lang)
	if err != nil {
		return ListTopicsOutput{}, err
	}
	hasPage := make(map[int64]bool, len(covered))
	for _, t := range covered {
		hasPage[t.ID] = true
	}
	out := ListTopicsOutput{Topics: make([]TopicOut, 0, len(registry))}
	for _, t := range registry {
		out.Topics = append(out.Topics, TopicOut{
			Slug: t.Slug, Name: t.Name, IsCore: t.IsCore, HasPage: hasPage[t.ID],
		})
	}
	return out, nil
}

type GetPlaybookInput struct {
	JurisdictionSlug string `json:"jurisdiction_slug"`
	TopicSlug        string `json:"topic_slug"`
	Language         string `json:"language,omitempty" jsonschema:"language of the version to fetch: \"en\" (default) or \"es\". Fetch language=\"en\" as the source of truth before translating it into another language."`
}

type GetPlaybookOutput struct {
	Title      string         `json:"title"`
	IntroMD    string         `json:"intro_md"`
	Language   string         `json:"language"`
	Statements []StatementOut `json:"statements"`
}

type StatementOut struct {
	// Key is the statement's durable identity; pass it back on
	// save_draft_playbook when revising this statement.
	Key       string        `json:"key"`
	BodyMD    string        `json:"body_md"`
	Citations []CitationOut `json:"citations"`
}

type CitationOut struct {
	SourceURL string `json:"source_url"`
	Publisher string `json:"publisher"`
	Locator   string `json:"locator"`
	Quote     string `json:"quote"`
}

func (tb *Toolbelt) GetPlaybook(ctx context.Context, in GetPlaybookInput) (GetPlaybookOutput, error) {
	lang, err := ResolveLanguage(in.Language)
	if err != nil {
		return GetPlaybookOutput{}, err
	}
	pb, err := tb.db.GetPlaybook(ctx, in.JurisdictionSlug, in.TopicSlug, lang)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return GetPlaybookOutput{}, reject("no published playbook for %s/%s in language %q yet", in.JurisdictionSlug, in.TopicSlug, lang)
		}
		return GetPlaybookOutput{}, err
	}
	out := GetPlaybookOutput{Title: pb.Title, IntroMD: pb.IntroMD, Language: lang}
	for _, st := range pb.Statements {
		so := StatementOut{Key: st.Key, BodyMD: st.BodyMD}
		for _, c := range st.Citations {
			so.Citations = append(so.Citations, CitationOut{
				SourceURL: c.SourceURL, Publisher: c.Publisher, Locator: c.Locator, Quote: c.Quote,
			})
		}
		out.Statements = append(out.Statements, so)
	}
	return out, nil
}

// savedMessage tells the caller whether this draft stands alone or proposes a
// replacement for a page that is currently live, since the consequence of
// publishing it differs.
func savedMessage(revises bool) string {
	if revises {
		return "Draft saved as a proposed revision of the page that is currently live. " +
			"The live page is unchanged. Publishing this revision replaces it and retires the old version; nothing was published."
	}
	return "Draft saved. It is visible in the authoring tool for the human author to verify and publish; nothing was published."
}

// validateConcepts checks every statement's concept tag and topic reference
// against their closed registries before anything is written. The rejection
// lists the valid choices so the agent's retry is a choice, not a guess — the
// same contract the topic-slug check keeps.
func (tb *Toolbelt) validateConcepts(ctx context.Context, topicSlug string, stmts []StatementInput) error {
	var tagged bool
	for _, st := range stmts {
		if strings.TrimSpace(st.Concept) != "" || strings.TrimSpace(st.TopicRef) != "" {
			tagged = true
			break
		}
	}
	if !tagged {
		return nil
	}
	concepts, err := tb.db.ListConcepts(ctx)
	if err != nil {
		return err
	}
	// Any registry concept is taggable on any page — claims cross topics
	// constantly, so the old own-topic-or-cross-cutting restriction hid most
	// of the vocabulary (dropped 2026-08-22, matching the form). Only
	// invented slugs are rejected.
	allowed := map[string]bool{}
	var choices []string
	for _, c := range concepts {
		allowed[c.Slug] = true
		choices = append(choices, c.Slug)
	}
	topics, err := tb.db.ListTopicRegistry(ctx)
	if err != nil {
		return err
	}
	topicOK := map[string]bool{}
	var topicChoices []string
	for _, t := range topics {
		topicOK[t.Slug] = true
		topicChoices = append(topicChoices, t.Slug)
	}
	for si, st := range stmts {
		concept := strings.TrimSpace(st.Concept)
		topicRef := strings.TrimSpace(st.TopicRef)
		if concept != "" && topicRef != "" {
			return reject("statement %d sets both concept %q and topic_ref %q. A statement is one claim or one whole-topic summary, never both (ADR-011 D7). Drop one.",
				si+1, concept, topicRef)
		}
		if concept != "" && !allowed[concept] {
			return reject("statement %d concept %q is not in the registry. Concepts are a closed registry (ADR-011): pick one of [%s] or omit the field. Do not invent concepts.",
				si+1, concept, strings.Join(choices, ", "))
		}
		if topicRef != "" && (!topicOK[topicRef] || topicRef == topicSlug) {
			return reject("statement %d topic_ref %q must be a registry topic other than this page's own (%q). Valid: [%s].",
				si+1, topicRef, topicSlug, strings.Join(topicChoices, ", "))
		}
	}
	// Several statements on one page may share a tag: a claim often takes more
	// than one statement to state (2026-08-22, reversing the one-per-page rule
	// from the day before). Only the first tagged statement renders the
	// public anchor, so the HTML stays valid; see BuildPlaybookPage.
	return nil
}

// ---- propose_statement -----------------------------------------------------

// ProposeStatementInput files one per-statement change into the review queue
// (ADR-014) instead of saving a page. The same guardrails as
// save_draft_playbook apply to the replacement: voice lint on the body, and
// every quote verbatim in text this session fetched.
type ProposeStatementInput struct {
	StatementKey string `json:"statement_key" jsonschema:"the key of the statement to change, from get_playbook"`
	Reason       string `json:"reason" jsonschema:"why: \"agent-pass:<name>\" for a named bulk pass (e.g. agent-pass:rerun-sources), \"source-drift\" when a cited quote no longer appears at its source, \"source-quality:<signal>\" for a structurally weak source (guidance-not-statute, archive-url, no-locator, single-citation)"`
	Note         string `json:"note" jsonschema:"one or two sentences for the reviewer: what you found and why this change follows. Stored as evidence."`
	// Statement is the replacement. Omit it to file a finding with no
	// replacement, which the reviewer handles in the editor.
	Statement *StatementInput `json:"statement,omitempty" jsonschema:"the statement as it should read afterward: body_md, optional concept or topic_ref, and citations with verbatim quotes from sources fetched this session. Omit to file a finding only."`
	Evidence  map[string]any  `json:"evidence,omitempty" jsonschema:"optional structured evidence, e.g. {\"old_quote\": \"…\", \"new_quote\": \"…\", \"source_url\": \"…\"}. Rendered to the reviewer as-is."`
}

type ProposeStatementOutput struct {
	ProposalID int64  `json:"proposal_id"`
	Page       string `json:"page"`
	Message    string `json:"message"`
}

func (tb *Toolbelt) ProposeStatement(ctx context.Context, in ProposeStatementInput) (ProposeStatementOutput, error) {
	if !store.ValidReason(in.Reason) {
		return ProposeStatementOutput{}, reject("reason %q must be agent-pass:<name>, source-drift, or source-quality:<signal>", in.Reason)
	}
	if strings.TrimSpace(in.Note) == "" {
		return ProposeStatementOutput{}, reject("note is required: tell the reviewer what you found")
	}
	var proposed *store.ProposedStatement
	if st := in.Statement; st != nil {
		if strings.TrimSpace(st.BodyMD) == "" {
			return ProposeStatementOutput{}, reject("statement.body_md is empty; omit statement to file a finding only")
		}
		if len(st.Citations) == 0 {
			return ProposeStatementOutput{}, reject("statement has no citations — every statement must cite a source")
		}
		// The language is the page's; the lint needs it. Look it up from
		// the key rather than trusting a caller-supplied value.
		lang, err := tb.languageOfKey(ctx, in.StatementKey)
		if err != nil {
			return ProposeStatementOutput{}, err
		}
		if violations := voice.LintAll(lang, map[string]string{"body_md": st.BodyMD}); len(violations) > 0 {
			return ProposeStatementOutput{}, reject("rejected by the editorial-voice lint. Rewrite in plain language (do NOT change citation quotes):\n- %s", strings.Join(violations, "\n- "))
		}
		proposed = &store.ProposedStatement{BodyMD: st.BodyMD, Concept: strings.TrimSpace(st.Concept), TopicRef: strings.TrimSpace(st.TopicRef)}
		for ci, c := range st.Citations {
			if c.Kind == "editorial" {
				proposed.Citations = append(proposed.Citations, store.ProposedCitation{Editorial: true})
				continue
			}
			u := strings.TrimSpace(c.URL)
			if u == "" || strings.TrimSpace(c.Quote) == "" {
				return ProposeStatementOutput{}, reject("citation %d needs both a url and a quote", ci+1)
			}
			if discover.ReferenceOnly(u) {
				return ProposeStatementOutput{}, reject("citation %d cites %s, which is reference-only: lawyer marketing and content-mill sites are never sources. Cite the statute, regulation, or official guidance it summarizes.", ci+1, u)
			}
			cached, ok := tb.cache.get(u)
			if !ok {
				return ProposeStatementOutput{}, reject("citation %d cites %s but that URL was never fetched — call fetch_source(%q) first", ci+1, u, u)
			}
			if !QuoteAppearsIn(cached, c.Quote) {
				return ProposeStatementOutput{}, reject("citation %d quote is NOT present verbatim in %s — quote exact text returned by fetch_source. Offending quote: %q", ci+1, u, truncate(c.Quote, 120))
			}
			proposed.Citations = append(proposed.Citations, store.ProposedCitation{
				URL: u, Publisher: c.Publisher, Kind: defaultKind(c.Kind), Locator: c.Locator, Quote: c.Quote,
				// Matched against this session's fetch, so the approval can
				// stamp it on the proposer's word.
				Checked: true,
			})
		}
	}
	evidence := map[string]any{"note": in.Note}
	for k, v := range in.Evidence {
		evidence[k] = v
	}
	evJSON, err := json.Marshal(evidence)
	if err != nil {
		return ProposeStatementOutput{}, err
	}
	id, err := tb.db.FileProposal(ctx, store.FileProposalParams{
		StatementKey: in.StatementKey, Reason: in.Reason, Proposed: proposed,
		Evidence: evJSON, ProposedBy: store.ActorDraftingAgent,
	})
	if err != nil {
		return ProposeStatementOutput{}, reject("%v", err)
	}
	row, err := tb.db.GetProposal(ctx, id)
	if err != nil {
		return ProposeStatementOutput{}, err
	}
	msg := "Filed for review. Nothing changed on the page; a person decides on the review queue."
	if proposed == nil {
		msg = "Filed as a finding with no replacement. Nothing changed on the page."
	}
	return ProposeStatementOutput{ProposalID: id, Page: row.Title, Message: msg}, nil
}

// languageOfKey finds the language of the page carrying a statement key, so
// the voice lint runs the right ruleset.
func (tb *Toolbelt) languageOfKey(ctx context.Context, key string) (string, error) {
	// A cheap probe through the proposal reader: file nothing, just resolve.
	// FileProposal resolves the page itself; here the language is needed
	// before filing, so ask the store for pages carrying the key.
	lang, err := tb.db.LanguageOfStatementKey(ctx, key)
	if errors.Is(err, store.ErrNotFound) {
		return "", reject("no page carries a statement with key %q; take keys from get_playbook", key)
	}
	return lang, err
}
