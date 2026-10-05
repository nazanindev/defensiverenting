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

// stateCodes maps each postal code to its state name, in the same order as
// usStates. Used to turn a visitor's region code into a state slug.
var stateCodes = map[string]string{
	"AL": "Alabama", "AK": "Alaska", "AZ": "Arizona", "AR": "Arkansas", "CA": "California",
	"CO": "Colorado", "CT": "Connecticut", "DE": "Delaware", "DC": "District of Columbia",
	"FL": "Florida", "GA": "Georgia", "HI": "Hawaii", "ID": "Idaho", "IL": "Illinois",
	"IN": "Indiana", "IA": "Iowa", "KS": "Kansas", "KY": "Kentucky", "LA": "Louisiana",
	"ME": "Maine", "MD": "Maryland", "MA": "Massachusetts", "MI": "Michigan", "MN": "Minnesota",
	"MS": "Mississippi", "MO": "Missouri", "MT": "Montana", "NE": "Nebraska", "NV": "Nevada",
	"NH": "New Hampshire", "NJ": "New Jersey", "NM": "New Mexico", "NY": "New York",
	"NC": "North Carolina", "ND": "North Dakota", "OH": "Ohio", "OK": "Oklahoma", "OR": "Oregon",
	"PA": "Pennsylvania", "RI": "Rhode Island", "SC": "South Carolina", "SD": "South Dakota",
	"TN": "Tennessee", "TX": "Texas", "UT": "Utah", "VT": "Vermont", "VA": "Virginia",
	"WA": "Washington", "WV": "West Virginia", "WI": "Wisconsin", "WY": "Wyoming",
}

// StateSlugForCode is the jurisdiction slug for a two-letter postal code, or
// "" for anything that is not a state or DC.
func StateSlugForCode(code string) string {
	if n, ok := stateCodes[strings.ToUpper(code)]; ok {
		return StateSlug(n)
	}
	return ""
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
