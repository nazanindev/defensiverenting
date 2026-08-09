package main

import (
	"bytes"
	"html/template"
	"io"
	"strings"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// parseTemplates mirrors the parsing done in main() so tests catch template
// syntax errors and undefined functions/fields without needing a database.
func parseTemplates(t *testing.T) *template.Template {
	t.Helper()
	tmpl, err := template.New("").Funcs(template.FuncMap{"date": fmtDate}).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		t.Fatalf("parse templates: %v", err)
	}
	return tmpl
}

func TestDashboardTemplateRenders(t *testing.T) {
	tmpl := parseTemplates(t)
	data := map[string]any{
		"Playbooks": []store.AuthorPlaybookRow{
			{ID: 1, Title: "Heat Not Working", JurisdictionName: "Chicago", Status: "draft", PageKind: "playbook", Language: "en"},
		},
		"Cities": []store.Jurisdiction{{Name: "Chicago", Slug: "chicago"}},
		"ReviewCounts": []store.CandidateCountRow{
			{JurisdictionName: "Chicago", JurisdictionSlug: "chicago", PendingCount: 7},
		},
		"Status": "all", "TotalCount": 1, "DraftCount": 1, "PublishedCount": 0,
	}
	if err := tmpl.ExecuteTemplate(io.Discard, "dashboard.html", data); err != nil {
		t.Fatalf("execute dashboard.html: %v", err)
	}

	// Empty state differs per tab, so each one has to render.
	for _, status := range []string{"all", "draft", "published"} {
		empty := map[string]any{
			"Playbooks": []store.AuthorPlaybookRow{}, "Cities": []store.Jurisdiction{},
			"Status": status, "TotalCount": 0, "DraftCount": 0, "PublishedCount": 0,
		}
		if err := tmpl.ExecuteTemplate(io.Discard, "dashboard.html", empty); err != nil {
			t.Fatalf("execute dashboard.html (empty, status=%s): %v", status, err)
		}
	}
}

func TestCandidatesTemplateRenders(t *testing.T) {
	tmpl := parseTemplates(t)
	data := map[string]any{
		"City":   store.Jurisdiction{Name: "Chicago", Slug: "chicago"},
		"Status": "pending",
		"Candidates": []candidateView{
			{
				SourceCandidate: store.SourceCandidate{
					ID: 1, URL: "https://example.gov/rlto", Publisher: "City of Chicago",
					Title: "RLTO", KindGuess: "statute", Rationale: "primary law",
					Confidence: 1.0, DiscoveredVia: "registry", Status: "pending",
				},
				ConfidencePct: 100,
			},
			{
				SourceCandidate: store.SourceCandidate{
					ID: 2, URL: "https://example.org", Publisher: "MTO",
					Title: "MTO", KindGuess: "nonprofit", Confidence: 0.8,
					DiscoveredVia: "registry", Status: "approved",
				},
				ConfidencePct: 80,
			},
		},
	}
	if err := tmpl.ExecuteTemplate(io.Discard, "candidates.html", data); err != nil {
		t.Fatalf("execute candidates.html: %v", err)
	}

	// Empty state for a non-pending tab.
	empty := map[string]any{
		"City": store.Jurisdiction{Name: "Chicago", Slug: "chicago"}, "Status": "rejected",
		"Candidates": []candidateView{},
	}
	if err := tmpl.ExecuteTemplate(io.Discard, "candidates.html", empty); err != nil {
		t.Fatalf("execute candidates.html (empty): %v", err)
	}
}

// renderForm executes form.html and returns the HTML, failing the test if the
// template errors.
func renderForm(t *testing.T, data map[string]any) string {
	t.Helper()
	var buf bytes.Buffer
	if err := parseTemplates(t).ExecuteTemplate(&buf, "form.html", data); err != nil {
		t.Fatalf("execute form.html: %v", err)
	}
	return buf.String()
}

