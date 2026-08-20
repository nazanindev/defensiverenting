package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/http/handlers"
	"github.com/nazanindev/defensiverenting/internal/store"
)

// Every 404 variant carries the same explanation of why a page can be
// missing; this is the sentence the assertions below key on.
const whyMissing = "checks every citation"

// A URL that matches no route at all gets the styled fallback.
func TestNotFound_fallbackExplainsManualCuration(t *testing.T) {
	rec := httptest.NewRecorder()
	handlers.NotFound()(rec, httptest.NewRequest(http.MethodGet, "/no-such-page", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"We can't find that page", whyMissing, `href="/locations"`, `href="/contact"`} {
		if !strings.Contains(body, want) {
			t.Errorf("fallback 404 missing %q", want)
		}
	}
}

// A place we do not know is an uncovered place, not a dead end: the page says
// coverage is added one place at a time and links where to go.
func TestNotFound_uncoveredPlace(t *testing.T) {
	rec := serve(t, scopeStub(), "/j/springfield")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"We don't cover this place yet", whyMissing, `href="/locations"`} {
		if !strings.Contains(body, want) {
			t.Errorf("uncovered-place 404 missing %q", want)
		}
	}
}

// An unknown segment under a known state (neither a city nor a registry
// topic) reads as an uncovered place, and the state hub is offered.
func TestNotFound_uncoveredPlaceUnderState(t *testing.T) {
	rec := serve(t, scopeStub(), "/j/massachusetts/springfield")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "We don't cover this place yet") {
		t.Error("expected the uncovered-place story")
	}
	if !strings.Contains(body, `href="/j/massachusetts"`) {
		t.Error("expected a link to the known parent's hub")
	}
}

// A covered city missing one registry topic names both and offers the nearest
// guide up the ancestor chain, since that guide's law applies in the city.
func TestNotFound_missingTopicOffersNearestGuide(t *testing.T) {
	stub := nationalStub()
	stub.playbookErr = store.ErrNotFound
	stub.topicCoverage = map[int64]bool{10: true, 100: true} // state and country only

	rec := serve(t, stub, "/j/massachusetts/boston/security-deposits")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"No Security Deposits guide for Boston yet",
		`href="/j/massachusetts/security-deposits"`,
		"Massachusetts law applies in Boston",
		`href="/j/massachusetts/boston"`, // everything we have for the city
		whyMissing,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing-topic 404 missing %q", want)
		}
	}
}

// A registry topic with no published guides anywhere says "not written yet"
// rather than pretending the topic does not exist.
func TestNotFound_topicNotWrittenYet(t *testing.T) {
	stub := &stubStore{topics: []store.Topic{{ID: 1, Slug: "security-deposits", Name: "Security Deposits"}}}
	rec := serve(t, stub, "/t/security-deposits")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "No Security Deposits guides yet") {
		t.Errorf("expected the not-written-yet story, got: %.200s", body)
	}
}
