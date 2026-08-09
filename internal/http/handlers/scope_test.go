package handlers_test

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

func ptr(v int64) *int64 { return &v }

// scopeStub builds a store with one state and two cities under it.
func scopeStub() *stubStore {
	return &stubStore{
		jurisdictions: []store.Jurisdiction{
			{ID: 10, Kind: "state", Name: "Massachusetts", Slug: "massachusetts"},
			{ID: 1, Kind: "city", Name: "Boston", Slug: "boston", ParentID: ptr(10), ParentSlug: "massachusetts", ParentName: "Massachusetts"},
			{ID: 2, Kind: "city", Name: "Cambridge", Slug: "cambridge", ParentID: ptr(10), ParentSlug: "massachusetts", ParentName: "Massachusetts"},
		},
		topics: []store.Topic{{ID: 1, Slug: "security-deposits", Name: "Security Deposits"}},
	}
}

var cityCardRe = regexp.MustCompile(`class="card card--link" href="/j/`)

// The homepage must not grow a box per city. Cities reach it only as options on
// the location scope; the browsable list lives at /locations. Without this the
// page silently returns to being a wall as coverage expands.
func TestIndex_citiesAreScopeOptionsNotCards(t *testing.T) {
	rec := serve(t, scopeStub(), "/")
	body := rec.Body.String()

	if got := cityCardRe.FindAllString(body, -1); len(got) != 0 {
		t.Errorf("homepage rendered %d per-city cards, want 0", len(got))
	}
	for _, want := range []string{
		`<option value="boston"`,
		`<option value="cambridge"`,
		`<optgroup label="Massachusetts">`,
		`href="/locations"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("homepage missing %q", want)
		}
	}
}

func TestLocations_listsEveryCityGroupedByState(t *testing.T) {
	rec := serve(t, scopeStub(), "/locations")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{`href="/j/massachusetts"`, `href="/j/massachusetts/boston"`, `href="/j/massachusetts/cambridge"`} {
		if !strings.Contains(body, want) {
			t.Errorf("/locations missing %q", want)
		}
	}
}

// A state hub used to render "no playbooks yet" and link nowhere, stranding
// every city beneath it.
func TestStateHub_listsChildCities(t *testing.T) {
	rec := serve(t, scopeStub(), "/j/massachusetts")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Cities in Massachusetts") {
		t.Error("state hub should have a cities section")
	}
	for _, want := range []string{`href="/j/massachusetts/boston"`, `href="/j/massachusetts/cambridge"`} {
		if !strings.Contains(body, want) {
			t.Errorf("state hub missing %q", want)
		}
	}
}

// A reader with a location set should land on their city's guide, not a picker.
func TestTopicHub_locationScopeRedirectsToTheCityGuide(t *testing.T) {
	rec := serve(t, scopeStub(), "/t/security-deposits?j=boston")

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "/j/massachusetts/boston/security-deposits" {
		t.Errorf("Location = %q, want /j/massachusetts/boston/security-deposits", got)
	}
}

// Resolving the scope server-side is only safe if an uncovered or bogus city
// degrades to the picker. A redirect here would 404 the reader instead.
func TestTopicHub_unknownLocationFallsThroughToThePicker(t *testing.T) {
	for _, slug := range []string{"not-a-city", ""} {
		target := "/t/security-deposits"
		if slug != "" {
			target += "?j=" + slug
		}
		rec := serve(t, scopeStub(), target)
		if rec.Code != http.StatusOK {
			t.Errorf("j=%q: status = %d, want 200", slug, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Choose your city") {
			t.Errorf("j=%q: expected the city picker", slug)
		}
	}
}

// State-level guides are listed apart from cities; grouped by parent they would
// file under a "United States" heading inside "Choose your city".
func TestTopicHub_statewideGuidesAreSeparateFromCities(t *testing.T) {
	stub := scopeStub()
	rec := serve(t, stub, "/t/security-deposits")
	body := rec.Body.String()

	if !strings.Contains(body, "Statewide guides") {
		t.Fatal("expected a statewide section, since the stub's state has this topic")
	}
	cities := strings.Index(body, "Choose your city")
	statewide := strings.Index(body, "Statewide guides")
	if cities == -1 || statewide == -1 || cities > statewide {
		t.Errorf("cities should be listed before statewide guides (city=%d statewide=%d)", cities, statewide)
	}
	if !strings.Contains(body, `href="/j/massachusetts/security-deposits"`) {
		t.Error("statewide section should link the state's own guide")
	}
}
