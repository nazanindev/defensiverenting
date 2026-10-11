package templates_test

import (
	"strings"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
	tmpl "github.com/nazanindev/defensiverenting/web/templates"
)

// The place picker is one select in the header on every page (ADR-031 as
// amended 2026-10-11). On a guide it shows the page's place and submits to
// the topic hub, so a pick lands on the same topic in the new place. A
// nationwide guide repeats the picker under its title: the searcher who
// lands there typed a problem, not a place.

func withPlaces(t *testing.T) {
	t.Helper()
	prev := tmpl.PlaceList
	tmpl.PlaceList = func() ([]tmpl.StateGroup, []store.Jurisdiction) {
		return []tmpl.StateGroup{
			{Name: "Massachusetts", Slug: "massachusetts", Selectable: true, Cities: []store.Jurisdiction{{Name: "Boston", Slug: "boston", ParentSlug: "massachusetts"}}},
			{Name: "Ohio", Slug: "ohio", Selectable: true},
		}, []store.Jurisdiction{{Name: "United States", Slug: "united-states", Kind: "country"}}
	}
	t.Cleanup(func() { tmpl.PlaceList = prev })
}

func renderGuide(t *testing.T, j store.Jurisdiction) string {
	t.Helper()
	var sb strings.Builder
	err := tmpl.Render(&sb, tmpl.PlaybookPage{
		Playbook:     store.Playbook{Title: "No Heat in Your Rental: What Can I Do?", Language: "en"},
		Jurisdiction: j,
		Topic:        store.Topic{ID: 2, Slug: "heat-not-working", Name: "Heat Not Working"},
		Canonical:    "https://renterlaw.org/j/united-states/heat-not-working",
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return sb.String()
}

func TestNationwidePageAsksWhereYouRent(t *testing.T) {
	withPlaces(t)
	body := renderGuide(t, store.Jurisdiction{ID: 1, Kind: "country", Name: "United States", Slug: "united-states"})
	picker := strings.Index(body, `class="place-picker"`)
	main := strings.Index(body, `aria-label="Guide content"`)
	if picker < 0 || main < 0 || picker > main {
		t.Fatalf("expected the place picker before the statements; picker=%d main=%d", picker, main)
	}
	if !strings.Contains(body, "Where do you rent?") {
		t.Fatalf("picker heading missing")
	}
	// Two pickers on the page, the header's and the inline one, each with
	// its own id and both going to this topic in the picked place.
	if strings.Count(body, `action="/t/heat-not-working"`) != 2 {
		t.Fatalf("both pickers should submit to the topic hub:\n%s", body)
	}
	if !strings.Contains(body, `id="header-j"`) || !strings.Contains(body, `id="place-j"`) {
		t.Fatalf("each picker needs its own id")
	}
	if !strings.Contains(body, `<option value="boston">&nbsp;&nbsp;Boston</option>`) {
		t.Fatalf("cities sit indented under their state")
	}
	if strings.Contains(body, "place-switch") || strings.Contains(body, "place-suggest") {
		t.Fatalf("the old place line is gone")
	}
}

// A city or state guide names its place above the title and selects it in
// the header picker. Nothing else at the top of the page names the place.
func TestCityPagePickerShowsThePagesPlace(t *testing.T) {
	withPlaces(t)
	body := renderGuide(t, store.Jurisdiction{ID: 4, Kind: "city", Name: "Boston", Slug: "boston", ParentSlug: "massachusetts", ParentName: "Massachusetts"})
	if strings.Contains(body, `class="place-picker"`) {
		t.Fatalf("a city page has no place question to ask")
	}
	if !strings.Contains(body, `<option value="boston" selected>&nbsp;&nbsp;Boston</option>`) {
		t.Fatalf("the header picker should show this page's place:\n%s", body)
	}
	if !strings.Contains(body, `data-page-place="boston"`) {
		t.Fatalf("the script must know the picker shows the page's place, not the reader's")
	}
	if !strings.Contains(body, `action="/t/heat-not-working"`) {
		t.Fatalf("a pick should go to the same topic in the new place")
	}
	if strings.Contains(body, "place-switch") || strings.Contains(body, "For Boston") {
		t.Fatalf("the old place line is gone")
	}
	if strings.Contains(body, `class="breadcrumb"`) || strings.Contains(body, "sidebar") {
		t.Fatalf("the breadcrumb and sidebar are gone from guides")
	}
}

// Without a place list (a page rendered outside the handlers) the picker
// still shows the page's own place rather than an empty control.
func TestPickerWithoutPlaceListShowsThePagesPlace(t *testing.T) {
	prev := tmpl.PlaceList
	tmpl.PlaceList = nil
	t.Cleanup(func() { tmpl.PlaceList = prev })
	body := renderGuide(t, store.Jurisdiction{ID: 5, Kind: "state", Name: "Ohio", Slug: "ohio"})
	if !strings.Contains(body, `<option value="ohio" selected>Ohio</option>`) {
		t.Fatalf("the page's place should be the one option:\n%s", body)
	}
}
