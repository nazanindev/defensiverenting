package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nazanindev/defensiverenting/internal/store"
)

func TestDashboardView_normalizeRejectsAnythingNotOnTheAllowlist(t *testing.T) {
	got := dashboardView{Status: "; DROP", Sort: "../../etc", Dir: "sideways"}.normalize()
	want := dashboardView{Status: "all", Sort: "updated", Dir: "desc"}
	if got != want {
		t.Errorf("normalize = %+v, want %+v", got, want)
	}
}

// Dates read newest-first; everything else reads A to Z. Getting this backwards
// makes the default view show the oldest pages, which is never what a review
// queue wants.
func TestDashboardView_defaultDirectionDependsOnTheColumn(t *testing.T) {
	if d := (dashboardView{Sort: "updated"}).normalize().Dir; d != "desc" {
		t.Errorf("updated default dir = %q, want desc", d)
	}
	if d := (dashboardView{Sort: "city"}).normalize().Dir; d != "asc" {
		t.Errorf("city default dir = %q, want asc", d)
	}
}

// The bug this guards: SortLink used to return a plain string, and
// html/template treats a bare value after "/?" as one query parameter, so every
// sort link shipped as href="/?dir%3dasc%26sort%3dcity" — a single meaningless
// parameter. Every header looked fine and none of them sorted.
func TestSortLink_isARealURLNotAnEscapedParameter(t *testing.T) {
	link := string(dashboardView{Status: "draft", Sort: "city", Dir: "asc"}.normalize().SortLink("title"))
	if strings.Contains(link, "%3d") || strings.Contains(link, "%26") {
		t.Fatalf("SortLink = %q — the separators are percent-encoded, so the link does not sort", link)
	}
	for _, want := range []string{"/?", "sort=title", "status=draft"} {
		if !strings.Contains(link, want) {
			t.Errorf("SortLink = %q, want it to contain %q", link, want)
		}
	}
}

func TestSortLink_flipsOnlyTheColumnAlreadySorted(t *testing.T) {
	v := dashboardView{Status: "all", Sort: "city", Dir: "asc"}.normalize()
	if got := string(v.SortLink("city")); !strings.Contains(got, "dir=desc") {
		t.Errorf("re-clicking the sorted column = %q, want dir=desc", got)
	}
	// A different column starts at its own natural direction rather than
	// inheriting the current one.
	if got := string(v.SortLink("updated")); !strings.Contains(got, "dir=desc") {
		t.Errorf("updated = %q, want its natural dir=desc", got)
	}
	if got := string(v.SortLink("title")); !strings.Contains(got, "dir=asc") {
		t.Errorf("title = %q, want its natural dir=asc", got)
	}
}

func TestNormalize_boundsLanguageToTheTwoSupportedCodes(t *testing.T) {
	if got := (dashboardView{Lang: "fr"}).normalize().Lang; got != "" {
		t.Errorf("Lang = %q, want an unknown code cleared", got)
	}
	if got := (dashboardView{Lang: "es"}).normalize().Lang; got != "es" {
		t.Errorf("Lang = %q, want es kept", got)
	}
}

func TestLangLink_keepsPlaceStatusAndSort(t *testing.T) {
	v := dashboardView{Status: "draft", Sort: "city", Dir: "asc", Place: "texas"}.normalize()
	got := string(v.LangLink("en"))
	for _, want := range []string{"lang=en", "place=texas", "status=draft", "sort=city"} {
		if !strings.Contains(got, want) {
			t.Errorf("LangLink = %q, missing %s", got, want)
		}
	}
	if got := string(v.StatusLink("published")); !strings.Contains(got, "place=texas") || strings.Contains(got, "lang=") {
		t.Errorf("StatusLink = %q, want place carried and no lang when none is set", got)
	}
}

func TestReadView_rememberedLanguageSurvivesABareRequest(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: viewCookie, Value: "status=draft&sort=updated&dir=desc&lang=en"})
	if got := readView(r); got.Lang != "en" {
		t.Errorf("readView.Lang = %q, want the remembered en", got.Lang)
	}
}

func TestReadView_queryBeatsTheRememberedCookie(t *testing.T) {
	r := httptest.NewRequest("GET", "/?status=published&sort=title&dir=asc", nil)
	r.AddCookie(&http.Cookie{Name: viewCookie, Value: "status=draft&sort=city&dir=desc"})
	got := readView(r)
	want := dashboardView{Status: "published", Sort: "title", Dir: "asc"}
	if got != want {
		t.Errorf("readView = %+v, want the explicit query %+v", got, want)
	}
}

// Publish, delete and save all redirect to a bare "/". Without the cookie the
// sort resets on every action, which is what makes working through one slice of
// the queue impossible.
func TestReadView_bareRequestRestoresTheRememberedView(t *testing.T) {
	r := httptest.NewRequest("GET", "/?msg=Published", nil)
	r.AddCookie(&http.Cookie{Name: viewCookie, Value: "status=draft&sort=city&dir=desc"})
	got := readView(r)
	want := dashboardView{Status: "draft", Sort: "city", Dir: "desc"}
	if got != want {
		t.Errorf("readView = %+v, want the remembered %+v", got, want)
	}
}

func TestReadView_ignoresAGarbageCookie(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: viewCookie, Value: "status=%zz&sort=;DROP"})
	if got := readView(r); got.Status != "all" || got.Sort != "updated" {
		t.Errorf("readView = %+v, want the safe default", got)
	}
}

