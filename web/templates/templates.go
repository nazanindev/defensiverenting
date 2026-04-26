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

	"github.com/nazanin212/bostontenantsrights/internal/store"
)

//go:embed *.html
var tmplFS embed.FS

var tmpl *template.Template

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
	}
}

// Page types ----------------------------------------------------------------

// IndexPage is the landing page listing all city jurisdictions.
type IndexPage struct {
	Jurisdictions []store.Jurisdiction
}

// JurisdictionPage lists topics available for a city.
type JurisdictionPage struct {
	Jurisdiction store.Jurisdiction
	Topics       []store.Topic
}

// PlaybookPage is a single topic playbook with cited statements.
type PlaybookPage struct {
	Playbook     store.Playbook
	Jurisdiction store.Jurisdiction
	Topic        store.Topic
	IntroHTML    template.HTML
	Statements   []RenderedStatement
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
	SourceKind string // statute|regulation|gov_guidance|nonprofit|editorial
}

// SearchPage holds search results.
type SearchPage struct {
	Query            string
	JurisdictionSlug string
	Results          []store.SearchResult
}

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
	default:
		return fmt.Errorf("unknown page type %T", page)
	}
}
