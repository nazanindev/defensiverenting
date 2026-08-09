package templates_test

import (
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
	tmpl "github.com/nazanindev/defensiverenting/web/templates"
)

func TestGroupByState_bucketsCitiesUnderTheirState(t *testing.T) {
	cities := []store.Jurisdiction{
		{Name: "Boston", Slug: "boston", ParentSlug: "massachusetts", ParentName: "Massachusetts"},
		{Name: "Cambridge", Slug: "cambridge", ParentSlug: "massachusetts", ParentName: "Massachusetts"},
		{Name: "Philadelphia", Slug: "philadelphia", ParentSlug: "pennsylvania", ParentName: "Pennsylvania"},
	}

	groups := tmpl.GroupByState(cities)

	if len(groups) != 2 {
		t.Fatalf("groups = %d, want 2", len(groups))
	}
	if groups[0].Name != "Massachusetts" || len(groups[0].Cities) != 2 {
		t.Errorf("group 0 = %q with %d cities, want Massachusetts with 2",
			groups[0].Name, len(groups[0].Cities))
	}
	if groups[1].Name != "Pennsylvania" || len(groups[1].Cities) != 1 {
		t.Errorf("group 1 = %q with %d cities, want Pennsylvania with 1",
			groups[1].Name, len(groups[1].Cities))
	}
	if got := groups[0].Path(); got != "/j/massachusetts" {
		t.Errorf("Path() = %q, want /j/massachusetts", got)
	}
}

// A city whose state is missing must still reach the page. Dropping it would
// hide a published guide because of a data problem one level up.
func TestGroupByState_keepsCitiesWithNoState(t *testing.T) {
	groups := tmpl.GroupByState([]store.Jurisdiction{
		{Name: "Boston", Slug: "boston", ParentSlug: "massachusetts", ParentName: "Massachusetts"},
		{Name: "Orphan", Slug: "orphan"},
	})

	if len(groups) != 2 {
		t.Fatalf("groups = %d, want 2", len(groups))
	}
	orphans := groups[1]
	if orphans.Name != "" {
		t.Errorf("stateless group name = %q, want empty", orphans.Name)
	}
	if orphans.Path() != "" {
		t.Errorf("stateless group Path() = %q, want empty so no link is emitted", orphans.Path())
	}
	if len(orphans.Cities) != 1 || orphans.Cities[0].Slug != "orphan" {
		t.Errorf("stateless group = %+v, want the orphan city", orphans.Cities)
	}
}

func TestGroupByState_empty(t *testing.T) {
	if groups := tmpl.GroupByState(nil); len(groups) != 0 {
		t.Errorf("groups = %d, want 0", len(groups))
	}
}
