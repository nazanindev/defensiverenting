package handlers_test

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/http/handlers"
	"github.com/nazanindev/defensiverenting/internal/store"
)

func describe(t *testing.T, kind, name, intro string) string {
	return describeIn(t, "en", kind, name, intro)
}

func describeIn(t *testing.T, lang, kind, name, intro string) string {
	t.Helper()
	pb := store.PlaybookWithStatements{
		Playbook:     store.Playbook{Title: "Security Deposit Not Returned: What Can I Do?", Language: lang, IntroMD: intro},
		Jurisdiction: store.Jurisdiction{Kind: kind, Name: name, Slug: "x"},
		Topic:        store.Topic{Slug: "security-deposits", Name: "Security Deposits"},
	}
	page := handlers.BuildPlaybookPage(context.Background(), pb, nil, nil, slog.Default())
	return page.Description
}

// The snippet has to tell a searcher who typed no place that this page is
// for their place, or that they will pick it here. The intro alone did not.
func TestDescription_namesThePlaceAndCutsAtASentence(t *testing.T) {
	intro := "Your deposit is your money. Your landlord has 30 days after you move out to return it or explain in writing what they kept. If they miss that deadline you can sue for up to twice the deposit."
	got := describe(t, "city", "Seattle", intro)
	want := "Your deposit is your money. The rules for Seattle and what to do next."
	if got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
	if len(got) > 160 {
		t.Fatalf("too long for a snippet: %d", len(got))
	}
}

func TestDescription_nationwideSaysRulesDifferByState(t *testing.T) {
	got := describe(t, "country", "United States", "Every state writes its own deposit rules.")
	if !strings.HasSuffix(got, "Rules differ by state. Pick yours on this page.") {
		t.Fatalf("got %q", got)
	}
}

func TestDescription_fallsBackToTheTitle(t *testing.T) {
	got := describe(t, "state", "Texas", "")
	if got != "Security Deposit Not Returned: What Can I Do? The rules for Texas and what to do next." {
		t.Fatalf("got %q", got)
	}
}

func TestDescription_wordBoundaryWhenNoSentenceFits(t *testing.T) {
	intro := strings.Repeat("word ", 60)
	got := describe(t, "city", "Austin", intro)
	if len(got) > 160 || !strings.Contains(got, "… The rules for Austin") {
		t.Fatalf("got %q (%d)", got, len(got))
	}
}

func TestDescription_spanishPageGetsASpanishTail(t *testing.T) {
	got := describeIn(t, "es", "city", "Austin", "Su depósito es su dinero.")
	if got != "Su depósito es su dinero. Las reglas para Austin y qué hacer después." {
		t.Fatalf("got %q", got)
	}
}
