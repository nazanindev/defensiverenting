package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nazanindev/defensiverenting/internal/http/handlers"
	"github.com/nazanindev/defensiverenting/internal/store"
	"log/slog"
	"os"
)

// stubStore implements browseStore with in-memory data for handler tests.
type stubStore struct {
	jurisdictions []store.Jurisdiction
	topics        []store.Topic
	playbook      store.PlaybookWithStatements
	playbookErr   error
	// retired slug -> live slug, for the alias-driven 301s
	jurisdictionAliases map[string]string
	topicAliases        map[string]string

	// otherLangPlaybooks, when non-nil, makes GetPlaybook answer per the
	// requested language (keyed by language code; a miss is store.ErrNotFound)
	// instead of the single playbook/playbookErr every other test in this
	// package relies on regardless of what language is asked. Nil (the zero
	// value) is the pre-existing behavior — only set this in a test that
	// specifically probes language-toggle/hreflang logic.
	otherLangPlaybooks map[string]store.PlaybookWithStatements

	// conceptHubTopics is returned by ConceptHubTopics: concept slug -> the
	// topic hub where that claim is localized (ADR-011 D4, amended).
	conceptHubTopics map[string]string

	// terms and conceptPage feed the reference layer (ADR-012).
	terms       []store.Term
	conceptPage store.ConceptPageData

	// topicCoverage, when non-nil, restricts which jurisdiction IDs
	// GetNearestTopicJurisdiction treats as having a published guide. Nil (the
	// zero value) means every jurisdiction in the stub has one, which keeps
	// the ?j= redirect resolving to the selected city itself, the behavior
	// every pre-existing test relies on.
	topicCoverage map[int64]bool
}

func (s *stubStore) ResolveJurisdictionAlias(_ context.Context, alias string) (store.Jurisdiction, error) {
	live, ok := s.jurisdictionAliases[alias]
	if !ok {
		return store.Jurisdiction{}, store.ErrNotFound
	}
	for _, j := range s.jurisdictions {
		if j.Slug == live {
			return j, nil
		}
	}
	return store.Jurisdiction{Slug: live}, nil
}

func (s *stubStore) ResolveTopicAlias(_ context.Context, alias string) (store.Topic, error) {
	live, ok := s.topicAliases[alias]
	if !ok {
		return store.Topic{}, store.ErrNotFound
	}
	for _, t := range s.topics {
		if t.Slug == live {
			return t, nil
		}
	}
	return store.Topic{Slug: live}, nil
}

func (s *stubStore) ConceptHubTopics(_ context.Context, _ string) (map[string]string, error) {
	return s.conceptHubTopics, nil
}

func (s *stubStore) ListTerms(_ context.Context, _ string) ([]store.Term, error) {
	return s.terms, nil
}

func (s *stubStore) GetConceptPage(_ context.Context, slug, _ string) (store.ConceptPageData, error) {
	if s.conceptPage.Concept.Slug != slug {
		return store.ConceptPageData{}, store.ErrNotFound
	}
	return s.conceptPage, nil
}

func (s *stubStore) ListPublishedCityJurisdictions(_ context.Context) ([]store.Jurisdiction, error) {
	return s.jurisdictions, nil
}

func (s *stubStore) ListPublishedHubJurisdictions(_ context.Context) ([]store.Jurisdiction, error) {
	return s.jurisdictions, nil
}

func (s *stubStore) GetNearestTopicJurisdiction(_ context.Context, jurisdictionID, _ int64, _ string) (store.Jurisdiction, error) {
	byID := make(map[int64]store.Jurisdiction, len(s.jurisdictions))
	for _, j := range s.jurisdictions {
		byID[j.ID] = j
	}
	j, ok := byID[jurisdictionID]
	for ok {
		if s.topicCoverage == nil || s.topicCoverage[j.ID] {
			return j, nil
		}
		if j.ParentID == nil {
			break
		}
		j, ok = byID[*j.ParentID]
	}
	return store.Jurisdiction{}, store.ErrNotFound
}

