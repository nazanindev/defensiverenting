package main

import (
	"strings"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

func widenFixture() (store.ProposalRow, store.CitedStatement) {
	cur := store.CitedStatement{
		Key: "k", BodyMD: "The landlord returns the deposit within 30 days.", ConceptSlug: "deposit-return",
		Citations: []store.CitationWithSource{
			{SourceURL: "https://law.example.gov/s1950", Locator: "§ 1950.5(g)", Quote: "within 30 days", SourceKind: "statute"},
			{SourceURL: "https://city.example.gov/help", Locator: "", Quote: "call the office", SourceKind: "gov_guidance"},
		},
	}
	p := store.ProposalRow{Position: 3}
	p.Reason = ReasonWidenQuote
	p.Proposed = &store.ProposedStatement{
		BodyMD: cur.BodyMD, Concept: "deposit-return",
		Citations: []store.ProposedCitation{
			{URL: "https://law.example.gov/s1950", Locator: "§ 1950.5(g)", Quote: "(g) The landlord shall, within 30 days after the tenant vacates, return the deposit.", Kind: "statute"},
			{URL: "https://city.example.gov/help", Quote: "call the office", Kind: "gov_guidance"},
		},
	}
	return p, cur
}

func TestWidenVerdict_widenedQuoteIsDecidable(t *testing.T) {
	p, cur := widenFixture()
	changed, why := widenVerdict(p, cur)
	if why != "" || len(changed) != 1 || changed[0].Locator != "§ 1950.5(g)" {
		t.Fatalf("verdict = %v, %q; want the one widened statute quote and no reason", changed, why)
	}
}

func TestWidenVerdict_refusesAnythingButAWiderQuote(t *testing.T) {
	cases := map[string]func(p *store.ProposalRow){
		"body changes":    func(p *store.ProposalRow) { p.Proposed.BodyMD += " Or else." },
		"tag changes":     func(p *store.ProposalRow) { p.Proposed.Concept = "" },
		"quote shrinks":   func(p *store.ProposalRow) { p.Proposed.Citations[0].Quote = "30 days" },
		"quote moves":     func(p *store.ProposalRow) { p.Proposed.Citations[0].Quote = "a different passage entirely" },
		"locator changes": func(p *store.ProposalRow) { p.Proposed.Citations[0].Locator = "§ 1950.5(h)" },
		"source dropped":  func(p *store.ProposalRow) { p.Proposed.Citations = p.Proposed.Citations[:1] },
		"source added": func(p *store.ProposalRow) {
			p.Proposed.Citations = append(p.Proposed.Citations, store.ProposedCitation{URL: "https://x.example.gov", Quote: "x"})
		},
		"editorial added": func(p *store.ProposalRow) {
			p.Proposed.Citations = append(p.Proposed.Citations, store.ProposedCitation{Editorial: true})
		},
		"nothing changes": func(p *store.ProposalRow) { p.Proposed.Citations[0].Quote = "within 30 days" },
		"no replacement":  func(p *store.ProposalRow) { p.Proposed = nil },
		"over the widen cap": func(p *store.ProposalRow) {
			p.Proposed.Citations[0].Quote = "within 30 days " + strings.Repeat("word ", widenCap)
		},
	}
	for name, mutate := range cases {
		p, cur := widenFixture()
		mutate(&p)
		if changed, why := widenVerdict(p, cur); why == "" {
			t.Errorf("%s: rule decided it (%d changed); it must leave this for a person", name, len(changed))
		}
	}
}

func TestWidenVerdict_toleratesWhitespaceAndTypography(t *testing.T) {
	p, cur := widenFixture()
	cur.Citations[0].Quote = "the tenant’s deposit"
	p.Proposed.Citations[0].Quote = "(g) The landlord returns the tenant's\n  deposit within 30 days."
	if _, why := widenVerdict(p, cur); why != "" {
		t.Errorf("a curly apostrophe or a line break is not a different quote: %q", why)
	}
}
