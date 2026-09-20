package main

import (
	"context"
	"strings"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/drafting"
	"github.com/nazanindev/defensiverenting/internal/store"
)

func editFixture() (store.ProposalRow, drafting.QuoteCheck) {
	p := store.ProposalRow{Position: 2, Language: "en"}
	p.Proposed = &store.ProposedStatement{
		BodyMD:    "The landlord returns the deposit within 30 days.",
		Citations: []store.ProposedCitation{{URL: "https://law.example.gov/s1950", Locator: "§ 1950.5(g)", Quote: "within 30 days", Kind: "statute"}},
	}
	check := func(_ context.Context, _ int, url, quote string) drafting.QuoteVerdict {
		switch {
		case strings.Contains(url, "blocked"):
			return drafting.QuoteVerdict{Overridable: true}
		case quote == "within 30 days":
			return drafting.QuoteVerdict{Verified: true}
		}
		return drafting.QuoteVerdict{}
	}
	return p, check
}

func TestEditVerdict_appliesABackedEdit(t *testing.T) {
	p, check := editFixture()
	if why := editVerdict(context.Background(), editDecision{Verdict: "apply", Reason: "matches the note"}, p, check); why != "" {
		t.Fatalf("backed edit refused: %s", why)
	}
}

func TestEditVerdict_holdsTheLine(t *testing.T) {
	cases := map[string]func(p *store.ProposalRow, d *editDecision){
		"leave":           func(_ *store.ProposalRow, d *editDecision) { d.Verdict = "leave" },
		"no reason":       func(_ *store.ProposalRow, d *editDecision) { d.Reason = "" },
		"quote missing":   func(p *store.ProposalRow, _ *editDecision) { p.Proposed.Citations[0].Quote = "" },
		"quote not there": func(p *store.ProposalRow, _ *editDecision) { p.Proposed.Citations[0].Quote = "within 60 days" },
		"source blocked": func(p *store.ProposalRow, _ *editDecision) {
			p.Proposed.Citations[0].URL = "https://blocked.example.gov"
		},
		"reference-only": func(p *store.ProposalRow, _ *editDecision) {
			p.Proposed.Citations[0].URL = "https://www.nolo.com/legal-encyclopedia/deposit"
		},
		"empty body": func(p *store.ProposalRow, _ *editDecision) { p.Proposed.BodyMD = " " },
		"voice lint": func(p *store.ProposalRow, _ *editDecision) {
			p.Proposed.BodyMD = "The landlord — who must return the deposit — has 30 days."
		},
		"cites nothing":  func(p *store.ProposalRow, _ *editDecision) { p.Proposed.Citations = nil },
		"no replacement": func(p *store.ProposalRow, _ *editDecision) { p.Proposed = nil },
	}
	for name, mutate := range cases {
		p, check := editFixture()
		d := editDecision{Verdict: "apply", Reason: "r"}
		mutate(&p, &d)
		if why := editVerdict(context.Background(), d, p, check); why == "" {
			t.Errorf("%s: applied; it must stay for a person", name)
		}
	}
}
