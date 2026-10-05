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
	main := strings.Index(body, `aria-label="Guide content"`)
	if picker < 0 || main < 0 || picker > main {
		t.Fatalf("expected the place picker before the statements; picker=%d main=%d", picker, main)
	}
	if !strings.Contains(body, "Where do you rent?") {
		t.Fatalf("picker heading missing")
	}
	if !strings.Contains(body, `href="/j/massachusetts/boston/heat-not-working"`) {
		t.Fatalf("picker should link each place's own page for this topic")
	}
	if strings.Contains(body, `class="place-switch"`) || strings.Contains(body, `class="place-suggest"`) {
		t.Fatalf("a nationwide page routes with its picker, not the place line")
	}
}

// ADR-031 D1, D2: a city or state guide says whose rules it gives, near the
// top, with one way to switch place that keeps the topic. The old list of
// every other place at the foot of the page is gone.
func TestCityPageHasPlaceLineAtTheTop(t *testing.T) {
	var sb strings.Builder
	err := tmpl.Render(&sb, tmpl.PlaybookPage{
		Playbook:     store.Playbook{Title: "No Heat in Boston: What Can I Do?", Language: "en"},
		Jurisdiction: store.Jurisdiction{ID: 4, Kind: "city", Name: "Boston", Slug: "boston", ParentSlug: "massachusetts", ParentName: "Massachusetts"},
		Topic:        store.Topic{ID: 2, Slug: "heat-not-working", Name: "Heat Not Working"},
		Canonical:    "https://renterlaw.org/j/massachusetts/boston/heat-not-working",
		OtherCities: []store.Jurisdiction{
			{ID: 5, Kind: "state", Name: "Ohio", Slug: "ohio"},
		},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	body := sb.String()
	if strings.Contains(body, `class="place-picker"`) {
		t.Fatalf("a city page has no place question to ask")
	}
	if !strings.Contains(body, "For Boston, Massachusetts.") {
		t.Fatalf("the place line should name the city and its state")
	}
	sw := strings.Index(body, `class="place-switch"`)
	main := strings.Index(body, `aria-label="Guide content"`)
	if sw < 0 || main < 0 || sw > main {
		t.Fatalf("expected the place switch before the statements; switch=%d main=%d", sw, main)
	}
	if !strings.Contains(body, `href="/j/ohio/heat-not-working" data-set-location="ohio"`) {
		t.Fatalf("the switch should go to the same topic in the other place, and remember it")
	}
	if !strings.Contains(body, `data-topic="heat-not-working"`) || !strings.Contains(body, `data-state="massachusetts"`) {
		t.Fatalf("the suggestion line needs the topic and this page's state")
	}
	if strings.Contains(body, `id="related-topic-heading"`) {
		t.Fatalf("the foot-of-page places list is replaced by the place line")
	}
	if strings.Contains(body, `class="breadcrumb"`) || strings.Contains(body, "sidebar") {
		t.Fatalf("the breadcrumb and sidebar are gone from guides")
	}
}
