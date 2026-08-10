package templates_test

import (
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
	tmpl "github.com/nazanindev/defensiverenting/web/templates"
)

// The report links are built from template actions that reach across a range
// into the outer scope ($.Jurisdiction, $.Topic). That resolves at execution
// time, not at parse time, so nothing catches a wrong path until a Local Help
// page is served. These render the page and read the links back.
//
// Assertions are on the decoded query, not on the escaped href. html/template
// percent-encodes values in query context, and pinning that spelling would
// make the test fail on a correct page.

var hrefRe = regexp.MustCompile(`href="(/report[^"]*)"`)

func renderPlaybook(t *testing.T, pageKind string) string {
	t.Helper()

	chip := tmpl.CitationChip{
		URL:        "https://clvu.org",
		Label:      "City Life Vida Urbana",
		SourceKind: "nonprofit",
	}
	var sb strings.Builder
	err := tmpl.Render(&sb, tmpl.PlaybookPage{
		Playbook:     store.Playbook{Title: "Local Help in Boston", PageKind: pageKind},
		Jurisdiction: store.Jurisdiction{ID: 1, Kind: "city", Name: "Boston", Slug: "boston"},
		Topic:        store.Topic{ID: 2, Slug: "resource-directory", Name: "Local Help"},
		Canonical:    "https://renterlaw.org/j/boston/resource-directory",
		Statements: []tmpl.RenderedStatement{
			{BodyHTML: "Call 617-934-5006 for eviction help.", Citations: []tmpl.CitationChip{chip}},
			{BodyHTML: "Walk-in hours are Tuesday and Thursday.", Citations: []tmpl.CitationChip{chip}},
		},
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return sb.String()
}

// reportLinks returns the decoded query of every /report link on the page.
func reportLinks(t *testing.T, body string) []url.Values {
	t.Helper()

	var out []url.Values
	for _, m := range hrefRe.FindAllStringSubmatch(strings.ReplaceAll(body, "&amp;", "&"), -1) {
		u, err := url.Parse(m[1])
		if err != nil {
			t.Fatalf("unparseable report link %q: %v", m[1], err)
		}
		out = append(out, u.Query())
	}
	return out
}

// A directory carries one report link, at the foot of the page, not one under
// every organization.
//
// The per-entry link was well meant — an organization that has closed is the
// report we most want — but repeated under each entry it read as part of the
// entry rather than as a page action, and on a directory trimmed to the two or
// three organizations worth calling it was three near-identical prompts. The
// reader says which organization in the form.
func TestDirectoryPage_hasNoPerOrganisationReportLink(t *testing.T) {
	for _, q := range reportLinks(t, renderPlaybook(t, "directory")) {
		if org := q.Get("org"); org != "" {
			t.Errorf("directory entries must not carry their own report link, found one for %q", org)
		}
	}
}

func TestPlaybookPage_reportLinkCarriesThePage(t *testing.T) {
	for _, kind := range []string{"playbook", "directory", "faq", "checklist"} {
		t.Run(kind, func(t *testing.T) {
			var ok bool
			for _, q := range reportLinks(t, renderPlaybook(t, kind)) {
				if q.Get("org") == "" && q.Get("url") == "/j/boston/resource-directory" {
					ok = true
				}
			}
			if !ok {
				t.Error("page has no report link carrying its own path")
			}
		})
	}
}

// The footer reaches both forms from every page, which is the only route in
// for someone who is not looking at the page they want to report.
func TestFooter_linksToBothForms(t *testing.T) {
	body := renderPlaybook(t, "playbook")
	for _, href := range []string{`href="/report"`, `href="/contact"`} {
		if !strings.Contains(body, href) {
			t.Errorf("footer missing %s", href)
		}
	}
}
