package templates_test

import (
	"strings"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
	tmpl "github.com/nazanindev/defensiverenting/web/templates"
)

// A nationwide page asks where the reader rents before the statements, and
// only there: the searcher who lands on it typed a problem, not a place, and
// the page's job is to route them to their state. A city page has no such
// question to ask and keeps the "elsewhere" list at the foot.
func renderWithPlaces(t *testing.T, kind string) string {
	t.Helper()
	var sb strings.Builder
	err := tmpl.Render(&sb, tmpl.PlaybookPage{
		Playbook:     store.Playbook{Title: "No Heat in Your Rental: What Can I Do?", Language: "en"},
		Jurisdiction: store.Jurisdiction{ID: 1, Kind: kind, Name: "United States", Slug: "united-states"},
		Topic:        store.Topic{ID: 2, Slug: "heat-not-working", Name: "Heat Not Working"},
		Canonical:    "https://renterlaw.org/j/united-states/heat-not-working",
		OtherCities: []store.Jurisdiction{
			{ID: 3, Kind: "state", Name: "Massachusetts", Slug: "massachusetts"},
			{ID: 4, Kind: "city", Name: "Boston", Slug: "boston", ParentSlug: "massachusetts"},
		},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return sb.String()
}

func TestNationwidePageAsksWhereYouRent(t *testing.T) {
	body := renderWithPlaces(t, "country")
	picker := strings.Index(body, `class="place-picker"`)
	main := strings.Index(body, `aria-label="Playbook content"`)
	if picker < 0 || main < 0 || picker > main {
		t.Fatalf("expected the place picker before the statements; picker=%d main=%d", picker, main)
	}
	if !strings.Contains(body, "Where do you rent?") {
		t.Fatalf("picker heading missing")
	}
	if !strings.Contains(body, `href="/j/massachusetts/boston/heat-not-working"`) {
		t.Fatalf("picker should link each place's own page for this topic")
	}
	if strings.Contains(body, `id="related-topic-heading"`) {
		t.Fatalf("the foot-of-page places list duplicates the picker on a nationwide page")
	}
}

func TestCityPageKeepsPlacesAtTheFoot(t *testing.T) {
	body := renderWithPlaces(t, "city")
	if strings.Contains(body, `class="place-picker"`) {
		t.Fatalf("a city page has no place question to ask")
	}
	if !strings.Contains(body, `id="related-topic-heading"`) {
		t.Fatalf("city page lost its elsewhere list")
	}
}
