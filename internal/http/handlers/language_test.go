package handlers_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// ADR-007 D1: bare English routes are unchanged (every other test in this
// package pins that already); /es/* mirrors them under a literal prefix, and
// redirect targets built along the way must carry that same prefix rather
// than silently dropping back to English.

func TestSpanishRoutes_stateHubServes(t *testing.T) {
	stub := hierarchyStub()
	if rec := serve(t, stub, "/es/j/texas"); rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestSpanishRoutes_flatCityURLRedirectsWithinSpanish(t *testing.T) {
	stub := hierarchyStub()
	stub.playbookErr = store.ErrNotFound
	assertRedirect(t, serve(t, stub, "/es/j/austin"), "/es/j/texas/austin")
	assertRedirect(t, serve(t, stub, "/es/j/austin/security-deposits"), "/es/j/texas/austin/security-deposits")
}

func TestSpanishRoutes_cityUnderWrongStateStaysInSpanish(t *testing.T) {
	stub := hierarchyStub()
	assertRedirect(t, serve(t, stub, "/es/j/illinois/austin"), "/es/j/texas/austin")
}

func TestSpanishRoutes_retiredJurisdictionAliasStaysInSpanish(t *testing.T) {
	stub := &stubStore{
		jurisdictions:       []store.Jurisdiction{{ID: 1, Kind: "city", Name: "Boston", Slug: "boston", ParentSlug: "massachusetts"}},
		jurisdictionAliases: map[string]string{"boston-ma": "boston"},
	}
	assertRedirect(t, serve(t, stub, "/es/j/boston-ma"), "/es/j/massachusetts/boston")
}

func TestSpanishRoutes_retiredTopicAliasStaysInSpanish(t *testing.T) {
	stub := &stubStore{
		jurisdictions: []store.Jurisdiction{{ID: 1, Kind: "city", Name: "Pittsburgh", Slug: "pittsburgh", ParentSlug: "pennsylvania"}},
		topics:        []store.Topic{{ID: 1, Slug: "discrimination", Name: "Housing Discrimination"}},
		topicAliases:  map[string]string{"pittsburgh-discrimination": "discrimination"},
	}
	assertRedirect(t, serve(t, stub, "/es/t/pittsburgh-discrimination"), "/es/t/discrimination")
}

// An unsupported language code was never registered as a route at all, so it
// 404s like any other unknown path rather than resolving to English.
func TestUnsupportedLanguagePrefix404s(t *testing.T) {
	stub := hierarchyStub()
	if rec := serve(t, stub, "/fr/j/texas"); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func spanishPlaybookStub(otherExists bool) *stubStore {
	esPlaybook := store.PlaybookWithStatements{
		Playbook:     store.Playbook{ID: 2, Title: "Depósito de Seguridad", Slug: "security-deposits", Language: "es"},
		Jurisdiction: store.Jurisdiction{ID: 1, Kind: "city", Name: "Boston", Slug: "boston", ParentSlug: "massachusetts"},
		Topic:        store.Topic{ID: 1, Slug: "security-deposits", Name: "Depósito de Seguridad"},
		Statements: []store.CitedStatement{{
			ID: 1, BodyMD: "Una afirmación.",
			Citations: []store.CitationWithSource{{SourceID: 1, SourceURL: "https://example.gov/law", Publisher: "Example", SourceKind: "statute", Locator: "§ 1"}},
		}},
	}
	byLang := map[string]store.PlaybookWithStatements{"es": esPlaybook}
	if otherExists {
		byLang["en"] = store.PlaybookWithStatements{
			Playbook:     store.Playbook{ID: 1, Title: "Security Deposits", Slug: "security-deposits", Language: "en"},
			Jurisdiction: esPlaybook.Jurisdiction,
			Topic:        esPlaybook.Topic,
		}
	}
	return &stubStore{
		jurisdictions:      []store.Jurisdiction{{ID: 1, Kind: "city", Name: "Boston", Slug: "boston", ParentSlug: "massachusetts"}},
		otherLangPlaybooks: byLang,
	}
}

// <html lang> must reflect the playbook actually served, not the route's
// bare default.
func TestSpanishPlaybook_htmlLangIsSpanish(t *testing.T) {
	rec := serve(t, spanishPlaybookStub(true), "/es/j/massachusetts/boston/security-deposits")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `<html lang="es">`) {
		t.Error(`expected <html lang="es">`)
	}
}

// The toggle link and hreflang alternate only render when the other language
// actually exists (ADR-007 D6) — a link that 404s is worse than no link.
func TestSpanishPlaybook_togglesToEnglishWhenItExists(t *testing.T) {
	body := serve(t, spanishPlaybookStub(true), "/es/j/massachusetts/boston/security-deposits").Body.String()
	if !strings.Contains(body, `href="/j/massachusetts/boston/security-deposits"`) {
		t.Error("expected a link to the English version")
	}
	if !strings.Contains(body, "Read this page in English") {
		t.Error("expected the toggle link's English-language label")
	}
	if !strings.Contains(body, `hreflang="en"`) || !strings.Contains(body, `hreflang="es"`) {
		t.Error("expected hreflang alternates for both languages")
	}
	if !strings.Contains(body, `hreflang="x-default" href="`+`https://renterlaw.org/j/massachusetts/boston/security-deposits"`) {
		t.Error("expected x-default to point at the English version")
	}
}

func TestSpanishPlaybook_noToggleWhenEnglishDoesNotExist(t *testing.T) {
	body := serve(t, spanishPlaybookStub(false), "/es/j/massachusetts/boston/security-deposits").Body.String()
	if strings.Contains(body, "Read this page in English") {
		t.Error("must not link to an English version that doesn't exist")
	}
	if strings.Contains(body, `hreflang="en"`) {
		t.Error("must not advertise an English alternate that doesn't exist")
	}
}

// A Spanish page must not wrap Spanish law in English chrome: the disclaimer,
// trust lines, byline, report link, and dates all follow the page's language
// (usted register, matching the translated statements). The English page keeps
// its English chrome untouched.
func TestSpanishPlaybook_chromeSpeaksSpanish(t *testing.T) {
	checked := time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)
	stub := hierarchyStub()
	stub.playbook = store.PlaybookWithStatements{
		Playbook:     store.Playbook{ID: 1, Title: "Depósitos de seguridad en Austin", Slug: "security-deposits", Language: "es"},
		Jurisdiction: store.Jurisdiction{ID: 3, Kind: "city", Name: "Austin", Slug: "austin", ParentSlug: "texas"},
		Topic:        store.Topic{ID: 1, Name: "Security Deposits", Slug: "security-deposits"},
		Statements: []store.CitedStatement{{
			ID:     1,
			BodyMD: "Un depósito de seguridad es dinero que usted paga por adelantado.",
			Citations: []store.CitationWithSource{{
				SourceID: 1, SourceURL: "https://example.gov/law", Publisher: "Texas Legislature",
				SourceKind: "statute", CheckedAt: &checked, CheckedBy: "drafting-agent",
			}},
		}},
	}

	rec := serve(t, stub, "/es/j/texas/austin/security-deposits")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Esto no es asesoría legal.",
		"Fuentes verificadas el 30 de agosto de 2026",
		"Última revisión por",
		"Avísenos",
		"Busque su situación…",
		"Todos los lugares", // footer follows the page language too
	} {
		if !strings.Contains(body, want) {
			t.Errorf("Spanish page missing %q", want)
		}
	}
	for _, banned := range []string{"Sources checked", "Not legal advice.", "Tell us"} {
		if strings.Contains(body, banned) {
			t.Errorf("Spanish page still renders English chrome %q", banned)
		}
	}
}

// The Spanish 404 explains itself in Spanish, including the search box and
// the where-to-go links.
func TestSpanishRoutes_notFoundSpeaksSpanish(t *testing.T) {
	stub := hierarchyStub()
	stub.playbookErr = store.ErrNotFound
	rec := serve(t, stub, "/es/j/texas/austin/rent-increase")
	body := rec.Body.String()
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	for _, want := range []string{"A dónde ir mientras tanto", "Por qué faltan algunas páginas", "Todos los lugares que cubrimos"} {
		if !strings.Contains(body, want) {
			t.Errorf("Spanish 404 missing %q", want)
		}
	}
}
