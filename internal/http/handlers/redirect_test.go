package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/nazanindev/defensiverenting/internal/http/handlers"
	"github.com/nazanindev/defensiverenting/internal/store"
)

// A renamed slug must 301 to its current address rather than 404. Without this
// every indexed link to the old URL is lost, which is what the 2026-08-01 topic
// cleanup did to the /t/pittsburgh-* pages.

func serve(t *testing.T, stub *stubStore, path string) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	handlers.Browse(r, stub, logger())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func assertRedirect(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("status = %d, want 301", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
}

func TestJurisdictionHandler_retiredSlugRedirects(t *testing.T) {
	stub := &stubStore{
		jurisdictions:       []store.Jurisdiction{{ID: 1, Kind: "city", Name: "Boston", Slug: "boston", ParentSlug: "massachusetts"}},
		jurisdictionAliases: map[string]string{"boston-ma": "boston"},
	}
	assertRedirect(t, serve(t, stub, "/j/boston-ma"), "/j/massachusetts/boston")
}

func TestPlaybookHandler_retiredTopicRedirects(t *testing.T) {
	stub := &stubStore{
		jurisdictions: []store.Jurisdiction{{ID: 1, Kind: "city", Name: "Boston", Slug: "boston", ParentSlug: "massachusetts"}},
		topics:        []store.Topic{{ID: 1, Slug: "eviction-defense", Name: "Eviction & Notice to Quit"}},
		playbookErr:   store.ErrNotFound,
		topicAliases:  map[string]string{"notice-to-quit": "eviction-defense"},
	}
	assertRedirect(t, serve(t, stub, "/j/massachusetts/boston/notice-to-quit"), "/j/massachusetts/boston/eviction-defense")
}

// Both segments moving at once is the shape of the ADR-005 migration, which
// adds the state segment and renames topics in the same pass.
func TestPlaybookHandler_bothSlugsRetiredRedirectsOnce(t *testing.T) {
	stub := &stubStore{
		jurisdictions:       []store.Jurisdiction{{ID: 1, Kind: "city", Name: "Chicago", Slug: "chicago", ParentSlug: "illinois"}},
		topics:              []store.Topic{{ID: 1, Slug: "security-deposits", Name: "Security Deposits"}},
		playbookErr:         store.ErrNotFound,
		jurisdictionAliases: map[string]string{"chicago-il": "chicago"},
		topicAliases:        map[string]string{"security-deposit-not-returned": "security-deposits"},
	}
	rec := serve(t, stub, "/j/chicago-il/security-deposit-not-returned")
	assertRedirect(t, rec, "/j/illinois/chicago/security-deposits")
}

func TestTopicHubHandler_retiredSlugRedirects(t *testing.T) {
	stub := &stubStore{
		jurisdictions: []store.Jurisdiction{{ID: 1, Kind: "city", Name: "Pittsburgh", Slug: "pittsburgh", ParentSlug: "pennsylvania"}},
		topics:        []store.Topic{{ID: 1, Slug: "discrimination", Name: "Housing Discrimination"}},
		topicAliases:  map[string]string{"pittsburgh-discrimination": "discrimination"},
	}
	assertRedirect(t, serve(t, stub, "/t/pittsburgh-discrimination"), "/t/discrimination")
}

// A slug that was never in use must still 404. Redirecting every miss would
// turn typos into redirect loops and hand crawlers infinite URLs.
func TestRetiredSlugFallback_genuineMissStill404s(t *testing.T) {
	stub := &stubStore{
		jurisdictions:       []store.Jurisdiction{{ID: 1, Kind: "city", Name: "Boston", Slug: "boston", ParentSlug: "massachusetts"}},
		topics:              []store.Topic{{ID: 1, Slug: "eviction-defense", Name: "Eviction"}},
		playbookErr:         store.ErrNotFound,
		jurisdictionAliases: map[string]string{"boston-ma": "boston"},
		topicAliases:        map[string]string{"notice-to-quit": "eviction-defense"},
	}
	for _, path := range []string{"/j/atlantis", "/j/atlantis/nonsense", "/j/massachusetts/boston/nonsense", "/t/nonsense"} {
		if rec := serve(t, stub, path); rec.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", path, rec.Code)
		}
	}
}

// A live pair with no published playbook is a real miss, not a rename.
func TestPlaybookHandler_liveSlugsWithNoPlaybook404s(t *testing.T) {
	stub := &stubStore{
		jurisdictions: []store.Jurisdiction{{ID: 1, Kind: "city", Name: "Boston", Slug: "boston", ParentSlug: "massachusetts"}},
		topics:        []store.Topic{{ID: 1, Slug: "eviction-defense", Name: "Eviction"}},
		playbookErr:   store.ErrNotFound,
	}
	if rec := serve(t, stub, "/j/massachusetts/boston/eviction-defense"); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}
