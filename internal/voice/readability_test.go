package voice

import (
	"strings"
	"testing"
)

func TestReadingScore_densePassagesScoreHigher(t *testing.T) {
	plain := "Your landlord must give back your deposit within 30 days. Ask in writing if it does not come."
	dense := "Notwithstanding contractual stipulations, the obligation to reimburse the security deposit arises contemporaneously with termination and relinquishment of occupancy."
	if s := ReadingScore(plain); s >= targetScore {
		t.Errorf("plain text scored %.1f, want under %.1f", s, targetScore)
	}
	if s := ReadingScore(dense); s < MaxStatementScore {
		t.Errorf("dense text scored %.1f, want %.1f or more", s, MaxStatementScore)
	}
}

func TestUnfamiliarWords(t *testing.T) {
	got := UnfamiliarWords("Your landlord's escrow account must hold the rent. Everything else, like the refrigerator, is ordinary. See hud.gov/findacounselor or Philadelphia.")
	if strings.Join(got, ",") != "escrow" {
		t.Errorf("UnfamiliarWords = %v, want [escrow]", got)
	}
	if got := UnfamiliarWords("The sheriff carries out a writ (the court order to return the home to your landlord)."); len(got) != 0 {
		t.Errorf("a glossed official term should pass, got %v", got)
	}
	if got := UnfamiliarWords("The clerk issues a writ of possession (the court order that lets the sheriff remove you)."); len(got) != 0 {
		t.Errorf("a gloss after a short phrase should pass, got %v", got)
	}
	if got := UnfamiliarWords("The clerk issues a writ and then the sheriff comes to your home (after 24 hours)."); len(got) == 0 {
		t.Error("a parenthesis far from the word is not its gloss")
	}
}

func TestLintAll_readability(t *testing.T) {
	v := LintAll("en", map[string]string{"body_md": "Your landlord must hold the money in escrow."})
	if !strings.Contains(strings.Join(v, " "), "escrow") {
		t.Errorf("unfamiliar word not flagged: %v", v)
	}
	if v := LintAll("en", map[string]string{"intro_md": "This page covers escrow rules."}); len(v) != 0 {
		t.Errorf("readability rules are for statements only, got %v", v)
	}
	if v := LintAll("en", map[string]string{"body_md": "Your landlord cannot shut off your utilities."}); !strings.Contains(strings.Join(v, " "), "utilities") {
		t.Errorf("unglossed utilities not flagged: %v", v)
	}
}

func TestHarderThan(t *testing.T) {
	old := "Your landlord must fix the heat. Tell your landlord in writing."
	if why := HarderThan("en", old, "Your landlord must fix the heat. Tell your landlord in writing and keep a copy."); why != "" {
		t.Errorf("a plain edit was refused: %s", why)
	}
	if why := HarderThan("en", old, "Your landlord must fix the heat. Put the rent in escrow."); !strings.Contains(why, "escrow") {
		t.Errorf("an added unfamiliar word passed: %q", why)
	}
}
