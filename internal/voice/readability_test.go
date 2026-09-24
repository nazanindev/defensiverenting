package voice

import (
	"strings"
	"testing"
)

func TestReadingGrade_densePasagesScoreHigher(t *testing.T) {
	plain := "Your landlord must give back your deposit within 30 days. Ask in writing if it does not come."
	dense := "Notwithstanding contractual stipulations, the landlord's obligation to reimburse the tenant's security deposit arises contemporaneously with termination and relinquishment of occupancy."
	if g := ReadingGrade(plain); g > 6 {
		t.Errorf("plain text graded %.1f, want 6 or less", g)
	}
	if g := ReadingGrade(dense); g <= MaxStatementGrade {
		t.Errorf("dense text graded %.1f, want over %.0f", g, MaxStatementGrade)
	}
}

func TestHardWords(t *testing.T) {
	got := HardWords("The court may enter a writ of restitution and find the fee unconscionable. Everything else is ordinary. See hud.gov/findacounselor or Philadelphia.")
	if strings.Join(got, ",") != "restitution,unconscionable" {
		t.Errorf("HardWords = %v, want [restitution unconscionable]", got)
	}
	if got := HardWords("The sheriff carries out a writ of restitution (the order to return the home to your landlord)."); len(got) != 0 {
		t.Errorf("a glossed official term should pass, got %v", got)
	}
}

func TestLintAll_readability(t *testing.T) {
	v := LintAll("en", map[string]string{"body_md": "Your landlord cannot charge an unconscionable fee."})
	if len(v) == 0 || !strings.Contains(strings.Join(v, " "), "unconscionable") {
		t.Errorf("hard word not flagged: %v", v)
	}
	if v := LintAll("en", map[string]string{"intro_md": "This page covers occupancy rules."}); len(v) != 0 {
		t.Errorf("readability rules are for statements only, got %v", v)
	}
}

func TestHarderThan(t *testing.T) {
	old := "Your landlord must fix the heat. Tell your landlord in writing."
	if why := HarderThan("en", old, "Your landlord must fix the heat. Tell your landlord in writing and keep a copy."); why != "" {
		t.Errorf("a plain edit was refused: %s", why)
	}
	if why := HarderThan("en", old, "Your landlord bears responsibility for heating infrastructure maintenance notwithstanding contractual stipulations."); why == "" {
		t.Error("a harder edit passed")
	}
	if why := HarderThan("en", old, "Your landlord must fix the heat. Give written notice of the occupancy problem."); !strings.Contains(why, "occupancy") {
		t.Errorf("an added hard word passed: %q", why)
	}
}
