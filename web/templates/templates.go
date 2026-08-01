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

// IndexPage is the landing page listing all city jurisdictions.
type IndexPage struct {
	Jurisdictions  []store.Jurisdiction
	StructuredData template.JS // JSON-LD WebSite + Organization schema, pre-marshaled
}

// JurisdictionPage lists topics available for a city.
type JurisdictionPage struct {
	Jurisdiction store.Jurisdiction
	Topics       []store.Topic
}

// PlaybookPage is a single topic playbook with cited statements.
type PlaybookPage struct {
	Playbook       store.Playbook
	Jurisdiction   store.Jurisdiction
	Topic          store.Topic
	IntroHTML      template.HTML
	Statements     []RenderedStatement
	Description    string      // meta description, derived from the intro when present
	Canonical      string      // absolute canonical URL
	StructuredData template.JS // JSON-LD Article + BreadcrumbList schema, pre-marshaled
	Preview        bool        // authoring-tool draft preview: shows a banner, never set on the live site
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
type SearchPage struct {
	Query            string
	JurisdictionSlug string
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
	default:
		return fmt.Errorf("unknown page type %T", page)
	}
}
