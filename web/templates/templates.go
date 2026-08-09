// Package templates renders HTML pages using html/template.
// Each page type is a distinct Go struct, enforcing compile-time type safety
// over the data passed to templates. The citation invariant (every RenderedStatement
// must have non-empty Citations) is enforced by the browse handler before reaching here.
package templates

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"strings"

	"github.com/nazanindev/defensiverenting/internal/store"
)

//go:embed *.html
var tmplFS embed.FS

var tmpl *template.Template

// baseURL is the canonical site origin (no trailing slash) used for absolute
// canonical/og:url links and JSON-LD. Overridden at startup via SetBaseURL.
var baseURL = "https://renterlaw.org"

// SetBaseURL sets the canonical site origin, e.g. from the SITE_URL env var.
func SetBaseURL(u string) {
	baseURL = strings.TrimRight(u, "/")
}

// BaseURL returns the canonical site origin with no trailing slash.
func BaseURL() string { return baseURL }

func init() {
	var err error
	tmpl, err = template.New("").Funcs(funcMap()).ParseFS(tmplFS, "*.html")
	if err != nil {
		panic(fmt.Sprintf("parse templates: %v", err))
	}
}

func funcMap() template.FuncMap {
	return template.FuncMap{
		"chipClass": func(kind string) string {
			switch kind {
			case "statute":
				return "chip chip--statute"
			case "regulation":
				return "chip chip--regulation"
			case "editorial":
				return "chip chip--editorial"
			case "court_ruling":
				return "chip chip--court-ruling"
			default:
				return "chip chip--gov"
			}
		},
		"chipLabel": func(label, locator string) string {
			if locator != "" {
				return label + " " + locator
			}
			return label
		},
		"absURL": func(path string) string {
			return baseURL + path
		},
	}
}

// Page types ----------------------------------------------------------------

// StateGroup is one state with the covered cities beneath it.
//
// Every city list on the site is grouped through this type. Listing cities flat
// meant the page grew one box per city forever; grouped, it grows one heading
// per state, and a state count is bounded at 51 no matter how many cities ship.
type StateGroup struct {
	Name   string // display name, e.g. "Massachusetts"
	Slug   string // URL slug, e.g. "massachusetts"
	Cities []store.Jurisdiction
}

// Path is the state hub URL, or empty when the city has no parent state.
func (g StateGroup) Path() string {
	if g.Slug == "" {
		return ""
	}
	return "/j/" + g.Slug
}

// GroupByState buckets cities under their parent state, preserving the order
// the store returned them in (state name, then city name).
//
// A city with no parent lands in a trailing group with an empty name. The
// templates render that group without a heading rather than dropping the city,
// so a data problem costs a heading instead of hiding a page.
func GroupByState(cities []store.Jurisdiction) []StateGroup {
	var out []StateGroup
	idx := make(map[string]int, len(cities))
	for _, c := range cities {
		i, ok := idx[c.ParentSlug]
		if !ok {
			i = len(out)
			idx[c.ParentSlug] = i
			out = append(out, StateGroup{Name: c.ParentName, Slug: c.ParentSlug})
		}
		out[i].Cities = append(out[i].Cities, c)
	}
	return out
}

// IndexPage is the landing page: search, situations, and a location scope.
//
// It deliberately does not carry a flat city list. Cities reach the page only
// through LocationGroups, which feeds the scope picker; the full crawlable
// directory lives at /locations.
type IndexPage struct {
	LocationGroups []StateGroup
	CityCount      int
	Topics         []store.Topic // topics with >=1 published playbook, shown as situations
	StructuredData template.JS   // JSON-LD WebSite + Organization schema, pre-marshaled
}

// ScopeSearch is the model for the shared search-with-location control.
//
// Location is a scope on the search, not a browse axis: it rides along as the
// `j` query param the search handler already understood, so choosing a city
// never needs the page to list every city as a link.
type ScopeSearch struct {
	Query     string
	Selected  string // selected city slug; empty means every location
	Groups    []StateGroup
	Autofocus bool
}

// Scope renders the homepage's search control, unscoped and focused.
func (p IndexPage) Scope() ScopeSearch {
	return ScopeSearch{Groups: p.LocationGroups, Autofocus: true}
}

