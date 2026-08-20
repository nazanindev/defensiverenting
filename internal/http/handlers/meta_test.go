package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/http/handlers"
	"github.com/nazanindev/defensiverenting/internal/store"
)

type sitemapStub struct {
	jurisdictions []store.Jurisdiction
	entries       []store.SitemapEntry
	topicsByLang  map[string][]store.Topic
}

func (s *sitemapStub) ListPublishedHubJurisdictions(_ context.Context) ([]store.Jurisdiction, error) {
	return s.jurisdictions, nil
}
func (s *sitemapStub) ListSitemapURLs(_ context.Context) ([]store.SitemapEntry, error) {
	return s.entries, nil
}
func (s *sitemapStub) ListPublishedTopics(_ context.Context, lang string) ([]store.Topic, error) {
	return s.topicsByLang[lang], nil
}

// ADR-007 D7: the sitemap must carry every language a playbook actually
// exists in, not just English — otherwise a translated page a search engine
// would otherwise find gets no crawl signal from the sitemap at all.
func TestSitemap_emitsBothLanguages(t *testing.T) {
	stub := &sitemapStub{
		entries: []store.SitemapEntry{
			{JurisdictionSlug: "boston", JurisdictionParentSlug: "massachusetts", JurisdictionKind: "city", TopicSlug: "security-deposits", Language: "en"},
			{JurisdictionSlug: "boston", JurisdictionParentSlug: "massachusetts", JurisdictionKind: "city", TopicSlug: "security-deposits", Language: "es"},
		},
		topicsByLang: map[string][]store.Topic{
			"en": {{Slug: "security-deposits"}},
			"es": {{Slug: "security-deposits"}},
		},
	}
	rec := httptest.NewRecorder()
	handlers.Sitemap(stub, "https://renterlaw.org")(rec, httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"<loc>https://renterlaw.org/j/massachusetts/boston/security-deposits</loc>",
		"<loc>https://renterlaw.org/es/j/massachusetts/boston/security-deposits</loc>",
		"<loc>https://renterlaw.org/t/security-deposits</loc>",
		"<loc>https://renterlaw.org/es/t/security-deposits</loc>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("sitemap missing %q", want)
		}
	}
}

// Hub pages of every kind belong in the sitemap. The city-only listing left
// /j/united-states (and any statewide hub) published but never submitted.
func TestSitemap_listsNationalAndStateHubs(t *testing.T) {
	stub := &sitemapStub{
		jurisdictions: []store.Jurisdiction{
			{ID: 1, Kind: "country", Name: "United States", Slug: "united-states"},
			{ID: 2, Kind: "state", Name: "Massachusetts", Slug: "massachusetts", ParentSlug: "united-states"},
			{ID: 3, Kind: "city", Name: "Boston", Slug: "boston", ParentSlug: "massachusetts"},
		},
		topicsByLang: map[string][]store.Topic{"en": {}, "es": {}},
	}
	rec := httptest.NewRecorder()
	handlers.Sitemap(stub, "https://renterlaw.org")(rec, httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil))
	body := rec.Body.String()
	for _, want := range []string{
		"<loc>https://renterlaw.org/j/united-states</loc>",
		"<loc>https://renterlaw.org/j/massachusetts</loc>",
		"<loc>https://renterlaw.org/j/massachusetts/boston</loc>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("sitemap missing %q", want)
		}
	}
}

// A topic with no Spanish coverage yet must not get an empty /es/t/ entry —
// the sitemap should only claim what's actually reachable.
func TestSitemap_omitsUncoveredLanguageForATopic(t *testing.T) {
	stub := &sitemapStub{
		topicsByLang: map[string][]store.Topic{
			"en": {{Slug: "eviction-defense"}},
			"es": {},
		},
	}
	rec := httptest.NewRecorder()
	handlers.Sitemap(stub, "https://renterlaw.org")(rec, httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil))
	body := rec.Body.String()
	if strings.Contains(body, "/es/t/eviction-defense") {
		t.Error("must not list an /es/t/ page for a topic with no Spanish coverage")
	}
	if !strings.Contains(body, "<loc>https://renterlaw.org/t/eviction-defense</loc>") {
		t.Error("the English entry should still be present")
	}
}
