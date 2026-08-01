package voice

import (
	"strings"
	"testing"
)

func TestLint_cleanTextPasses(t *testing.T) {
	clean := []string{
		"Your landlord must return your deposit within 14 days. If they do not, you can sue.",
		"The late fee is capped at 5% of your rent. If your rent is $1,000, that is $50 at most.",
		"A notice to quit (a letter saying you must move out) starts the clock.",
		"Avoid paying cash without a receipt.", // "avoid" must not trip the \bvoid\b rule
	}
	for _, s := range clean {
		if got := Lint(s); len(got) != 0 {
			t.Errorf("Lint(%q) = %v, want none", s, got)
		}
	}
}

func TestLint_violations(t *testing.T) {
	cases := []struct {
		text string
		want string // substring expected in a violation message
	}{
		{"This clause is void.", "void"},
		{"That waiver is unenforceable.", "unenforceable"},
		{"You waive this right.", "give up"},
		{"You have remedies.", "remed"},
		{"Pursuant to the lease provision.", "Pursuant"},
		{"The rent is due — pay it.", "dash"},
		{"Pay within 5–7 days.", "dash"},
		{"You must respond within seven days.", "digits"},
		{"The fee is capped at 10% of rent.", "dollar example"},
		{"This gives you a mental model of eviction.", "mental model"},
		{"Navigate the process carefully.", "Navigate"},
	}
	for _, c := range cases {
		got := Lint(c.text)
		found := false
		for _, v := range got {
			if strings.Contains(v, c.want) {
				found = true
			}
		}
		if !found {
			t.Errorf("Lint(%q) = %v, want a violation mentioning %q", c.text, got, c.want)
		}
	}
}

func TestLint_longSentence(t *testing.T) {
	long := strings.Repeat("word ", 26) + "end."
	if got := Lint(long); len(got) == 0 {
		t.Errorf("Lint(long sentence) = none, want length violation")
	}
	ok := strings.Repeat("word ", 20) + "end. " + strings.Repeat("word ", 20) + "end."
	if got := Lint(ok); len(got) != 0 {
		t.Errorf("Lint(two short sentences) = %v, want none", got)
	}
}

func TestLintAll_labelsAndCap(t *testing.T) {
	got := LintAll(map[string]string{"intro": "This is void.", "statement 2": "You waive it."})
	if len(got) != 2 {
		t.Fatalf("LintAll = %v, want 2 violations", got)
	}
	if !strings.HasPrefix(got[0], "intro:") || !strings.HasPrefix(got[1], "statement 2:") {
		t.Errorf("LintAll labels wrong: %v", got)
	}

	many := map[string]string{}
	for i := 0; i < 15; i++ {
		many[strings.Repeat("s", i+1)] = "void — waive remedies pursuant"
	}
	if got := LintAll(many); len(got) != 11 { // 10 + overflow marker
		t.Errorf("LintAll cap = %d messages, want 11", len(got))
	}
}

func TestLint_allowedTerms(t *testing.T) {
	ok := `Ask the court to let you skip the fees. The court form for this is called a "fee waiver".`
	if got := Lint(ok); len(got) != 0 {
		t.Errorf("Lint(fee waiver form name) = %v, want none", got)
	}
	if got := Lint("You waive this right."); len(got) == 0 {
		t.Error("plain 'waive' must still be banned")
	}
}
