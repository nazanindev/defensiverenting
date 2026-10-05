package templates_test

import (
	"html/template"
	"strings"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
	tmpl "github.com/nazanindev/defensiverenting/web/templates"
)

// ADR-031 D6: a directory entry leads with the organization's name, offers
// the number in its own text as a call button and its source as the website
// button, and hides the repeated "Name, phone." lead under the heading.
func TestDirectoryEntryHierarchy(t *testing.T) {
	chip := []tmpl.CitationChip{{SourceID: 7, Label: "City of Chicago Department of Housing", URL: "https://chicago.gov/housing", SourceKind: "gov_guidance"}}
	page := tmpl.PlaybookPage{
		Playbook:     store.Playbook{ID: 1, Title: "Local Help in Chicago", Language: "en", PageKind: "directory"},
		Jurisdiction: store.Jurisdiction{Kind: "city", Name: "Chicago", Slug: "chicago", ParentSlug: "illinois", ParentName: "Illinois"},
		Topic:        store.Topic{Slug: tmpl.LocalHelpTopic, Name: "Local Help"},
		Statements: []tmpl.RenderedStatement{{
			BodyHTML:  template.HTML(`<p><strong>City of Chicago Department of Housing, <a href="tel:+13127443653" data-out="7">312-744-3653</a>.</strong> The city office works to protect the right to quality homes.</p>`),
			Citations: chip,
		}},
	}
	var sb strings.Builder
	if err := tmpl.Render(&sb, page); err != nil {
		t.Fatalf("render: %v", err)
	}
	body := sb.String()
	for _, want := range []string{
		`<h2 class="directory-name">City of Chicago Department of Housing</h2>`,
		`<p class="directory-kind">Government office</p>`,
		`href="tel:&#43;13127443653" data-out="7">Call 312-744-3653</a>`,
		`href="https://chicago.gov/housing" target="_blank"`,
		`<p>The city office works to protect the right to quality homes.</p>`,
		"Some of the groups below can help.",
		"For Chicago, Illinois.",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(body, "<strong>City of Chicago Department of Housing,") {
		t.Error("the repeated name lead should be hidden under the heading")
	}
	if strings.Contains(body, "Details checked") {
		t.Error("no per-entry checked line; the page date is in the fine print")
	}
}