func (s *stubStore) ListTopicsByJurisdictionRecursive(_ context.Context, _ int64, _ string) ([]store.Topic, error) {
	return s.topics, nil
}

func (s *stubStore) ListPublishedChildCities(_ context.Context, parentID int64) ([]store.Jurisdiction, error) {
	var out []store.Jurisdiction
	for _, j := range s.jurisdictions {
		if j.Kind == "city" && j.ParentID != nil && *j.ParentID == parentID {
			out = append(out, j)
		}
	}
	return out, nil
}

func (s *stubStore) GetJurisdictionBySlug(_ context.Context, slug string) (store.Jurisdiction, error) {
	for _, j := range s.jurisdictions {
		if j.Slug == slug {
			return j, nil
		}
	}
	return store.Jurisdiction{}, store.ErrNotFound
}

func (s *stubStore) ListTopicsByJurisdiction(_ context.Context, _ int64, _ string) ([]store.Topic, error) {
	return s.topics, nil
}

func (s *stubStore) GetPlaybook(_ context.Context, _, _, lang string) (store.PlaybookWithStatements, error) {
	if s.otherLangPlaybooks != nil {
		pb, ok := s.otherLangPlaybooks[lang]
		if !ok {
			return store.PlaybookWithStatements{}, store.ErrNotFound
		}
		return pb, nil
	}
	return s.playbook, s.playbookErr
}

func (s *stubStore) GetTopicBySlug(_ context.Context, slug string) (store.Topic, error) {
	for _, t := range s.topics {
		if t.Slug == slug {
			return t, nil
		}
	}
	return store.Topic{}, store.ErrNotFound
}

func (s *stubStore) ListPublishedTopics(_ context.Context, _ string) ([]store.Topic, error) {
	return s.topics, nil
}

func (s *stubStore) ListJurisdictionsByTopic(_ context.Context, _ int64, _ string) ([]store.Jurisdiction, error) {
	return s.jurisdictions, nil
}

func logger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, nil))
}

// The homepage reaches every covered city through the location scope rather
// than a link per city. See TestIndex_citiesAreScopeOptionsNotCards for the
// shape that assertion depends on.
func TestIndexHandler_offersJurisdictionAsScope(t *testing.T) {
	stub := &stubStore{
		jurisdictions: []store.Jurisdiction{
			{ID: 1, Kind: "city", Name: "Boston", Slug: "boston", ParentSlug: "massachusetts", ParentName: "Massachusetts"},
		},
	}
	r := chi.NewRouter()
	handlers.Browse(r, stub, logger())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `<option value="boston">Boston</option>`) {
		t.Error("homepage should offer Boston as a location scope option")
	}
}

