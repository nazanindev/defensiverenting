package store_test

import (
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

func TestClaimKindOf(t *testing.T) {
	cases := map[string]string{
		"Your landlord has 30 days after you move out to return your deposit.": store.ClaimLegalRule,
		"Your landlord cannot keep your deposit for normal wear.":              store.ClaimLegalRule,
		"The fee is capped at $50.":                                            store.ClaimLegalRule,
		"File a complaint with the housing department.":                        store.ClaimProcedure,
		"Keep a copy of every letter you send.":                                store.ClaimPractical,
	}
	for body, want := range cases {
		if got := store.ClaimKindOf(body); got != want {
			t.Errorf("%q: got %s, want %s", body, got, want)
		}
	}
}

func TestSourceTier(t *testing.T) {
	if store.SourceTier("statute") != 1 || store.SourceTier("court_ruling") != 2 || store.SourceTier("gov_guidance") != 3 || store.SourceTier("nonprofit") != 4 {
		t.Fatal("tiers do not match ADR-027 D3")
	}
	if store.SourceTier("statute") >= store.SourceTier("unknown") {
		t.Fatal("an unknown kind must be weaker than every known one")
	}
}

func codes(checks []store.QualityCheck) map[string]int {
	m := map[string]int{}
	for _, c := range checks {
		m[c.Code] = c.Severity
	}
	return m
}

// A legal rule carried by a nonprofit page alone is the red check; the same
// rule with a statute beside it passes, because a statement is judged by its
// best citation.
func TestPageQualityChecksSourceTier(t *testing.T) {
	statute := store.QualityCitation{SourceKind: "statute", Locator: "§ 5321.16", Quote: "within thirty days", Checked: true}
	nonprofit := store.QualityCitation{SourceKind: "nonprofit", Quote: "30 days", Checked: true}
	rule := "Your landlord has 30 days after you move out to return your deposit."

	got := codes(store.PageQualityChecks(store.QualityInput{Language: "en", PageKind: "rules",
		Statements: []store.QualityStatement{{Body: rule, Citations: []store.QualityCitation{nonprofit}}}}))
	if got["nonprofit-alone"] != store.SeverityRed {
		t.Errorf("nonprofit alone: got %v", got)
	}
	got = codes(store.PageQualityChecks(store.QualityInput{Language: "en", PageKind: "rules",
		Statements: []store.QualityStatement{{Body: rule, Citations: []store.QualityCitation{nonprofit, statute}}}}))
	if _, bad := got["nonprofit-alone"]; bad {
		t.Errorf("a statute beside the nonprofit must pass: got %v", got)
	}
	if _, bad := got["below-tier"]; bad {
		t.Errorf("best citation is the law: got %v", got)
	}
	// Government guidance carries a procedure, not a legal rule.
	guidance := store.QualityCitation{SourceKind: "gov_guidance", Quote: "call 311", Checked: true}
	got = codes(store.PageQualityChecks(store.QualityInput{Language: "en", PageKind: "rules",
		Statements: []store.QualityStatement{
			{Body: "Call 311 to ask the housing department for an inspection.", Citations: []store.QualityCitation{guidance}},
			{Body: rule, Citations: []store.QualityCitation{guidance}},
		}}))
	if got["below-tier"] != store.SeverityRed {
		t.Errorf("guidance under a legal rule is red, and a procedure on guidance passes: got %v", got)
	}
}

// The citation signals: archive snapshot, browser-only, statute without a
// section, quote nobody checked.
func TestPageQualityChecksCitationSignals(t *testing.T) {
	got := codes(store.PageQualityChecks(store.QualityInput{Language: "en", PageKind: "rules",
		Statements: []store.QualityStatement{
			{Body: "Keep a copy of every letter you send.", Citations: []store.QualityCitation{
				{SourceKind: "gov_guidance", URL: "https://web.archive.org/web/2025/https://example.gov", Quote: "keep copies", Checked: true},
			}},
			{Body: "Take photos of the home before you leave.", Citations: []store.QualityCitation{
				{SourceKind: "statute", Locator: "March 2026", Quote: "photos", Checked: false, CheckedVia: "render"},
			}},
		}}))
	for _, want := range []string{"archive-url", "browser-tier-only", "no-locator", "unchecked"} {
		if got[want] != store.SeverityYellow {
			t.Errorf("%s: got %v", want, got)
		}
	}
}

// Actionability runs on situation pages only, and reads the page as a whole:
// one step and one place to go anywhere on it is enough.
func TestPageQualityChecksActionability(t *testing.T) {
	statute := store.QualityCitation{SourceKind: "statute", Locator: "§ 1", Quote: "the landlord shall return the deposit within 30 days", Checked: true}
	bare := store.QualityInput{Language: "en", PageKind: "playbook", Warns: map[string]bool{},
		Statements: []store.QualityStatement{{Body: "Your landlord must return your deposit.", Citations: []store.QualityCitation{statute}}}}
	got := codes(store.PageQualityChecks(bare))
	for _, want := range []string{"first-step", "somewhere-to-go", "deadline-number"} {
		if got[want] != store.SeverityOrange {
			t.Errorf("%s: got %v", want, got)
		}
	}
	full := store.QualityInput{Language: "en", PageKind: "playbook", Warns: map[string]bool{},
		Statements: []store.QualityStatement{
			{Body: "Your landlord has 30 days after you move out to return your deposit.", Citations: []store.QualityCitation{statute}},
			{Body: "Write to your landlord and ask for it back. Legal aid can help you for free.", Citations: []store.QualityCitation{{SourceKind: "editorial"}}},
		}}
	got = codes(store.PageQualityChecks(full))
	for _, bad := range []string{"first-step", "somewhere-to-go", "deadline-number"} {
		if _, ok := got[bad]; ok {
			t.Errorf("%s should pass: got %v", bad, got)
		}
	}
	// A step after its condition still counts as a step.
	after := store.QualityInput{Language: "en", PageKind: "playbook", Warns: map[string]bool{},
		Statements: []store.QualityStatement{{Body: "If your landlord does not fix it in 30 days after your letter, tell the court. Legal aid can help.", Citations: []store.QualityCitation{statute}}}}
	if got := codes(store.PageQualityChecks(after)); got["first-step"] != 0 {
		t.Errorf("a step after a condition should pass: got %v", got)
	}
	// The same bare page as a rules page is not held to actionability.
	bare.PageKind = "rules"
	got = codes(store.PageQualityChecks(bare))
	for _, bad := range []string{"first-step", "somewhere-to-go", "deadline-number"} {
		if _, ok := got[bad]; ok {
			t.Errorf("rules page %s: got %v", bad, got)
		}
	}
}

// A risky step without its warning fails, unless the page says the risk
// once at the top (ADR-016 A3).
func TestPageQualityChecksRisksPaired(t *testing.T) {
	statute := store.QualityCitation{SourceKind: "statute", Locator: "§ 1", Quote: "may withhold", Checked: true}
	in := store.QualityInput{Language: "en", PageKind: "playbook", Warns: map[string]bool{},
		Statements: []store.QualityStatement{{Body: "You can withhold rent until the heat is fixed.", Citations: []store.QualityCitation{statute}}}}
	if got := codes(store.PageQualityChecks(in)); got["risks-paired"] != store.SeverityOrange {
		t.Errorf("unwarned risky step: got %v", got)
	}
	in.Warns["evict"] = true
	if got := codes(store.PageQualityChecks(in)); got["risks-paired"] != 0 {
		t.Errorf("page warns once at the top: got %v", got)
	}
}

// Worst first: the order the map colors by.
func TestPageQualityChecksOrder(t *testing.T) {
	out := store.PageQualityChecks(store.QualityInput{Language: "en", PageKind: "playbook", Warns: map[string]bool{},
		Statements: []store.QualityStatement{
			{Body: "Write to your landlord. Legal aid can help.", Citations: []store.QualityCitation{{SourceKind: "statute", Locator: "", Quote: "x", Checked: true}}},
			{Body: "Your landlord cannot keep your deposit for normal wear.", Citations: []store.QualityCitation{{SourceKind: "nonprofit", Quote: "x", Checked: true}}},
		}})
	if len(out) < 2 || out[0].Code != "nonprofit-alone" {
		t.Fatalf("worst first, got %+v", out)
	}
	for i := 1; i < len(out); i++ {
		if out[i].Severity > out[i-1].Severity {
			t.Fatalf("not ordered by severity: %+v", out)
		}
	}
}