// Scope renders the results page's search control, preserving the query and the
// location the search ran under.
func (p SearchPage) Scope() ScopeSearch {
	return ScopeSearch{Query: p.Query, Selected: p.JurisdictionSlug, Groups: p.LocationGroups}
}

// LocationsPage is the crawlable directory of every covered city, grouped by
// state. It exists so the homepage does not have to enumerate cities to keep
// them linked for search engines.
type LocationsPage struct {
	Groups    []StateGroup
	CityCount int
}

// TopicHubPage lists every place that has a published playbook for one topic.
//
// Statewide is kept apart from Groups because a state-level guide is not a
// city: grouped by parent it would file itself under a "United States" heading
// and read as if the country were a city you could pick.
type TopicHubPage struct {
	Topic     store.Topic
	Groups    []StateGroup
	Statewide []store.Jurisdiction
	CityCount int
}

// AuthorPage is a reviewer bio page.
type AuthorPage struct{}

// JurisdictionPage lists topics available for a city, state, or country.
//
// Cities is populated only for a parent jurisdiction (a state), listing the
// covered cities beneath it. Without it a state hub rendered as a dead end:
// /j/massachusetts said "no playbooks yet" and never linked to Boston.
type JurisdictionPage struct {
	Jurisdiction store.Jurisdiction
	Topics       []store.Topic
	Cities       []store.Jurisdiction
}

// PlaybookPage is a single topic playbook with cited statements.
type PlaybookPage struct {
	Playbook       store.Playbook
	Jurisdiction   store.Jurisdiction
	Topic          store.Topic
	IntroHTML      template.HTML
	Statements     []RenderedStatement
	Description    string               // meta description, derived from the intro when present
	Canonical      string               // absolute canonical URL
	StructuredData template.JS          // JSON-LD Article + BreadcrumbList schema, pre-marshaled
	Preview        bool                 // authoring-tool draft preview: shows a banner, never set on the live site
	ReviewedOn     string               // human-readable last-verified date for the byline; empty hides the date
	SiblingTopics  []store.Topic        // other published topics in this city
	OtherCities    []store.Jurisdiction // other cities with this topic published
}

// RenderedStatement is a statement whose body has been converted to HTML.
// Citations is always non-empty; the handler guarantees this before constructing the value.
type RenderedStatement struct {
	BodyHTML  template.HTML
	Citations []CitationChip
}

// CitationChip is a rendered citation link shown inline after each statement.
type CitationChip struct {
	URL        string
	Label      string
	Locator    string
	SourceKind string // statute|regulation|gov_guidance|nonprofit|editorial|court_ruling
}

// SearchPage holds search results.
//
// LocationGroups feeds the same scope picker the homepage uses, so the location
// a search was run under can be changed from the results rather than only from
// the page the search started on.
type SearchPage struct {
	Query            string
	JurisdictionSlug string
	JurisdictionName string
	LocationGroups   []StateGroup
	Results          []store.SearchResult
}

// EditorialPage is the /editorial explainer page.
type EditorialPage struct{}

// AboutPage is the /about page.
type AboutPage struct{}

// SupportPage is the /support page.
type SupportPage struct{}

// Render dispatches to the correct template based on the concrete page type.
func Render(w io.Writer, page any) error {
	switch p := page.(type) {
	case IndexPage:
		return tmpl.ExecuteTemplate(w, "index.html", p)
	case JurisdictionPage:
		return tmpl.ExecuteTemplate(w, "jurisdiction.html", p)
	case PlaybookPage:
		return tmpl.ExecuteTemplate(w, "playbook.html", p)
	case SearchPage:
		return tmpl.ExecuteTemplate(w, "search.html", p)
	case EditorialPage:
		return tmpl.ExecuteTemplate(w, "editorial.html", p)
	case AboutPage:
		return tmpl.ExecuteTemplate(w, "about.html", p)
	case SupportPage:
		return tmpl.ExecuteTemplate(w, "support.html", p)
	case TopicHubPage:
		return tmpl.ExecuteTemplate(w, "topichub.html", p)
	case LocationsPage:
		return tmpl.ExecuteTemplate(w, "locations.html", p)
	case AuthorPage:
		return tmpl.ExecuteTemplate(w, "author.html", p)
	default:
		return fmt.Errorf("unknown page type %T", page)
	}
}
