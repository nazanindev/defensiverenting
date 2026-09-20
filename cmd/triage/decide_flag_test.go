package main

import (
	"strings"
	"testing"
)

func TestFlagVerdict_standsOnlyWithAVerbatimPassage(t *testing.T) {
	srcs := []flagSource{{URL: "https://law.example.gov", Text: "(g) The landlord shall, within 30 days after the tenant vacates, return the deposit.\n(h) Interest accrues."}}
	ok, why := flagVerdict(flagDecision{Verdict: "stands", Passage: "within 30 days after the tenant vacates, return the deposit", Reason: "The section sets 30 days."}, srcs)
	if !ok {
		t.Fatalf("verbatim passage refused: %s", why)
	}
	ok, _ = flagVerdict(flagDecision{Verdict: "stands", Passage: "within  30 days after the tenant’s vacating", Reason: "r"}, srcs)
	if ok {
		t.Error("a paraphrase passed as verbatim")
	}
	cases := map[string]flagDecision{
		"leave":             {Verdict: "leave", Reason: "not covered"},
		"stands, no quote":  {Verdict: "stands", Reason: "r"},
		"stands, no reason": {Verdict: "stands", Passage: "Interest accrues."},
		"too long":          {Verdict: "stands", Reason: "r", Passage: strings.Repeat("Interest accrues. ", flagPassageCap)},
		"from nowhere":      {Verdict: "stands", Reason: "r", Passage: "the tenant may withhold rent"},
		"unknown verdict":   {Verdict: "maybe", Reason: "r", Passage: "Interest accrues."},
	}
	for name, d := range cases {
		if ok, _ := flagVerdict(d, srcs); ok {
			t.Errorf("%s: accepted as stands", name)
		}
	}
	// A source with no text cannot host a passage.
	if ok, _ := flagVerdict(flagDecision{Verdict: "stands", Reason: "r", Passage: "Interest accrues."}, []flagSource{{Err: "blocked"}}); ok {
		t.Error("stands accepted against a source that was never read")
	}
}
