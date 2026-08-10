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
