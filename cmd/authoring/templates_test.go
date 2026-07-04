package main

import (
	"html/template"
	"io"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// parseTemplates mirrors the parsing done in main() so tests catch template
// syntax errors and undefined functions/fields without needing a database.
func parseTemplates(t *testing.T) *template.Template {
	t.Helper()
	tmpl, err := template.ParseFS(templateFS, "templates/*.html")
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
	}
	if err := tmpl.ExecuteTemplate(io.Discard, "dashboard.html", data); err != nil {
		t.Fatalf("execute dashboard.html: %v", err)
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
