package main

import (
	"bytes"
	"html/template"
	"io"
	"net/http/httptest"
	"net/url"
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

// dashboardData mirrors every key s.dashboard puts in the render map. Keeping
// it in one place means a template that starts reading a new field fails here
// once, with the field named, rather than in each fixture separately.
func dashboardData(status string, playbooks []store.AuthorPlaybookRow) map[string]any {
	coverage := []store.CoverageRow{{
		JurisdictionName: "Chicago", JurisdictionSlug: "chicago",
		Status: map[string]string{"security-deposits": "published", "resource-directory": ""},
	}}
	return map[string]any{
		"Playbooks": playbooks,
		"Cities":    []store.Jurisdiction{{Name: "Chicago", Slug: "chicago"}},
		"ReviewCounts": []store.CandidateCountRow{
			{JurisdictionName: "Chicago", JurisdictionSlug: "chicago", PendingCount: 7},
		},
		"Flagged": []store.Source{},
		"View":    dashboardView{Status: status}.normalize(),
		"Status":  status,
		"CoreTopics": []store.Topic{
			{Slug: "security-deposits", Name: "Security Deposits", IsCore: true},
			{Slug: "resource-directory", Name: "Local Help", IsCore: true},
		},
		"Coverage":       coverage,
		"ShowLanguage":   false,
		"TotalCount":     len(playbooks),
		"DraftCount":     len(playbooks),
		"PublishedCount": 0,
	}
}

func TestDashboardTemplateRenders(t *testing.T) {
	tmpl := parseTemplates(t)
	data := dashboardData("all", []store.AuthorPlaybookRow{
		{ID: 1, Title: "Heat Not Working", JurisdictionName: "Chicago", Status: "draft", PageKind: "playbook", Language: "en"},
	})
	if err := tmpl.ExecuteTemplate(io.Discard, "dashboard.html", data); err != nil {
		t.Fatalf("execute dashboard.html: %v", err)
	}

	// Empty state differs per tab, so each one has to render.
	for _, status := range []string{"all", "draft", "published"} {
		empty := dashboardData(status, []store.AuthorPlaybookRow{})
		if err := tmpl.ExecuteTemplate(io.Discard, "dashboard.html", empty); err != nil {
			t.Fatalf("execute dashboard.html (empty, status=%s): %v", status, err)
		}
	}
}

// The sort headers are links built in Go and marked trusted, so a regression
// there ships as a page full of dead links that still look right.
func TestDashboardTemplate_sortHeadersAreUsableLinks(t *testing.T) {
	var buf bytes.Buffer
	if err := parseTemplates(t).ExecuteTemplate(&buf, "dashboard.html",
		dashboardData("all", []store.AuthorPlaybookRow{{ID: 1, Title: "T", JurisdictionName: "Chicago", Status: "draft", PageKind: "playbook"}})); err != nil {
		t.Fatal(err)
	}
	html := buf.String()
	if !strings.Contains(html, `href="/?dir=`) {
		t.Error("no unescaped sort link in the rendered header row")
	}
	if strings.Contains(html, "%3d") || strings.Contains(html, "%26") {
		t.Error("a sort link is percent-encoded, so clicking the header would not sort")
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

// Language is part of the page's identity (jurisdiction+topic+language is the
// actual uniqueness key), so a published edit must lock it the same way it
// locks location and topic: no "language" select posted, and the stored value
// shown as a label rather than editable.
func TestFormTemplateLocksLanguageOnPublishedEdit(t *testing.T) {
	html := renderForm(t, map[string]any{
		"EditMode": true, "EditID": int64(2), "Status": "published",
		"CityName": "Pittsburgh", "TopicSlug": "resource-directory",
		"Topics": topicRegistry(), "SelectedPageKind": "directory",
		"SelectedLanguage": "es", "SelectedLanguageLabel": "Spanish",
		"Title": "T", "Intro": "I", "PreloadJSON": template.JS("null"),
	})
	if strings.Contains(html, `name="language"`) {
		t.Error("published edit must not expose a language select — the save would reuse the stored language regardless of what's posted, so an editable one would mislead")
	}
	if !strings.Contains(html, "Spanish") {
		t.Error("the page's actual language should be shown, even locked")
	}
}

// A draft's language must be pre-selected from the stored playbook, and the
// select must be posted so submitEditForm can read it — this is the fix for
// the bug where editing a Spanish draft silently reset it to English.
func TestFormTemplateSelectsLanguageOnDraftEdit(t *testing.T) {
	html := renderForm(t, map[string]any{
		"EditMode": true, "EditID": int64(3), "Status": "draft",
		"Jurisdictions": allJurisdictions(), "Topics": topicRegistry(),
		"SelectedCitySlug": "boston", "SelectedTopicKey": "security-deposits",
		"SelectedPageKind": "playbook", "SelectedLanguage": "es",
		"Languages": []languageOption{{Code: "en", Label: "English"}, {Code: "es", Label: "Spanish"}},
		"Title":     "T", "Intro": "I", "PreloadJSON": template.JS("null"),
	})
	if !strings.Contains(html, `name="language"`) {
		t.Fatal("draft edit form posts no language — saving would fall back to en regardless of the stored draft's language")
	}
	if !strings.Contains(html, `value="es" selected`) {
		t.Error("language selector does not reflect the draft's stored language")
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

// A validation error must hand back what was submitted. Re-rendering from
// stored state discarded every statement and source the author had in the
// browser, which on a 16-statement page meant one missing citation cost the
// whole session.
func TestPreloadFromFormRoundTripsASubmission(t *testing.T) {
	form := url.Values{}
	// Card ids are whatever the JS handed out; 4 and 9 survive after the author
	// added and removed cards, and the reconstruction has to renumber them.
	form.Set("active_sources", "4,9")
	form.Set("src_url_4", " https://malegislature.gov/Laws/GeneralLaws/PartII/TitleI/Chapter186/Section15B ")
	form.Set("src_pub_4", " Massachusetts Legislature ")
	form.Set("src_kind_4", "statute")
	form.Set("src_loc_4", "§ 15B(4)")
	form.Set("src_url_9", "https://www.mass.gov/info-details/security-deposits")
	form.Set("src_pub_9", "Mass.gov")
	form.Set("src_kind_9", "gov_guidance")

	form.Set("active_stmts", "2,7")
	form.Set("stmt_2", "Your landlord must return the deposit within 30 days of the end of the tenancy.")
	form.Set("cite_2_4", "on")
	form.Set("loc_2_4", "§ 15B(4)(iii)")
	form.Set("stmt_7", "Keep a copy of every letter you send.")
	form.Set("edit_7", "on")

	r := httptest.NewRequest("POST", "/new", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	got := preloadFromForm(r)

	if len(got.Sources) != 2 || len(got.Stmts) != 2 {
		t.Fatalf("lost cards: %d sources, %d statements", len(got.Sources), len(got.Stmts))
	}
	// Renumbered to sequential positions, since that is what the form re-renders.
	if got.Sources[0].ID != 0 || got.Sources[1].ID != 1 {
		t.Errorf("sources not renumbered: %d, %d", got.Sources[0].ID, got.Sources[1].ID)
	}
	if got.Sources[0].URL != "https://malegislature.gov/Laws/GeneralLaws/PartII/TitleI/Chapter186/Section15B" {
		t.Errorf("source URL not trimmed or lost: %q", got.Sources[0].URL)
	}
	if got.Sources[0].Publisher != "Massachusetts Legislature" {
		t.Errorf("publisher not trimmed or lost: %q", got.Sources[0].Publisher)
	}
	if got.Sources[0].Locator != "§ 15B(4)" {
		t.Errorf("source locator lost: %q", got.Sources[0].Locator)
	}

	// Citation checkboxes point at renumbered source positions, not card ids.
	if len(got.Stmts[0].Cites) != 1 || got.Stmts[0].Cites[0] != 0 {
		t.Errorf("citation lost or misaddressed: %v", got.Stmts[0].Cites)
	}
	if got.Stmts[0].Locators["0"] != "§ 15B(4)(iii)" {
		t.Errorf("per-statement locator override lost: %v", got.Stmts[0].Locators)
	}
	if !strings.HasPrefix(got.Stmts[0].Body, "Your landlord must return") {
		t.Errorf("statement body lost: %q", got.Stmts[0].Body)
	}
	if !got.Stmts[1].Editorial {
		t.Error("editorial flag lost")
	}
	if len(got.Stmts[1].Cites) != 0 {
		t.Errorf("editorial statement gained citations: %v", got.Stmts[1].Cites)
	}
}

// An empty statement is usually the one the author has to come back and fill
// in, so it has to survive the round trip rather than being dropped.
func TestPreloadFromFormKeepsEmptyStatements(t *testing.T) {
	form := url.Values{}
	form.Set("active_sources", "0")
	form.Set("src_url_0", "https://example.gov")
	form.Set("src_pub_0", "Example")
	form.Set("active_stmts", "0,1")
	form.Set("stmt_0", "Written down.")
	form.Set("cite_0_0", "on")
	form.Set("stmt_1", "")

	r := httptest.NewRequest("POST", "/new", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if got := preloadFromForm(r); len(got.Stmts) != 2 {
		t.Fatalf("empty statement dropped: got %d statements", len(got.Stmts))
	}
}

// Nothing submitted must not look like a submission, or editErr would replace
// the stored playbook with an empty form.
func TestPreloadFromFormEmptyRequest(t *testing.T) {
	r := httptest.NewRequest("POST", "/new", strings.NewReader(""))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	got := preloadFromForm(r)
	if len(got.Sources) != 0 || len(got.Stmts) != 0 {
		t.Errorf("expected an empty reconstruction, got %d sources / %d statements",
			len(got.Sources), len(got.Stmts))
	}
}

// The verbatim quote is the site's central guarantee, and the form never
// carried it: every save wrote citations.quote back as "". A reviewer opening a
// draft, changing nothing and clicking save destroyed the evidence the drafting
// agent had verified — and since the source checker skips unquoted citations
// (internal/store/monitor.go), the page then published into a blind spot.
func TestPreloadFromFormPreservesQuotes(t *testing.T) {
	quote := "The lessor shall, within thirty days after the termination of occupancy, return to the tenant the security deposit"

	form := url.Values{}
	form.Set("active_sources", "0")
	form.Set("src_url_0", "https://malegislature.gov/Laws/GeneralLaws/PartII/TitleI/Chapter186/Section15B")
	form.Set("src_pub_0", "Massachusetts Legislature")
	form.Set("src_kind_0", "statute")
	form.Set("active_stmts", "0")
	form.Set("stmt_0", "Your landlord must return the deposit within 30 days.")
	form.Set("cite_0_0", "on")
	form.Set("loc_0_0", "§ 15B(4)")
	form.Set("quote_0_0", quote)

	r := httptest.NewRequest("POST", "/edit/1", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	got := preloadFromForm(r)
	if got.Stmts[0].Quotes["0"] != quote {
		t.Errorf("quote lost on round trip: %q", got.Stmts[0].Quotes["0"])
	}
}

// buildPreload is the other half: a quote in the database has to reach the form
// or the next save has nothing to hand back.
func TestBuildPreloadCarriesQuotes(t *testing.T) {
	const editorialID int64 = 99
	quote := "Heat shall be provided from September 15th to June 15th"

	pw := store.PlaybookWithStatements{
		Statements: []store.CitedStatement{{
			BodyMD: "Your landlord must heat the apartment.",
			Citations: []store.CitationWithSource{
				{SourceID: 7, SourceURL: "https://example.gov/heat", Publisher: "City", SourceKind: "regulation",
					Locator: "§ 410.201", Quote: quote},
				{SourceID: editorialID},
			},
		}},
	}

	got := buildPreload(pw, editorialID)
	if len(got.Stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(got.Stmts))
	}
	if got.Stmts[0].Quotes["0"] != quote {
		t.Errorf("quote did not reach the form: %q", got.Stmts[0].Quotes["0"])
	}
	if !got.Stmts[0].Editorial {
		t.Error("editorial flag lost")
	}
}

// A full loop: database -> form -> submitted form -> database. This is the save
// that used to strip the quote.
func TestQuoteSurvivesAnUnchangedSave(t *testing.T) {
	const editorialID int64 = 99
	quote := "no landlord shall increase rent without thirty days written notice"

	pw := store.PlaybookWithStatements{
		Statements: []store.CitedStatement{{
			BodyMD: "Your landlord owes you 30 days notice.",
			Citations: []store.CitationWithSource{{
				SourceID: 3, SourceURL: "https://example.gov/rent", Publisher: "State",
				SourceKind: "statute", Locator: "§ 12", Quote: quote,
			}},
		}},
	}

	// Database -> form.
	loaded := buildPreload(pw, editorialID)

	// Form -> POST, exactly as the rendered fields would serialise it.
	form := url.Values{}
	form.Set("active_sources", "0")
	form.Set("src_url_0", loaded.Sources[0].URL)
	form.Set("src_pub_0", loaded.Sources[0].Publisher)
	form.Set("src_kind_0", loaded.Sources[0].Kind)
	form.Set("active_stmts", "0")
	form.Set("stmt_0", loaded.Stmts[0].Body)
	form.Set("cite_0_0", "on")
	form.Set("loc_0_0", loaded.Stmts[0].Locators["0"])
	form.Set("quote_0_0", loaded.Stmts[0].Quotes["0"])

	r := httptest.NewRequest("POST", "/edit/1", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = r.ParseForm()

	// POST -> what the handler would write to citations.quote.
	if written := r.FormValue("quote_0_0"); written != quote {
		t.Fatalf("an unchanged save would write quote=%q, want %q", written, quote)
	}
	if r.FormValue("loc_0_0") != "§ 12" {
		t.Errorf("locator lost: %q", r.FormValue("loc_0_0"))
	}
}