func TestJurisdictionHandler_notFound(t *testing.T) {
	stub := &stubStore{}
	r := chi.NewRouter()
	handlers.Browse(r, stub, logger())

	req := httptest.NewRequest(http.MethodGet, "/j/nowhere", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestPlaybookHandler_citationChips(t *testing.T) {
	stub := &stubStore{
		jurisdictions: []store.Jurisdiction{{ID: 1, Kind: "city", Name: "Boston", Slug: "boston", ParentSlug: "massachusetts"}},
		topics:        []store.Topic{{ID: 1, Slug: "heat-not-working", Name: "Heat Not Working"}},
		playbook: store.PlaybookWithStatements{
			Playbook: store.Playbook{
				ID: 1, Title: "Heat Not Working",
				Slug: "heat-not-working", Language: "en",
			},
			Jurisdiction: store.Jurisdiction{Name: "Boston", Slug: "boston"},
			Topic:        store.Topic{Name: "Heat Not Working", Slug: "heat-not-working"},
			Statements: []store.CitedStatement{
				{
					ID:     1,
					BodyMD: "Massachusetts requires heat of at least 68°F.",
					Citations: []store.CitationWithSource{
						{
							SourceID:   1,
							SourceURL:  "https://www.mass.gov/regulations/105-CMR-41000",
							Publisher:  "Massachusetts DPH",
							SourceKind: "regulation",
							Locator:    "§ 410.200",
						},
					},
				},
			},
		},
	}

	r := chi.NewRouter()
	handlers.Browse(r, stub, logger())

	req := httptest.NewRequest(http.MethodGet, "/j/massachusetts/boston/heat-not-working", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Massachusetts DPH") {
		t.Error("response should contain citation publisher")
	}
	if !strings.Contains(body, "105-CMR") {
		t.Error("response should contain citation URL fragment")
	}
}

func TestPlaybookHandler_playbookNotFound(t *testing.T) {
	stub := &stubStore{
		jurisdictions: []store.Jurisdiction{{ID: 1, Kind: "city", Name: "Boston", Slug: "boston", ParentSlug: "massachusetts"}},
		playbookErr:   store.ErrNotFound,
	}
	r := chi.NewRouter()
	handlers.Browse(r, stub, logger())

	req := httptest.NewRequest(http.MethodGet, "/j/massachusetts/boston/no-such-topic", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// localHelpStub serves a Boston playbook whose city publishes the topics given.
func localHelpStub(publishedTopics []store.Topic, viewing store.Topic) *stubStore {
	return &stubStore{
		jurisdictions: []store.Jurisdiction{{ID: 1, Kind: "city", Name: "Boston", Slug: "boston", ParentSlug: "massachusetts"}},
		topics:        publishedTopics,
		playbook: store.PlaybookWithStatements{
			Playbook: store.Playbook{ID: 1, Title: viewing.Name, Slug: viewing.Slug, Language: "en"},
			// Kind matters: Path() emits the state segment only for a city, so a
			// fixture without it silently produces the pre-hierarchy flat URL.
			Jurisdiction: store.Jurisdiction{ID: 1, Kind: "city", Name: "Boston", Slug: "boston", ParentSlug: "massachusetts"},
			Topic:        viewing,
			Statements: []store.CitedStatement{{
				ID: 1, BodyMD: "A claim.",
				Citations: []store.CitationWithSource{{
					SourceID: 1, SourceURL: "https://example.gov/law",
					Publisher: "Example", SourceKind: "statute", Locator: "§ 1",
				}},
			}},
		},
	}
}

func getPlaybookBody(t *testing.T, stub *stubStore, path string) string {
	t.Helper()
	r := chi.NewRouter()
	handlers.Browse(r, stub, logger())
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	return rec.Body.String()
}

var (
	evictionTopic  = store.Topic{ID: 1, Slug: "eviction-defense", Name: "Eviction"}
	localHelpTopic = store.Topic{ID: 2, Slug: "resource-directory", Name: "Local Help"}
)

// Someone reading about eviction may need a phone number more than the next
// paragraph of law, so the link sits above the guide rather than among the
// sibling-topic links at the foot.
func TestPlaybookHandler_linksToLocalHelpWhenTheCityHasOne(t *testing.T) {
	body := getPlaybookBody(t,
		localHelpStub([]store.Topic{evictionTopic, localHelpTopic}, evictionTopic),
		"/j/massachusetts/boston/eviction-defense")

	if !strings.Contains(body, `href="/j/massachusetts/boston/resource-directory"`) {
		t.Error("no link to the city's Local Help page")
	}
	if !strings.Contains(body, "help-bar") {
		t.Error("the Local Help link should render as the help bar above the guide")
	}
	// The generic dead-end sentence is replaced by the real link, not doubled up.
	if strings.Contains(body, "contact your local legal aid organization") {
		t.Error("the generic 'contact your local legal aid organization' should give way to the link")
	}
}

// A link to a page that does not exist is worse than no link, so the bar only
// renders off the city's own published-topic list.
func TestPlaybookHandler_noLocalHelpLinkWhenTheCityHasNone(t *testing.T) {
	body := getPlaybookBody(t,
		localHelpStub([]store.Topic{evictionTopic}, evictionTopic),
		"/j/massachusetts/boston/eviction-defense")

	if strings.Contains(body, "help-bar") {
		t.Error("a city with no Local Help page must not get a help bar")
	}
	if !strings.Contains(body, "contact your local legal aid organization") {
		t.Error("without a link, the generic fallback sentence should remain")
	}
}

func TestPlaybookHandler_localHelpPageDoesNotLinkToItself(t *testing.T) {
	body := getPlaybookBody(t,
		localHelpStub([]store.Topic{evictionTopic, localHelpTopic}, localHelpTopic),
		"/j/massachusetts/boston/resource-directory")

	if strings.Contains(body, "help-bar") {
		t.Error("the Local Help page must not link to itself")
	}
}

// A national statement tagged with a concept renders its anchor and points
// the reader at the hub of the topic where that claim is localized (ADR-011
// D4, amended) — the follow-up "check your state" never had. A statement
// referencing a whole topic (D7) instead links to that topic's hub with the
// full-guides line. A statement with neither gets no link.
func TestPlaybookHandler_nationalStatementTagLinks(t *testing.T) {
	stub := &stubStore{
		jurisdictions: []store.Jurisdiction{
			{ID: 9, Kind: "country", Name: "United States", Slug: "united-states"},
			{ID: 2, Kind: "state", Name: "Massachusetts", Slug: "massachusetts"},
		},
		topics: []store.Topic{
			{ID: 4, Slug: "security-deposits", Name: "Security Deposits"},
			{ID: 5, Slug: "repairs-and-habitability", Name: "Repairs and Unsafe Conditions"},
		},
		conceptHubTopics: map[string]string{"retaliation-protection": "security-deposits"},
		playbook: store.PlaybookWithStatements{
			Playbook:     store.Playbook{ID: 1, Title: "Security Deposits", Slug: "security-deposits", Language: "en"},
			Jurisdiction: store.Jurisdiction{ID: 9, Kind: "country", Name: "United States", Slug: "united-states"},
			Topic:        store.Topic{ID: 4, Name: "Security Deposits", Slug: "security-deposits"},
			Statements: []store.CitedStatement{
				{
					ID:          1,
					BodyMD:      "Your landlord cannot punish you for using your rights.",
					ConceptSlug: "retaliation-protection",
					Citations: []store.CitationWithSource{{
						SourceID: 1, SourceURL: "https://example.gov/law", Publisher: "HUD", SourceKind: "gov_guidance",
					}},
				},
				{
					ID:           2,
					BodyMD:       "Your home must be safe and fit to live in.",
					TopicRefSlug: "repairs-and-habitability",
					TopicRefName: "Repairs and Unsafe Conditions",
					Citations: []store.CitationWithSource{{
						SourceID: 1, SourceURL: "https://example.gov/law", Publisher: "HUD", SourceKind: "gov_guidance",
					}},
				},
			},
		},
	}

	r := chi.NewRouter()
	handlers.Browse(r, stub, logger())

	req := httptest.NewRequest(http.MethodGet, "/j/united-states/security-deposits", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `id="retaliation-protection"`) {
		t.Error("concept-tagged statement must render its concept slug as an anchor")
	}
	if !strings.Contains(body, `href="/t/security-deposits"`) {
		t.Error("concept-tagged national statement must link to the hub where the claim is localized")
	}
	if !strings.Contains(body, `href="/t/repairs-and-habitability"`) {
		t.Error("topic-referencing statement must link to the referenced topic's hub")
	}
	if !strings.Contains(body, "We have full guides on this.") {
		t.Error("topic-referencing statement must render the full-guides line")
	}
}

// The per-statement trust line is an attestation, so it renders fully earned
// or not at all: every non-editorial citation confirmed live, dated by the
// stalest confirmation. A statement with one unconfirmed quote shows nothing.
func TestPlaybookHandler_statementTrustLine(t *testing.T) {
	older := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	stub := &stubStore{
		jurisdictions: []store.Jurisdiction{{ID: 1, Kind: "city", Name: "Boston", Slug: "boston", ParentSlug: "massachusetts"}},
		topics:        []store.Topic{{ID: 1, Slug: "security-deposits", Name: "Security Deposits"}},
		playbook: store.PlaybookWithStatements{
			Playbook:     store.Playbook{ID: 1, Title: "T", Slug: "security-deposits", Language: "en"},
			Jurisdiction: store.Jurisdiction{Name: "Boston", Slug: "boston"},
			Topic:        store.Topic{Name: "Security Deposits", Slug: "security-deposits"},
			Statements: []store.CitedStatement{
				{
					ID: 1, BodyMD: "Fully confirmed statement.",
					Citations: []store.CitationWithSource{
						{SourceID: 1, SourceURL: "https://a.gov", Publisher: "A", SourceKind: "statute", CheckedAt: &newer},
						{SourceID: 2, SourceURL: "https://b.gov", Publisher: "B", SourceKind: "statute", CheckedAt: &older},
					},
				},
				{
					ID: 2, BodyMD: "Partially confirmed statement.",
					Citations: []store.CitationWithSource{
						{SourceID: 3, SourceURL: "https://c.gov", Publisher: "C", SourceKind: "statute"},
					},
				},
			},
		},
	}

	r := chi.NewRouter()
	handlers.Browse(r, stub, logger())
	req := httptest.NewRequest(http.MethodGet, "/j/massachusetts/boston/security-deposits", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "Sources checked August 1, 2026") {
		t.Error("fully confirmed statement must show the trust line dated by its stalest confirmation")
	}
	if got := strings.Count(body, "Sources checked"); got != 1 {
		t.Errorf("trust line rendered %d times, want exactly 1 — a statement with an unconfirmed quote must show nothing", got)
	}
}

// The reference layer (ADR-012): /c/{slug} assembles the national definition
// and every place's answer from published statements, each linking home at
// its concept anchor; /terms indexes them; the homepage lists terms the
// national pages define.
func TestConceptPage_definitionAndLocalAnswers(t *testing.T) {
	checked := time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)
	stub := &stubStore{
		conceptPage: store.ConceptPageData{
			Concept: store.Concept{Slug: "notice-to-quit", Name: "Notice to quit", TopicSlug: "eviction-defense"},
			National: &store.ConceptInstance{
				Jurisdiction: store.Jurisdiction{Kind: "country", Name: "United States", Slug: "united-states"},
				TopicSlug:    "eviction-defense",
				Statement: store.CitedStatement{
					ID: 1, BodyMD: "Before an eviction, your landlord must give you a notice to quit.",
					ConceptSlug: "notice-to-quit",
					Citations: []store.CitationWithSource{{
						SourceID: 1, SourceURL: "https://example.gov", Publisher: "HUD", SourceKind: "gov_guidance", CheckedAt: &checked,
					}},
				},
			},
			Local: []store.ConceptInstance{{
				Jurisdiction: store.Jurisdiction{Kind: "city", Name: "Pittsburgh", Slug: "pittsburgh", ParentSlug: "pennsylvania"},
				TopicSlug:    "eviction-defense",
				Statement: store.CitedStatement{
					ID: 2, BodyMD: "The notice must give you 10 days to move out.",
					ConceptSlug: "notice-to-quit",
					Citations: []store.CitationWithSource{{
						SourceID: 2, SourceURL: "https://pa.gov/law", Publisher: "PA General Assembly", SourceKind: "statute", CheckedAt: &checked,
					}},
				},
			}},
		},
	}

	r := chi.NewRouter()
	handlers.Browse(r, stub, logger())
	req := httptest.NewRequest(http.MethodGet, "/c/notice-to-quit", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "The general rule") || !strings.Contains(body, "notice to quit") {
		t.Error("page must lead with the general rule from the national statement")
	}
	if !strings.Contains(body, "Pittsburgh") || !strings.Contains(body, "10 days") {
		t.Error("page must carry each place's own answer")
	}
	if !strings.Contains(body, `/j/pennsylvania/pittsburgh/eviction-defense#notice-to-quit`) {
		t.Error("each answer must link home at its concept anchor")
	}
	if !strings.Contains(body, "Sources checked August 21, 2026") {
		t.Error("entries must carry the trust line they earn on their own pages")
	}

	// An unknown or content-less concept has no page.
	req = httptest.NewRequest(http.MethodGet, "/c/no-such-term", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown concept: status = %d, want 404", rec.Code)
	}
}

func TestTermsIndexAndHomepageSection(t *testing.T) {
	stub := &stubStore{
		terms: []store.Term{
			{Slug: "notice-to-quit", Name: "Notice to quit", TopicSlug: "eviction-defense", Blurb: "The letter that starts an eviction.", HasNational: true, Localized: 3},
			{Slug: "small-claims-court", Name: "Small claims court", TopicSlug: "renting-fundamentals", Blurb: "A low cost court for money disputes.", Localized: 2},
		},
	}
	r := chi.NewRouter()
	handlers.Browse(r, stub, logger())

	req := httptest.NewRequest(http.MethodGet, "/terms", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("/terms status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `href="/c/notice-to-quit"`) || !strings.Contains(body, "The letter that starts an eviction.") {
		t.Error("/terms must list every term with its blurb")
	}
	if !strings.Contains(body, `href="/c/small-claims-court"`) {
		t.Error("/terms must include terms without a national definition")
	}

	// The homepage lists only terms a national page defines (ADR-012 D3).
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	body = rec.Body.String()
	if !strings.Contains(body, "Legal terms, explained") || !strings.Contains(body, `href="/c/notice-to-quit"`) {
		t.Error("homepage must carry the reference section with nationally defined terms")
	}
	if strings.Contains(body, `href="/c/small-claims-court"`) {
		t.Error("homepage must not list terms with no national definition")
	}
}

// Statements may share a concept tag, but an HTML id must be unique: the
// first tagged statement owns the anchor and later ones render without it.
func TestPlaybookHandler_sharedConceptAnchorsOnce(t *testing.T) {
	stub := &stubStore{
		jurisdictions: []store.Jurisdiction{{ID: 1, Kind: "city", Name: "Boston", Slug: "boston", ParentSlug: "massachusetts"}},
		topics:        []store.Topic{{ID: 1, Slug: "security-deposits", Name: "Security Deposits"}},
		playbook: store.PlaybookWithStatements{
			Playbook:     store.Playbook{ID: 1, Title: "T", Slug: "security-deposits", Language: "en"},
			Jurisdiction: store.Jurisdiction{Name: "Boston", Slug: "boston"},
			Topic:        store.Topic{Name: "Security Deposits", Slug: "security-deposits"},
			Statements: []store.CitedStatement{
				{
					ID: 1, BodyMD: "Deposits come back in 30 days.", ConceptSlug: "deposit-return-deadline",
					Citations: []store.CitationWithSource{{SourceID: 1, SourceURL: "https://a.gov", Publisher: "A", SourceKind: "statute"}},
				},
				{
					ID: 2, BodyMD: "The clock starts at key handback.", ConceptSlug: "deposit-return-deadline",
					Citations: []store.CitationWithSource{{SourceID: 1, SourceURL: "https://a.gov", Publisher: "A", SourceKind: "statute"}},
				},
			},
		},
	}
	r := chi.NewRouter()
	handlers.Browse(r, stub, logger())
	req := httptest.NewRequest(http.MethodGet, "/j/massachusetts/boston/security-deposits", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if got := strings.Count(rec.Body.String(), `id="deposit-return-deadline"`); got != 1 {
		t.Errorf("anchor rendered %d times, want exactly 1", got)
	}
}
