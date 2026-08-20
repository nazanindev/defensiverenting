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

// nationalStub is scopeStub plus the country above the state, so the ancestor
// chain boston → massachusetts → united-states is complete.
func nationalStub() *stubStore {
	s := scopeStub()
	s.jurisdictions = append([]store.Jurisdiction{
		{ID: 100, Kind: "country", Name: "United States", Slug: "united-states"},
	}, s.jurisdictions...)
	for i := range s.jurisdictions {
		if s.jurisdictions[i].Kind == "state" {
			s.jurisdictions[i].ParentID = ptr(100)
			s.jurisdictions[i].ParentSlug = "united-states"
			s.jurisdictions[i].ParentName = "United States"
		}
	}
	return s
}

// A located reader whose city lacks the topic should land on the nearest guide
// up the chain — the state's, else the national one — not on the picker. This
// is the ?j= counterpart of upward-only search scoping.
func TestTopicHub_locationScopeFallsBackUpTheChain(t *testing.T) {
	cases := []struct {
		name     string
		coverage map[int64]bool
		want     string
	}{
		{"city lacks it, state has it", map[int64]bool{10: true, 100: true}, "/j/massachusetts/security-deposits"},
		{"only the national guide exists", map[int64]bool{100: true}, "/j/united-states/security-deposits"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := nationalStub()
			stub.topicCoverage = tc.coverage
			rec := serve(t, stub, "/t/security-deposits?j=boston")
			if rec.Code != http.StatusFound {
				t.Fatalf("status = %d, want 302", rec.Code)
			}
			if got := rec.Header().Get("Location"); got != tc.want {
				t.Errorf("Location = %q, want %q", got, tc.want)
			}
		})
	}
}

// A national guide is neither a city nor "Statewide"; it gets its own section
// after the statewide one.
func TestTopicHub_nationalGuideGetsItsOwnSection(t *testing.T) {
	rec := serve(t, nationalStub(), "/t/security-deposits")
	body := rec.Body.String()

	if !strings.Contains(body, "Nationwide guide") {
		t.Fatal("expected a nationwide section, since the stub's country has this topic")
	}
	if !strings.Contains(body, `href="/j/united-states/security-deposits"`) {
		t.Error("nationwide section should link the national guide")
	}
	statewide := strings.Index(body, "Statewide guides")
	national := strings.Index(body, "Nationwide guide")
	if statewide == -1 || statewide > national {
		t.Errorf("statewide guides should be listed before the nationwide one (statewide=%d national=%d)", statewide, national)
	}
}

// /locations lists national guides in a section of their own rather than
// grouping "United States" as if it were a city.
func TestLocations_nationalHubGetsItsOwnSection(t *testing.T) {
	rec := serve(t, nationalStub(), "/locations")
	body := rec.Body.String()

	if !strings.Contains(body, "Nationwide") {
		t.Fatal("expected a nationwide section on /locations")
	}
	if !strings.Contains(body, `href="/j/united-states"`) {
		t.Error("nationwide section should link the national hub")
	}
	if strings.Contains(body, ">Other<") {
		t.Error("the country must not fall into the parentless 'Other' city group")
	}
}

// /api/coverage backs the homepage's situation filter: the topics that resolve
// to a real guide for a location, so the list never shows a situation that
// would dead-end on the picker.
func TestCoverage_reportsTopicsForALocation(t *testing.T) {
	rec := serve(t, nationalStub(), "/api/coverage?j=boston")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"security-deposits"`) {
		t.Errorf("coverage body missing the stub topic: %s", body)
	}

	if rec := serve(t, nationalStub(), "/api/coverage?j=not-a-place"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown location: status = %d, want 404", rec.Code)
	}
	if rec := serve(t, nationalStub(), "/api/coverage"); rec.Code != http.StatusBadRequest {
		t.Errorf("missing j: status = %d, want 400", rec.Code)
	}
}
