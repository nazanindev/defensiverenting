package drafting

import (
	"context"
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
	CitySlug string `json:"city_slug" jsonschema:"the city's slug, e.g. \"boston\" or \"chicago\""`
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

// FindSources returns ranked, vetted candidate primary sources for a city.
func (tb *Toolbelt) FindSources(_ context.Context, in FindSourcesInput) (FindSourcesOutput, error) {
	slug := strings.TrimSpace(in.CitySlug)
	if slug == "" {
		return FindSourcesOutput{}, reject("city_slug is required")
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
	URL string `json:"url" jsonschema:"the http(s) URL of the primary source to fetch and cache"`
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
	BodyMD    string          `json:"body_md" jsonschema:"one atomic, plain-language claim in Markdown"`
	Citations []CitationInput `json:"citations" jsonschema:"at least one citation quoting a fetched source"`
}

type SaveDraftInput struct {
	CitySlug   string           `json:"city_slug"`
	// No topic_name: topics are a closed registry, so the display name comes
	// from the topics table and is never supplied by the caller.
	TopicSlug  string           `json:"topic_slug" jsonschema:"a slug from list_topics, e.g. \"security-deposits\""`
	Title      string           `json:"title"`
	IntroMD    string           `json:"intro_md" jsonschema:"short Markdown intro for the playbook"`
	Statements []StatementInput `json:"statements"`
}

type SaveDraftOutput struct {
	CitySlug       string `json:"city_slug"`
	TopicSlug      string `json:"topic_slug"`
	StatementCount int    `json:"statement_count"`
	CitationCount  int    `json:"citation_count"`
	Status         string `json:"status"`
	Message        string `json:"message"`
}

// SaveDraft validates every citation quote against the cached fetched source
// text, then writes a status="draft" playbook. Guardrail failures are returned
// as *RejectionError so the caller can hand them back to the model to fix.
func (tb *Toolbelt) SaveDraft(ctx context.Context, in SaveDraftInput) (SaveDraftOutput, error) {
	if strings.TrimSpace(in.CitySlug) == "" || strings.TrimSpace(in.TopicSlug) == "" || strings.TrimSpace(in.Title) == "" {
		return SaveDraftOutput{}, reject("city_slug, topic_slug and title are required")
	}
	if len(in.Statements) == 0 {
		return SaveDraftOutput{}, reject("a playbook needs at least one statement")
	}

	jur, err := tb.db.GetJurisdictionBySlug(ctx, in.CitySlug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return SaveDraftOutput{}, reject("no jurisdiction with slug %q — call list_jurisdictions to see valid cities", in.CitySlug)
		}
		return SaveDraftOutput{}, err
	}

	// Guardrail: topics are shared across cities; a city-prefixed topic slug
	// fragments cross-city hubs and produces /j/{city}/{city}-{topic} URLs.
	if strings.HasPrefix(in.TopicSlug, in.CitySlug+"-") {
		return SaveDraftOutput{}, reject("topic_slug %q is prefixed with the city slug — topics are shared across cities. Use %q instead (call list_topics to see existing shared topics).",
			in.TopicSlug, strings.TrimPrefix(in.TopicSlug, in.CitySlug+"-"))
	}

	// Refuse to clobber a PUBLISHED playbook. GetPlaybook returns only published
	// rows, so a not-found here means the slot is free or holds a draft (which
	// re-drafting may safely overwrite).
	if _, err := tb.db.GetPlaybook(ctx, in.CitySlug, in.TopicSlug, draftLanguage); err == nil {
		return SaveDraftOutput{}, reject("a PUBLISHED playbook already exists for %s/%s — refusing to overwrite it. Choose a different topic, or a human must unpublish it first.", in.CitySlug, in.TopicSlug)
	} else if !errors.Is(err, store.ErrNotFound) {
		return SaveDraftOutput{}, err
	}

	// Guardrail: renter-facing text must pass the editorial-voice lint.
	// Citation quotes are exempt: they must stay verbatim source text.
	texts := map[string]string{"title": in.Title, "intro_md": in.IntroMD}
	for si, st := range in.Statements {
		texts[fmt.Sprintf("statement %d body_md", si+1)] = st.BodyMD
	}
	if violations := voice.LintAll(texts); len(violations) > 0 {
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
			cached, ok := tb.cache.get(strings.TrimSpace(c.URL))
			if !ok {
				return SaveDraftOutput{}, reject("statement %d citation %d cites %s but that URL was never fetched — call fetch_source(%q) first", si+1, ci+1, c.URL, c.URL)
			}
			if !strings.Contains(normalizeForMatch(cached), normalizeForMatch(c.Quote)) &&
				!strings.Contains(stripAllWS(cached), stripAllWS(c.Quote)) {
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

	stmts := make([]store.IngestStatementParams, 0, len(in.Statements))
	for _, st := range in.Statements {
		cites := make([]store.IngestCitationParams, 0, len(st.Citations))
		for _, c := range st.Citations {
			cites = append(cites, store.IngestCitationParams{
				SourceID: srcID[strings.TrimSpace(c.URL)],
				Locator:  c.Locator,
				Quote:    c.Quote,
			})
		}
		stmts = append(stmts, store.IngestStatementParams{
			BodyMD:   st.BodyMD,
			Language: draftLanguage,
			Sources:  cites,
		})
	}

	if err := tb.db.IngestPlaybook(ctx, store.IngestPlaybookParams{
		JurisdictionID: jID,
		TopicID:        topic.ID,
		Language:       draftLanguage,
		Slug:           in.TopicSlug,
		Title:          in.Title,
		IntroMD:        in.IntroMD,
		PageKind:       "playbook",
		Status:         "draft",
		Statements:     stmts,
	}); err != nil {
		return SaveDraftOutput{}, err
	}

	return SaveDraftOutput{
		CitySlug:       in.CitySlug,
		TopicSlug:      in.TopicSlug,
		StatementCount: len(in.Statements),
		CitationCount:  citationCount,
		Status:         "draft",
		Message:        "Draft saved. It is visible in the authoring tool for the human author to verify and publish; nothing was published.",
	}, nil
}

// ---- read helpers ----------------------------------------------------------

type ListJurisdictionsOutput struct {
	Cities []JurisdictionOut `json:"cities"`
}

type JurisdictionOut struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

func (tb *Toolbelt) ListJurisdictions(ctx context.Context) (ListJurisdictionsOutput, error) {
	js, err := tb.db.ListCityJurisdictions(ctx)
	if err != nil {
		return ListJurisdictionsOutput{}, err
	}
	out := ListJurisdictionsOutput{Cities: make([]JurisdictionOut, 0, len(js))}
	for _, j := range js {
		out.Cities = append(out.Cities, JurisdictionOut{Slug: j.Slug, Name: j.Name})
	}
	return out, nil
}

type ListTopicsInput struct {
	CitySlug string `json:"city_slug"`
}

type ListTopicsOutput struct {
	Topics []TopicOut `json:"topics"`
}

type TopicOut struct {
	Slug    string `json:"slug"`
	Name    string `json:"name"`
	IsCore  bool   `json:"is_core" jsonschema:"true for the topics every city should cover"`
	HasPage bool   `json:"has_page" jsonschema:"true when this city already has a published playbook for the topic"`
}

// ListTopics returns the whole topic registry, marking which ones this city
// already covers.
//
// It used to return only topics already published in the given city, which is
// empty for every new city — exactly when the agent needs the vocabulary most.
// With nothing to reuse, drafting runs invented slugs, which is how a second
// topic vocabulary came to exist alongside the first. See docs/ADRs/ADR-005 D5.
func (tb *Toolbelt) ListTopics(ctx context.Context, in ListTopicsInput) (ListTopicsOutput, error) {
	jur, err := tb.db.GetJurisdictionBySlug(ctx, in.CitySlug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ListTopicsOutput{}, reject("no jurisdiction with slug %q", in.CitySlug)
		}
		return ListTopicsOutput{}, err
	}
	registry, err := tb.db.ListTopicRegistry(ctx)
	if err != nil {
		return ListTopicsOutput{}, err
	}
	covered, err := tb.db.ListTopicsByJurisdiction(ctx, jur.ID, draftLanguage)
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
	CitySlug  string `json:"city_slug"`
	TopicSlug string `json:"topic_slug"`
}

type GetPlaybookOutput struct {
	Title      string         `json:"title"`
	IntroMD    string         `json:"intro_md"`
	Statements []StatementOut `json:"statements"`
}

type StatementOut struct {
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
	pb, err := tb.db.GetPlaybook(ctx, in.CitySlug, in.TopicSlug, draftLanguage)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return GetPlaybookOutput{}, reject("no published playbook for %s/%s yet", in.CitySlug, in.TopicSlug)
		}
		return GetPlaybookOutput{}, err
	}
	out := GetPlaybookOutput{Title: pb.Title, IntroMD: pb.IntroMD}
	for _, st := range pb.Statements {
		so := StatementOut{BodyMD: st.BodyMD}
		for _, c := range st.Citations {
			so.Citations = append(so.Citations, CitationOut{
				SourceURL: c.SourceURL, Publisher: c.Publisher, Locator: c.Locator, Quote: c.Quote,
			})
		}
		out.Statements = append(out.Statements, so)
	}
	return out, nil
}