func rows() []store.AuthorPlaybookRow {
	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return []store.AuthorPlaybookRow{
		{Title: "b", JurisdictionName: "Seattle", Status: "published", StatementCount: 9, UpdatedAt: old},
		{Title: "a", JurisdictionName: "Austin", Status: "superseded", StatementCount: 3, UpdatedAt: old.AddDate(0, 0, 2)},
		{Title: "c", JurisdictionName: "Boston", Status: "draft", StatementCount: 14, UpdatedAt: old.AddDate(0, 0, 1)},
	}
}

func TestSortPlaybooks(t *testing.T) {
	cases := []struct {
		key, dir string
		wantTop  string
	}{
		{"city", "asc", "Austin"},
		{"city", "desc", "Seattle"},
		{"size", "desc", "Boston"},
		{"updated", "desc", "Austin"},
		// The list is a review queue, so ascending status means the thing most
		// needing attention, not the alphabetically first one.
		{"status", "asc", "Boston"},
	}
	for _, c := range cases {
		r := rows()
		sortPlaybooks(r, c.key, c.dir)
		if r[0].JurisdictionName != c.wantTop {
			t.Errorf("sort %s %s: top = %q, want %q", c.key, c.dir, r[0].JurisdictionName, c.wantTop)
		}
	}
}

func TestNormalize_boundsKindAndTrimsTheTextFilter(t *testing.T) {
	v := dashboardView{Kind: "poem", Q: "  deposit  "}.normalize()
	if v.Kind != "" {
		t.Errorf("Kind = %q, want an unknown kind cleared", v.Kind)
	}
	if v.Q != "deposit" {
		t.Errorf("Q = %q, want trimmed", v.Q)
	}
	if got := (dashboardView{Kind: "rules"}).normalize().Kind; got != "rules" {
		t.Errorf("Kind = %q, want rules kept", got)
	}
	long := strings.Repeat("x", maxQueryLen+20)
	if got := (dashboardView{Q: long}).normalize().Q; len(got) != maxQueryLen {
		t.Errorf("len(Q) = %d, want capped at %d", len(got), maxQueryLen)
	}
}

// The text filter reads the title, the place and the topic, case-insensitively:
// "deposit" finds "Can my landlord keep my deposit?" and the
// security-deposits topic alike.
func TestDashboardView_matchesTitlePlaceAndTopic(t *testing.T) {
	row := store.AuthorPlaybookRow{Title: "Can my landlord keep my Deposit?", JurisdictionName: "Ohio", TopicSlug: "security-deposits", PageKind: "playbook"}
	for _, q := range []string{"deposit", "OHIO", "security-dep"} {
		if !(dashboardView{Q: q}).normalize().matches(row) {
			t.Errorf("q=%q did not match", q)
		}
	}
	if (dashboardView{Q: "eviction"}).normalize().matches(row) {
		t.Error("q=eviction matched a deposit page")
	}
	if (dashboardView{Kind: "rules"}).normalize().matches(row) {
		t.Error("kind=rules matched a playbook")
	}
	if !(dashboardView{Kind: "playbook", Q: "ohio"}).normalize().matches(row) {
		t.Error("kind and text together did not match")
	}
}

func TestKindLink_keepsTheRestOfTheView(t *testing.T) {
	v := dashboardView{Status: "draft", Sort: "city", Dir: "asc", Place: "texas", Q: "deposit"}.normalize()
	got := string(v.KindLink("rules"))
	for _, want := range []string{"kind=rules", "place=texas", "status=draft", "sort=city", "q=deposit"} {
		if !strings.Contains(got, want) {
			t.Errorf("KindLink = %q, missing %s", got, want)
		}
	}
	if sl := string(v.SortLink("title")); !strings.Contains(sl, "q=deposit") {
		t.Errorf("SortLink = %q dropped the text filter", sl)
	}
}

// Each column filter narrows on its own field and composes with the others.
func TestDashboardView_columnFilters(t *testing.T) {
	row := store.AuthorPlaybookRow{Title: "Deposit", JurisdictionName: "Ohio", TopicSlug: "security-deposits", PageKind: "playbook", UpdatedBy: "Nazanin", HelpfulYes: 2}
	cases := []struct {
		v    dashboardView
		want bool
	}{
		{dashboardView{Topic: "security-deposits"}, true},
		{dashboardView{Topic: "eviction"}, false},
		{dashboardView{By: "Nazanin"}, true},
		{dashboardView{By: "agent"}, false},
		{dashboardView{Helpful: "answered"}, true},
		{dashboardView{Helpful: "unanswered"}, false},
		{dashboardView{Helpful: "maybe"}, true}, // unknown value is cleared, not a filter
		{dashboardView{Topic: "security-deposits", By: "Nazanin", Helpful: "answered", Q: "dep"}, true},
	}
	for _, c := range cases {
		if got := c.v.normalize().matches(row); got != c.want {
			t.Errorf("%+v matches = %v, want %v", c.v, got, c.want)
		}
	}
}

func TestLink_setsOneFieldAndKeepsTheRest(t *testing.T) {
	v := dashboardView{Status: "draft", Sort: "city", Dir: "asc", Place: "texas", Topic: "eviction", By: "Nazanin"}.normalize()
	got := string(v.Link("helpful", "answered"))
	for _, want := range []string{"helpful=answered", "place=texas", "topic=eviction", "by=Nazanin", "status=draft", "sort=city"} {
		if !strings.Contains(got, want) {
			t.Errorf("Link = %q, missing %s", got, want)
		}
	}
	if cleared := string(v.Link("topic", "")); strings.Contains(cleared, "topic=") {
		t.Errorf("Link(topic, \"\") = %q, want topic cleared", cleared)
	}
	if !v.Filtered() || (dashboardView{Status: "draft"}).normalize().Filtered() {
		t.Error("Filtered does not reflect the column filters")
	}
}
