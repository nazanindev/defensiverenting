package templates

import "strings"

// StateOption is one entry in the concept page's state picker (ADR-020 D3,
// D7). The picker lists every state, not only the covered ones, because the
// reader Google sends can rent anywhere; an uncovered pick still lands on a
// real page that says so.
type StateOption struct {
	Name     string
	Slug     string
	Selected bool
}

// usStates is every state plus the District of Columbia, in the slug form
// the jurisdictions table uses (lowercase, hyphenated name). A covered state
// carries the same slug there, so a pick resolves to its row when one exists.
var usStates = []string{
	"Alabama", "Alaska", "Arizona", "Arkansas", "California", "Colorado",
	"Connecticut", "Delaware", "District of Columbia", "Florida", "Georgia",
	"Hawaii", "Idaho", "Illinois", "Indiana", "Iowa", "Kansas", "Kentucky",
	"Louisiana", "Maine", "Maryland", "Massachusetts", "Michigan", "Minnesota",
	"Mississippi", "Missouri", "Montana", "Nebraska", "Nevada", "New Hampshire",
	"New Jersey", "New Mexico", "New York", "North Carolina", "North Dakota",
	"Ohio", "Oklahoma", "Oregon", "Pennsylvania", "Rhode Island",
	"South Carolina", "South Dakota", "Tennessee", "Texas", "Utah", "Vermont",
	"Virginia", "Washington", "West Virginia", "Wisconsin", "Wyoming",
}

// StateSlug is the jurisdiction slug for a state name.
func StateSlug(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, " ", "-"))
}

// StateOptions lists every state for the picker, marking selected.
func StateOptions(selected string) []StateOption {
	out := make([]StateOption, 0, len(usStates))
	for _, n := range usStates {
		s := StateSlug(n)
		out = append(out, StateOption{Name: n, Slug: s, Selected: s == selected})
	}
	return out
}

// StateName returns the display name for a state slug, or "" when the slug
// is not a state. It lets an uncovered state be named on the page without a
// row in the jurisdictions table.
func StateName(slug string) string {
	for _, n := range usStates {
		if StateSlug(n) == slug {
			return n
		}
	}
	return ""
}
