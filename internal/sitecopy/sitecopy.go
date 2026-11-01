// Package sitecopy holds the site's prose pages (about, editorial standards,
// privacy, support) as markdown, renders them, and checks an edit before it
// is saved.
//
// Each page ships with built-in text (defaults/*.md). A row in site_pages
// overrides it; that row is written from the authoring app's copy screen, so
// the editor can fix site copy without a code change. Check runs the same
// voice rules the guide pages are held to, plus a spelling pass, and a save
// that fails either is refused with the reasons listed. The rules live in
// code so they hold no matter who is typing.
package sitecopy

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"regexp"
	"sort"
	"strings"

	"github.com/nazanindev/defensiverenting/internal/voice"
	"github.com/yuin/goldmark"
)

//go:embed defaults/*.md
var defaultsFS embed.FS

// Page is one prose page: the fields the editor can change.
type Page struct {
	Slug        string
	Title       string
	Description string // the meta description, shown in search results
	Body        string // markdown, without the h1
}

// Info is what the code knows about a page that the editor does not edit.
type Info struct {
	Slug  string
	Path  string
	Crumb string // the breadcrumb label
}

// pages lists every editable page in the order the copy screen shows them.
var pages = []Info{
	{Slug: "about", Path: "/about", Crumb: "About"},
	{Slug: "editorial", Path: "/editorial", Crumb: "Editorial standards"},
	{Slug: "privacy", Path: "/privacy", Crumb: "Privacy"},
	{Slug: "support", Path: "/support", Crumb: "Support"},
}

// Pages returns the editable pages.
func Pages() []Info { return append([]Info(nil), pages...) }

// Lookup returns the info for a slug.
func Lookup(slug string) (Info, bool) {
	for _, p := range pages {
		if p.Slug == slug {
			return p, true
		}
	}
	return Info{}, false
}

// Default returns the built-in text for a slug. It panics on an unknown slug
// or a malformed file, both of which are programming errors a test catches.
func Default(slug string) Page {
	raw, err := defaultsFS.ReadFile("defaults/" + slug + ".md")
	if err != nil {
		panic(fmt.Sprintf("sitecopy: no default for %q: %v", slug, err))
	}
	head, body, ok := strings.Cut(string(raw), "\n---\n")
	if !ok {
		panic(fmt.Sprintf("sitecopy: %s.md has no front matter", slug))
	}
	p := Page{Slug: slug, Body: strings.TrimSpace(body)}
	for _, line := range strings.Split(head, "\n") {
		k, v, _ := strings.Cut(line, ":")
		switch strings.TrimSpace(k) {
		case "title":
			p.Title = strings.TrimSpace(v)
		case "description":
			p.Description = strings.TrimSpace(v)
		}
	}
	return p
}

// md renders with goldmark's defaults: raw HTML in the markdown is escaped,
// so the copy screen cannot put script on a public page.
var md = goldmark.New()

// Render converts the body to HTML.
func Render(body string) (template.HTML, error) {
	var buf bytes.Buffer
	if err := md.Convert([]byte(body), &buf); err != nil {
		return "", err
	}
	//nolint:gosec // goldmark escapes raw HTML; the output is ours to trust.
	return template.HTML(buf.String()), nil
}

var (
	htmlTag  = regexp.MustCompile(`<[^>]+>`)
	entities = strings.NewReplacer("&amp;", "&", "&quot;", "\"", "&#39;", "'", "&lt;", "<", "&gt;", ">", "&nbsp;", " ")
)

// blocks splits the rendered page into the pieces a reader reads, labeled
// by kind and position so a failure points at the text.
func blocks(p Page) (map[string]string, error) {
	html, err := Render(p.Body)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	if t := strings.TrimSpace(p.Title); t != "" {
		out["title"] = t
	}
	if d := strings.TrimSpace(p.Description); d != "" {
		out["description"] = d
	}
	for _, tag := range []string{"p", "li", "h2", "h3"} {
		re := regexp.MustCompile(`(?is)<` + tag + `(?:\s[^>]*)?>(.*?)</` + tag + `>`)
		for i, m := range re.FindAllStringSubmatch(string(html), -1) {
			text := strings.Join(strings.Fields(entities.Replace(htmlTag.ReplaceAllString(m[1], " "))), " ")
			if text == "" {
				continue
			}
			short := text
			if r := []rune(short); len(r) > 50 {
				short = string(r[:50]) + "…"
			}
			out[fmt.Sprintf("%s %d %q", kindName(tag), i+1, short)] = text
		}
	}
	return out, nil
}

func kindName(tag string) string {
	switch tag {
	case "p":
		return "paragraph"
	case "li":
		return "list item"
	default:
		return "heading"
	}
}

// Check returns every reason the page cannot be saved: an empty field, a
// voice rule broken, or a word the dictionary does not know. Nil means save.
func Check(p Page) []string {
	var out []string
	if strings.TrimSpace(p.Title) == "" {
		out = append(out, "The title is empty.")
	}
	if strings.TrimSpace(p.Description) == "" {
		out = append(out, "The description is empty. It is what people see in search results.")
	}
	if strings.TrimSpace(p.Body) == "" {
		out = append(out, "The page text is empty.")
	}
	if strings.Contains(p.Body, "# ") && strings.HasPrefix(strings.TrimSpace(p.Body), "# ") {
		out = append(out, "The text starts with a # heading. The title field is the page heading; use ## for sections.")
	}
	if len(out) > 0 {
		return out
	}
	bl, err := blocks(p)
	if err != nil {
		return []string{"The text could not be read as markdown: " + err.Error()}
	}
	out = append(out, voice.LintAll("en", bl)...)

	keys := make([]string, 0, len(bl))
	for k := range bl {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if bad := Misspelled(bl[k]); len(bad) > 0 {
			out = append(out, fmt.Sprintf("%s: unknown word %s. Fix the spelling, or ask for the word to be added to the site's word list.", k, quoteAll(bad)))
		}
	}
	return out
}

func quoteAll(ws []string) string {
	q := make([]string, len(ws))
	for i, w := range ws {
		q[i] = fmt.Sprintf("%q", w)
	}
	return strings.Join(q, ", ")
}