// allJurisdictions is the picker's input: one of each level, which is what the
// real registry looks like once a national or statewide page exists.
func allJurisdictions() []store.Jurisdiction {
	return []store.Jurisdiction{
		{Kind: "country", Name: "United States", Slug: "united-states"},
		{Kind: "state", Name: "Massachusetts", Slug: "massachusetts"},
		{Kind: "city", Name: "Boston", Slug: "boston", ParentSlug: "massachusetts", ParentName: "Massachusetts"},
	}
}

func topicRegistry() []store.Topic {
	return []store.Topic{
		{ID: 1, Slug: "renting-fundamentals", Name: "Renting Basics"},
		{ID: 2, Slug: "security-deposits", Name: "Security Deposits", IsCore: true},
	}
}

// A page can be scoped to any level of the hierarchy, so the picker has to be
// able to represent every level. When it listed cities only, a national or
// statewide draft rendered with nothing selected — and a required select with
// no selection refuses to submit, so the page could be edited but never saved.
func TestFormTemplateSelectsNonCityJurisdictions(t *testing.T) {
	for _, tc := range []struct {
		name, slug, wantOption string
	}{
		{"national", "united-states", `value="united-states" selected`},
		{"statewide", "massachusetts", `value="massachusetts" selected`},
		{"city", "boston", `value="boston" selected`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			html := renderForm(t, map[string]any{
				"EditMode": true, "EditID": int64(41), "Status": "draft",
				"Jurisdictions": allJurisdictions(), "Topics": topicRegistry(),
				"SelectedCitySlug": tc.slug, "SelectedTopicKey": "renting-fundamentals",
				"SelectedPageKind": "playbook", "Title": "T", "Intro": "I",
				"PreloadJSON": template.JS("null"),
			})
			if !strings.Contains(html, tc.wantOption) {
				t.Errorf("no option selected for %s — the form would submit an empty jurisdiction", tc.slug)
			}
			if n := strings.Count(html, `<option value="`+tc.slug+`"`); n != 1 {
				t.Errorf("expected exactly one option for %s, got %d", tc.slug, n)
			}
		})
	}
}

// Page type is rendering, not identity: locking it alongside the URL fields
// meant a published edit posted no page_kind, and the empty value defaulted
// back to "playbook", silently converting directory pages.
func TestFormTemplateKeepsPageKindOnPublishedEdit(t *testing.T) {
	html := renderForm(t, map[string]any{
		"EditMode": true, "EditID": int64(2), "Status": "published",
		"CityName": "Pittsburgh", "TopicSlug": "resource-directory",
		"Topics": topicRegistry(), "SelectedPageKind": "directory",
		"Title": "T", "Intro": "I", "PreloadJSON": template.JS("null"),
	})
	if !strings.Contains(html, `name="page_kind"`) {
		t.Fatal("published edit form posts no page_kind — the save would reset it to playbook")
	}
	if !strings.Contains(html, `value="directory" selected`) {
		t.Error("page type selector does not reflect the page's stored kind")
	}
	// City and topic stay locked: those are the URL, and changing them under a
	// live page is what ADR-005 D5 makes unrepresentable.
	if strings.Contains(html, `name="jurisdiction_select"`) || strings.Contains(html, `name="topic_key"`) {
		t.Error("published edit must not expose the jurisdiction or topic selects")
	}
}

// A validation error used to re-render with no Topics at all, leaving an empty
// topic dropdown the author could not resubmit from.
func TestFormTemplateErrorStateKeepsTopics(t *testing.T) {
	html := renderForm(t, map[string]any{
		"Jurisdictions": allJurisdictions(), "Topics": topicRegistry(),
		"Error": "Statement 2 needs at least one citation", "PreloadJSON": template.JS("null"),
	})
	if !strings.Contains(html, `value="security-deposits"`) {
		t.Error("topic dropdown is empty after a validation error")
	}
	if !strings.Contains(html, "Statement 2 needs at least one citation") {
		t.Error("validation message not shown")
	}
}
