package handlers_test

import (
	"net/http"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// Hierarchical routing: /j/{state}/{city}/{topic}. Two segments are ambiguous
// in shape between a state playbook and a city hub, so these pin the
// classification and the redirects that keep the old flat URLs alive.

func hierarchyStub() *stubStore {
	return &stubStore{
		jurisdictions: []store.Jurisdiction{
			{ID: 1, Kind: "country", Name: "United States", Slug: "united-states"},
			{ID: 2, Kind: "state", Name: "Texas", Slug: "texas", ParentSlug: "united-states"},
			{ID: 3, Kind: "city", Name: "Austin", Slug: "austin", ParentSlug: "texas"},
			{ID: 4, Kind: "state", Name: "Illinois", Slug: "illinois", ParentSlug: "united-states"},
			{ID: 5, Kind: "city", Name: "Chicago", Slug: "chicago", ParentSlug: "illinois"},
		},
		topics: []store.Topic{{ID: 1, Slug: "security-deposits", Name: "Security Deposits"}},
	}
}

// The 19 URLs live before this change were flat. They must keep working, and
// they do so without any alias rows: the handler reads the parent off the
// jurisdiction and builds the address.
func TestFlatCityURLRedirectsToHierarchy(t *testing.T) {
	stub := hierarchyStub()
	stub.playbookErr = store.ErrNotFound
	assertRedirect(t, serve(t, stub, "/j/austin"), "/j/texas/austin")
	assertRedirect(t, serve(t, stub, "/j/austin/security-deposits"), "/j/texas/austin/security-deposits")
}

// A flat URL whose topic was also renamed must land in one hop, not bounce
// through an intermediate address.
func TestFlatCityURLWithRetiredTopicRedirectsOnce(t *testing.T) {
	stub := hierarchyStub()
	stub.playbookErr = store.ErrNotFound
	stub.topicAliases = map[string]string{"security-deposit-not-returned": "security-deposits"}
	assertRedirect(t, serve(t, stub, "/j/austin/security-deposit-not-returned"),
		"/j/texas/austin/security-deposits")
}

// /j/{a}/{b} is a city hub when b is a city under a, and a state playbook when
// b is a topic. Both shapes have to resolve from the same route.
func TestTwoSegmentDisambiguation(t *testing.T) {
	stub := hierarchyStub()
	if rec := serve(t, stub, "/j/texas/austin"); rec.Code != http.StatusOK {
		t.Errorf("city hub: status = %d, want 200", rec.Code)
	}
	if rec := serve(t, stub, "/j/texas/security-deposits"); rec.Code != http.StatusOK {
		t.Errorf("state playbook: status = %d, want 200", rec.Code)
	}
}

// A city addressed under the wrong state is a real mistake, not a 404: send it
// to the right address rather than losing the visitor.
func TestCityUnderWrongStateRedirects(t *testing.T) {
	stub := hierarchyStub()
	assertRedirect(t, serve(t, stub, "/j/illinois/austin"), "/j/texas/austin")
	assertRedirect(t, serve(t, stub, "/j/illinois/austin/security-deposits"),
		"/j/texas/austin/security-deposits")
}

func TestStateHubServesAtOneSegment(t *testing.T) {
	stub := hierarchyStub()
	if rec := serve(t, stub, "/j/texas"); rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// Redirect targets must be reachable. A redirect that lands on another redirect
// burns crawl budget and dilutes the signal a 301 is supposed to pass on.
func TestRedirectsLandOnTheirFinalAddress(t *testing.T) {
	stub := hierarchyStub()
	stub.playbookErr = store.ErrNotFound
	stub.topicAliases = map[string]string{"security-deposit-not-returned": "security-deposits"}

	for _, start := range []string{
		"/j/austin",
		"/j/austin/security-deposits",
		"/j/austin/security-deposit-not-returned",
		"/j/illinois/austin",
	} {
		rec := serve(t, stub, start)
		if rec.Code != http.StatusMovedPermanently {
			t.Errorf("%s: expected a redirect, got %d", start, rec.Code)
			continue
		}
		next := rec.Header().Get("Location")
		if second := serve(t, stub, next); second.Code == http.StatusMovedPermanently {
			t.Errorf("%s redirected to %s, which redirects again to %s",
				start, next, second.Header().Get("Location"))
		}
	}
}

// A city with no parent addresses itself flatly, so the flat-URL redirect must
// not fire: it would send the URL to itself forever. Found by running the site
// against a database where the hierarchy repair had not been applied.
func TestParentlessCityDoesNotRedirectToItself(t *testing.T) {
	stub := &stubStore{
		jurisdictions: []store.Jurisdiction{{ID: 1, Kind: "city", Name: "Boston", Slug: "boston"}},
		topics:        []store.Topic{{ID: 1, Slug: "heat-not-working", Name: "Heat Not Working"}},
	}
	rec := serve(t, stub, "/j/boston/heat-not-working")
	if rec.Code == http.StatusMovedPermanently {
		t.Fatalf("redirected to %q, which is the same URL", rec.Header().Get("Location"))
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}
