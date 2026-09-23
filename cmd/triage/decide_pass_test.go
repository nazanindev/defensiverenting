package main

import (
	"context"
	"strings"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/drafting"
)

func TestPassVerdict(t *testing.T) {
	it := passItem{Position: 3, Citations: []citationOut{{URL: "https://law.example.gov/s1"}}}
	check := func(_ context.Context, _ int, url, quote string) drafting.QuoteVerdict {
		if strings.Contains(url, "blocked") {
			return drafting.QuoteVerdict{Overridable: true}
		}
		return drafting.QuoteVerdict{Verified: quote == "the landlord shall return the deposit within 30 days"}
	}
	ok := passDecision{Verdict: "pass", SourceURL: "https://law.example.gov/s1", Passage: "the landlord shall return the deposit within 30 days", Reason: "supports the 30-day claim"}
	if why := passVerdict(context.Background(), ok, it, check); why != "" {
		t.Fatalf("good pass refused: %s", why)
	}
	cases := map[string]func(d *passDecision){
		"leave":          func(d *passDecision) { d.Verdict = "leave" },
		"no reason":      func(d *passDecision) { d.Reason = "" },
		"no passage":     func(d *passDecision) { d.Passage = "" },
		"not verbatim":   func(d *passDecision) { d.Passage = "the landlord returns the deposit in 30 days" },
		"uncited source": func(d *passDecision) { d.SourceURL = "https://other.example.gov" },
		"over the cap":   func(d *passDecision) { d.Passage = strings.Repeat("word ", flagPassageCap+1) },
	}
	for name, mutate := range cases {
		d := ok
		mutate(&d)
		if why := passVerdict(context.Background(), d, it, check); why == "" {
			t.Errorf("%s: passed; it must stay for a person", name)
		}
	}
	blocked := passItem{Position: 1, Citations: []citationOut{{URL: "https://blocked.example.gov"}}}
	d := ok
	d.SourceURL = "https://blocked.example.gov"
	if why := passVerdict(context.Background(), d, blocked, check); !strings.Contains(why, "could not be read") {
		t.Errorf("unreadable source: %q", why)
	}
}

// Site guidance alone passes on its editorial citation; a statement that also
// cites a law must give its passage from the law.
func TestPassVerdictEditorial(t *testing.T) {
	check := func(context.Context, int, string, string) drafting.QuoteVerdict { return drafting.QuoteVerdict{} }
	d := passDecision{Verdict: "pass", SourceURL: editorialURL, Passage: "site guidance", Reason: "private-inspector guidance"}
	only := passItem{Citations: []citationOut{{URL: editorialURL, Kind: "editorial"}}}
	if why := passVerdict(context.Background(), d, only, check); why != "" {
		t.Fatalf("site guidance refused: %s", why)
	}
	mixed := passItem{Citations: []citationOut{{URL: editorialURL, Kind: "editorial"}, {URL: "https://law.example.gov/s1"}}}
	if why := passVerdict(context.Background(), d, mixed, check); why == "" {
		t.Error("a statement citing a law passed on its editorial citation")
	}
}
