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
	"sort"
	"strings"
	"time"

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
		"chipClass": ChipClass,
		"chipLabel": func(label, locator string) string {
			if locator != "" {
				return label + " " + locator
			}
			return label
		},
		"absURL": func(path string) string {
			return baseURL + path
		},
		"groupByOrg": groupByOrg,
		"langPrefix": store.LangPrefix,
	}
}

// SourceKinds is every value the sources.kind column permits, mirroring the
// CHECK constraint in migration 000006. ChipClass must return a distinct class
// for each one: the chip is the only thing telling a reader what kind of
// authority backs a statement (ADR-003, render layer), so a kind that falls
// through to another kind's styling misrepresents that authority. A test in
// this package fails if any kind here loses its own class.
var SourceKinds = []string{
	"statute", "regulation", "gov_guidance", "nonprofit", "editorial", "court_ruling",
}

// ChipClass maps a source kind to its citation-chip classes.
//
// Every kind is listed explicitly and the default is deliberately its own
// neutral class rather than an alias for a real one. Until 2026-08-09 the
// default returned chip--gov, so nonprofit and gov_guidance rendered
// identically: every legal-aid organisation on the site displayed with the
// green government chip and its 🏛 glyph. A source we have not styled must
// claim no authority it does not have.
func ChipClass(kind string) string {
	switch kind {
	case "statute":
		return "chip chip--statute"
	case "regulation":
		return "chip chip--regulation"
	case "gov_guidance":
		return "chip chip--gov"
	case "nonprofit":
		return "chip chip--nonprofit"
	case "editorial":
		return "chip chip--editorial"
	case "court_ruling":
		return "chip chip--court-ruling"
	default:
		return "chip chip--other"
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
	// Selectable marks a state with published statewide pages of its own, so
	// the scope picker offers it as an "All of {state}" choice. A state whose
	// only coverage is its cities stays an unselectable optgroup label:
	// scoping there would search state law only and silently exclude the
	// renter's city.
	Selectable bool
}

// Path is the state hub URL, or empty when the city has no parent state.
func (g StateGroup) Path() string {
	if g.Slug == "" {
		return ""
	}
	return "/j/" + g.Slug
}

// PathIn is Path() prefixed for lang, for a bilingual page (e.g. topichub.html)
// linking a state heading. Templates still check plain Path() for truthiness —
// it and PathIn are empty under exactly the same condition — so the "Other"
// fallback for a parentless city is unaffected by which one builds the href.
func (g StateGroup) PathIn(lang string) string {
	if g.Slug == "" {
		return ""
	}
	return store.LangPrefix(lang) + "/j/" + g.Slug
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

// MarkStatewide flags the groups whose state has published statewide pages of
// its own and appends a group for any such state with no covered cities yet,
// which would otherwise never reach the picker at all. Groups come back in
// name order (the order GroupByState preserves from the store), with the
// no-state group, when present, first.
func MarkStatewide(groups []StateGroup, states []store.Jurisdiction) []StateGroup {
	idx := make(map[string]int, len(groups))
	for i, g := range groups {
		idx[g.Slug] = i
	}
	for _, s := range states {
		if i, ok := idx[s.Slug]; ok {
			groups[i].Selectable = true
			continue
		}
		groups = append(groups, StateGroup{Name: s.Name, Slug: s.Slug, Selectable: true})
	}
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	return groups
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
	// Terms is the homepage's reference section (ADR-012 D3): concepts the
	// national pages define, so the list grows with editorial output.
	Terms          []store.Term
	StructuredData template.JS // JSON-LD WebSite + Organization schema, pre-marshaled
}

// TermsPage is /terms: the crawlable index of the reference layer (ADR-012 D2).
type TermsPage struct {
	Terms []store.Term
}

// ConceptPage is /c/{slug}: one legal term defined by the national statement
// and answered by every covered place, assembled from published verified
// statements (ADR-012 D1).
type ConceptPage struct {
	Slug      string
	Name      string
	TopicSlug string
	// Definition is the registry's plain-language gloss, shown as the lede.
	Definition  string
	Description string
	// National holds every national statement carrying the concept — tags may
	// be shared, and a second national statement is more of the general rule,
	// never a "where you live" entry.
	National []ConceptEntry
	Local    []ConceptEntry
}

// ConceptEntry is one place's statement on a concept page, carrying the same
// chips and trust line the statement shows on its own page, plus the link
// back to it at its anchor.
type ConceptEntry struct {
	PlaceName string
	PlaceKind string // country|state|city
	PlaceSlug string // jurisdiction slug, for the client-side place filter
	PagePath  string // the statement's home page, at its concept anchor
	BodyHTML  template.HTML
	Citations []CitationChip
	CheckedOn string
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
//
// National carries country-level hubs with published guides of their own,
// listed in a section of their own: bucketed like a city, "United States"
// would render under an empty state heading and read as a city you could pick.
type LocationsPage struct {
	National  []store.Jurisdiction
	Groups    []StateGroup
	CityCount int
}

// TopicHubPage lists every place that has a published playbook for one topic.
//
// Statewide and National are kept apart from Groups because neither is a
// city: grouped by parent, a state guide would file itself under a "United
// States" heading and read as if the country were a city you could pick, and
// a national guide would land in the statewide bucket labelled "Statewide".
type TopicHubPage struct {
	Topic     store.Topic
	Groups    []StateGroup
	Statewide []store.Jurisdiction
	National  []store.Jurisdiction
	CityCount int
	// Language is the language this hub's cities/statewide guides are
	// filtered to (see ADR-007 D5) — "en" or "es". Drives <html lang> and the
	// language-prefixed links this page renders.
	Language string
}

// AuthorsPage is the authoring team page: the editor and the reviewing
// editor, one page for both.
type AuthorsPage struct{}

// JurisdictionPage lists topics available for a city, state, or country.
//
// Cities is populated only for a parent jurisdiction (a state), listing the
// covered cities beneath it. Without it a state hub rendered as a dead end:
// /j/massachusetts said "no playbooks yet" and never linked to Boston.
type JurisdictionPage struct {
	Jurisdiction store.Jurisdiction
	Topics       []store.Topic
	Cities       []store.Jurisdiction
	// Language is the language Topics is filtered to (see ADR-007 D5) — "en"
	// or "es". Drives <html lang> and the language-prefixed links this page
	// renders. Cities is not language-scoped: a jurisdiction hub lists its
	// child cities regardless of what language they have content in yet.
	Language string
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
	ReviewedByName string               // byline name of who last saved or published the page; falls back to the historical reviewer
	SiblingTopics  []store.Topic        // other published topics in this city
	OtherCities    []store.Jurisdiction // other cities with this topic published
	// LocalHelpPath links to this city's Local Help page when one is published,
	// and is empty otherwise. It is shown near the top rather than among the
	// sibling topics because a reader who needs a phone number needs it before
	// they read the law, not after.
	LocalHelpPath string
	LocalHelpName string // the topic's display name, e.g. "Local Help"

	// Language-alternate fields (ADR-007 D6). All empty unless a translation
	// of this exact page actually exists, so a toggle link or hreflang tag is
	// never rendered pointing at a URL that would 404 — see ResolveLanguage
	// and voice.Supported for the languages this project trusts at all.
	OtherLangPath  string // relative path to the other language's version, or ""
	OtherLangCode  string // e.g. "es"; "" iff OtherLangPath is ""
	OtherLangLabel string // toggle-link text, written IN the other language
	// XDefaultPath is the absolute URL for hreflang="x-default": the English
	// version when one exists, this page's own Canonical otherwise. Always set.
	XDefaultPath string
}

// LocalHelpTopic is the slug of the topic whose pages list local organisations
// rather than explain law. The slug is historical; its display name is "Local
// Help". It is a topic, not to be confused with the "directory" page_kind,
// which is a layout any topic may use.
const LocalHelpTopic = "resource-directory"

// RenderedStatement is a statement whose body has been converted to HTML.
// Citations is always non-empty; the handler guarantees this before constructing the value.
type RenderedStatement struct {
	BodyHTML template.HTML
	// Anchor is the statement's concept slug (ADR-011), rendered as the
	// element id so other pages can deep-link to this exact claim. "" for
	// untagged statements: no id, no links.
	Anchor string
	// SpecificsPath is the topic hub the statement points a reader at for
	// their own place's rules, instead of a bare "check your state". Set only
	// on national pages' tagged statements (ADR-011 D4, amended: one hub link
	// listing every covered jurisdiction, not a per-place link row).
	SpecificsPath string
	// TopicRefPath/Name render the whole-topic line (ADR-011 D7): this
	// statement is a one-paragraph summary of a subject the site covers as
	// its own pages, and the link goes to that topic's hub. Set only on
	// national pages' topic-referencing statements.
	TopicRefPath string
	TopicRefName string
	// CheckedOn is the renter-facing trust line: the date this statement's
	// quoted sources were all last confirmed live (the oldest confirmation,
	// since the statement is only as current as its stalest evidence). Empty
	// when any checkable citation lacks a confirmation — the line is an
	// attestation, so it renders fully earned or not at all. CheckedAt is the
	// same instant as a time, for grouped layouts to compare.
	CheckedOn string
	CheckedAt *time.Time
	Citations []CitationChip
}

// CitationChip is a rendered citation link shown inline after each statement.
type CitationChip struct {
	URL        string
	Label      string
	Locator    string
	SourceKind string // statute|regulation|gov_guidance|nonprofit|editorial|court_ruling
}

// OrgGroup is a run of consecutive directory statements that all cite the same
// source, presented as one entry.
//
// A directory page lists organisations, and one organisation usually needs
// several statements: the phone number, the hours, who qualifies, what the
// organisation will not do. Rendered one statement per card, a single
// organisation became four cards carrying four identical chips, and a reader
// scanning for a phone number had to work out which cards belonged together.
//
// Grouping is by source rather than by any stored field because on a directory
// page the source IS the organisation: each entry cites that organisation's own
// site (see the drafting rules for resource-directory). Runs are consecutive,
// so a page that returns to an organisation later still gets a second entry
// rather than silently reordering what the author wrote.
type OrgGroup struct {
	Chip       CitationChip
	Statements []RenderedStatement
}

// Anchor is the group's element id: the first tagged statement's concept slug.
// A directory entry is one organisation rendered as one card, so the card gets
// one anchor even when several of its statements are tagged.
func (g OrgGroup) Anchor() string {
	for _, s := range g.Statements {
		if s.Anchor != "" {
			return s.Anchor
		}
	}
	return ""
}

// CheckedOn is the group's trust line: the oldest source confirmation across
// the organisation's statements, and only when every statement has one — the
// same fully-earned-or-absent rule the per-statement line keeps.
func (g OrgGroup) CheckedOn() string {
	var oldest *time.Time
	for _, s := range g.Statements {
		if s.CheckedAt == nil {
			return ""
		}
		if oldest == nil || s.CheckedAt.Before(*oldest) {
			oldest = s.CheckedAt
		}
	}
	if oldest == nil {
		return ""
	}
	return oldest.Format("January 2, 2006")
}

// groupByOrg collapses consecutive statements sharing a first citation into one
// OrgGroup. Statements are guaranteed to have at least one citation, so the
// first is always present.
//
// The group chip drops the per-statement locator: it spans several statements
// whose locators differ, and on a directory page a locator names a section of
// the organisation's site rather than a provision being relied on. The locator
// is still stored on every citation, so nothing the source checker uses is lost.
func groupByOrg(stmts []RenderedStatement) []OrgGroup {
	var groups []OrgGroup
	lastKey := ""
	for _, s := range stmts {
		if len(s.Citations) == 0 {
			continue
		}
		key := sourceKey(s.Citations[0].URL)
		if n := len(groups); n > 0 && key == lastKey {
			groups[n-1].Statements = append(groups[n-1].Statements, s)
			continue
		}
		lastKey = key
		chip := s.Citations[0]
		chip.Locator = ""
		chip.URL = key // the group links to the source, not to one statement's anchor
		groups = append(groups, OrgGroup{Chip: chip, Statements: []RenderedStatement{s}})
	}
	return groups
}

// sourceKey strips the "#locator" anchor a chip URL carries so two citations of
// the same document compare equal.
//
// The chip URL is built as SourceURL + anchorFragment(Locator), which points a
// reader at the right part of a long statute. It also means every statement
// citing one source has a different chip URL, so grouping on it grouped
// nothing: a directory of three organisations rendered as seven entries.
func sourceKey(chipURL string) string {
	if i := strings.IndexByte(chipURL, '#'); i >= 0 {
		return chipURL[:i]
	}
	return chipURL
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

// ReportPage is the /report form, where a reader tells us something on the
// site is wrong or an organisation has closed.
//
// The form posts to a Cloudflare Worker rather than to this server, so nothing
// here handles a submission. These fields only build the form: where to send
// it, and what to prefill.
type ReportPage struct {
	FormsURL         string
	TurnstileSiteKey string // empty hides the widget, e.g. in development
	// ErrorMessage is renter-facing text, already resolved from the ?error=
	// code the Worker redirects back with. Empty on a first visit.
	ErrorMessage string
	// PageURL is the site path being reported, carried in from the "Report a
	// problem" link at the foot of a playbook. Empty when the reader arrived
	// from the footer instead, and the form then asks which page they mean.
	PageURL string
	// OrgName prefills the organisation field when the report started from a
	// Local Help entry.
	OrgName string
}

// ContactPage is the /contact form for everything that is not a correction.
type ContactPage struct {
	FormsURL         string
	TurnstileSiteKey string
	ErrorMessage     string
}

// NotFoundPage is the styled 404. Every variant carries the same explanation:
// a person researches and checks every guide before it is published, so
// coverage grows one place and one topic at a time. Which fields are set
// decides the rest of the story:
//
//   - all zero: a plain wrong URL ("we can't find that page")
//   - UncoveredPlace: the URL named a place we have not covered, optionally
//     under a known Parent hub the page can link ("everything we have for
//     Massachusetts")
//   - Place + Topic: a covered place missing this one guide; NearestPath and
//     NearestName link the closest guide up the ancestor chain when one
//     exists, since an ancestor's law applies in the place beneath it
//   - Topic alone: a registry topic with no published guides in this language
//
// The page is always served with a 404 status; the body is helpful, the code
// stays honest so search engines drop the URL.
type NotFoundPage struct {
	Place          store.Jurisdiction
	Topic          store.Topic
	NearestPath    string
	NearestName    string
	UncoveredPlace bool
	Parent         store.Jurisdiction
	// Language is the language of the URL that missed ("en" or "es"), used
	// only to build links back into that language's pages. The chrome stays
	// English like the rest of the non-content pages (ADR-007 D2).
	Language string
}

// ThanksPage confirms a submission. Kind is "report" or "contact" and changes
// only the wording.
type ThanksPage struct {
	Kind string
}

// Reported reports whether the thank-you page is confirming a correction.
func (p ThanksPage) Reported() bool { return p.Kind == "report" }

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
	case TermsPage:
		return tmpl.ExecuteTemplate(w, "terms.html", p)
	case ConceptPage:
		return tmpl.ExecuteTemplate(w, "concept.html", p)
	case EditorialPage:
		return tmpl.ExecuteTemplate(w, "editorial.html", p)
	case AboutPage:
		return tmpl.ExecuteTemplate(w, "about.html", p)
	case SupportPage:
		return tmpl.ExecuteTemplate(w, "support.html", p)
	case ReportPage:
		return tmpl.ExecuteTemplate(w, "report.html", p)
	case ContactPage:
		return tmpl.ExecuteTemplate(w, "contact.html", p)
	case ThanksPage:
		return tmpl.ExecuteTemplate(w, "thanks.html", p)
	case TopicHubPage:
		return tmpl.ExecuteTemplate(w, "topichub.html", p)
	case LocationsPage:
		return tmpl.ExecuteTemplate(w, "locations.html", p)
	case AuthorsPage:
		return tmpl.ExecuteTemplate(w, "authors.html", p)
	case NotFoundPage:
		return tmpl.ExecuteTemplate(w, "notfound.html", p)
	default:
		return fmt.Errorf("unknown page type %T", page)
	}
}
