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
		if got := Lint("en", s); len(got) != 0 {
			t.Errorf("Lint(en, %q) = %v, want none", s, got)
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
		got := Lint("en", c.text)
		found := false
		for _, v := range got {
			if strings.Contains(v, c.want) {
				found = true
			}
		}
		if !found {
			t.Errorf("Lint(en, %q) = %v, want a violation mentioning %q", c.text, got, c.want)
		}
	}
}

func TestLint_longSentence(t *testing.T) {
	long := strings.Repeat("word ", 26) + "end."
	if got := Lint("en", long); len(got) == 0 {
		t.Errorf("Lint(en, long sentence) = none, want length violation")
	}
	ok := strings.Repeat("word ", 20) + "end. " + strings.Repeat("word ", 20) + "end."
	if got := Lint("en", ok); len(got) != 0 {
		t.Errorf("Lint(en, two short sentences) = %v, want none", got)
	}
}

func TestLintAll_labelsAndCap(t *testing.T) {
	got := LintAll("en", map[string]string{"intro": "This is void.", "statement 2": "You waive it."})
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
	if got := LintAll("en", many); len(got) != 11 { // 10 + overflow marker
		t.Errorf("LintAll cap = %d messages, want 11", len(got))
	}
}

func TestLint_allowedTerms(t *testing.T) {
	ok := `Ask the court to let you skip the fees. The court form for this is called a "fee waiver".`
	if got := Lint("en", ok); len(got) != 0 {
		t.Errorf("Lint(en, fee waiver form name) = %v, want none", got)
	}
	if got := Lint("en", "You waive this right."); len(got) == 0 {
		t.Error("plain 'waive' must still be banned")
	}
}

func TestLint_spanish_cleanTextPasses(t *testing.T) {
	clean := []string{
		"Su arrendador debe devolver el depósito en 14 días. Si no lo hace, usted puede demandar.",
		"El cargo tardío no puede pasar del 5% de la renta. Si la renta es $1,000, eso es $50 como máximo.",
	}
	for _, s := range clean {
		if got := Lint("es", s); len(got) != 0 {
			t.Errorf("Lint(es, %q) = %v, want none", s, got)
		}
	}
}

func TestLint_spanish_violations(t *testing.T) {
	cases := []struct {
		text string
		want string
	}{
		{"Esa cláusula es nulo.", "nulo"},
		{"Usted renuncia a este derecho.", "renunci"},
		{"El alquiler vence — páguelo.", "dash"},
		{"Debe responder en siete días.", "digits"},
		{"El cargo es del 10% de la renta.", "dollar example"},
		{"No obstante, usted debe pagar.", "obstante"},
	}
	for _, c := range cases {
		got := Lint("es", c.text)
		found := false
		for _, v := range got {
			if strings.Contains(v, c.want) {
				found = true
			}
		}
		if !found {
			t.Errorf("Lint(es, %q) = %v, want a violation mentioning %q", c.text, got, c.want)
		}
	}
}

// English-only jargon must not fire on Spanish text (and vice versa) — the
// rulesets are language-specific, not merged.
func TestLint_languagesDoNotCrossContaminate(t *testing.T) {
	if got := Lint("es", "You waive this right."); len(got) != 0 {
		t.Errorf("Lint(es, English text) = %v, want none — English banned words shouldn't fire under the es ruleset", got)
	}
	if got := Lint("en", "Usted renuncia a este derecho."); len(got) != 0 {
		t.Errorf("Lint(en, Spanish text) = %v, want none — Spanish banned words shouldn't fire under the en ruleset", got)
	}
}

func TestSupported(t *testing.T) {
	got := Supported()
	want := map[string]bool{"en": true, "es": true}
	if len(got) != len(want) {
		t.Fatalf("Supported() = %v, want %d languages", got, len(want))
	}
	for _, l := range got {
		if !want[l] {
			t.Errorf("Supported() includes unexpected language %q", l)
		}
	}
}

// The money-example and deadline-pile rules added 2026-08-21: multiplied
// money must carry a worked dollar example, and 3+ time periods in one block
// need ordering words or a split.
func TestLint_moneyMultiplierAndDeadlinePile(t *testing.T) {
	if v := Lint("en", "The landlord owes you 3 times the deposit."); len(v) != 1 || !strings.Contains(v[0], "worked dollar example") {
		t.Errorf("bare multiplier must require an example, got %v", v)
	}
	if v := Lint("en", "The landlord owes you 3 times the deposit. On a $1,500 deposit that is $4,500."); len(v) != 0 {
		t.Errorf("multiplier with an example must pass, got %v", v)
	}
	if v := Lint("en", "You get a 5 day notice. You have 10 days to respond. The appeal takes 30 days."); len(v) != 1 || !strings.Contains(v[0], "ordering words") {
		t.Errorf("3 unordered time periods must be flagged, got %v", v)
	}
	if v := Lint("en", "First you get a 5 day notice. Then you have 10 days to respond. After that the appeal takes 30 days."); len(v) != 0 {
		t.Errorf("an ordered sequence must pass, got %v", v)
	}
}

// New banned legalese, and the habitability allowance: the doctrine may be
// named once, but bare "habitable" is jargon.
func TestLint_newBannedWords(t *testing.T) {
	for _, bad := range []string{
		"The landlord shall fix it.",
		"You must vacate the premises.",
		"Your landlord may terminate the lease.",
		"The home must be habitable.",
	} {
		if v := Lint("en", bad); len(v) == 0 {
			t.Errorf("expected a violation for %q", bad)
		}
	}
	if v := Lint("en", "Lawyers call this the warranty of habitability. It means your home must be fit to live in."); len(v) != 0 {
		t.Errorf("naming the warranty of habitability must stay legal, got %v", v)
	}
}

// The jargon bans added 2026-08-26: damages, eligibility, collections,
// partial payment, "period of".
func TestLint_jargonBans2026_08(t *testing.T) {
	for _, bad := range []string{
		"The landlord can deduct for damages.",
		"You may be eligible for help.",
		"The debt can be sent to collections.",
		"Your landlord may refuse a partial payment.",
		"You have a period of 30 days to respond.",
	} {
		if v := Lint("en", bad); len(v) == 0 {
			t.Errorf("expected a violation for %q", bad)
		}
	}
	// The harm-to-the-home sense is "damage", no s, and stays legal.
	if v := Lint("en", "You do not pay for damage you did not cause."); len(v) != 0 {
		t.Errorf("singular 'damage' must pass, got %v", v)
	}
	if v := Lint("es", "El propietario debe pagar daños y perjuicios."); len(v) == 0 {
		t.Error("expected a violation for 'daños y perjuicios'")
	}
	if v := Lint("es", "Usted puede ser elegible para esta ayuda."); len(v) == 0 {
		t.Error("expected a violation for 'elegible'")
	}
}

// Official terms (judgment, mediation, rental assistance, wear and tear,
// harassment, grace period) may be named, but the same text block must
// explain them: a parenthetical right after, or "<term> means ...".
func TestLint_explainRequired(t *testing.T) {
	bare := []string{
		"The court can enter a judgment against you.",
		"You can ask the court for mediation.",
		"Apply for rental assistance today.",
		"Your landlord cannot charge you for normal wear and tear.",
		"This counts as harassment.",
		"Your lease may give you a grace period.",
	}
	for _, s := range bare {
		v := Lint("en", s)
		found := false
		for _, msg := range v {
			if strings.Contains(msg, "plain-words explanation") {
				found = true
			}
		}
		if !found {
			t.Errorf("Lint(en, %q) = %v, want an explain-required violation", s, v)
		}
	}

	explained := []string{
		"The court can enter a judgment (its final decision in your case) against you.",
		"Mediation means a meeting with a neutral person. You can ask the court for it.",
		"Apply for rental assistance (money to help pay rent) today.",
		"Normal wear and tear (normal use over time, like faded paint) is not damage you pay for.",
		"Harassment means repeated pressure to make you move out. Write down each time it happens.",
		"A grace period (extra days to pay before late fees start) is not required by law.",
	}
	for _, s := range explained {
		if v := Lint("en", s); len(v) != 0 {
			t.Errorf("Lint(en, %q) = %v, want none", s, v)
		}
	}
}

func TestLint_explainRequired_spanish(t *testing.T) {
	if v := Lint("es", "Puede pedir mediación al tribunal."); len(v) == 0 {
		t.Error("bare 'mediación' must be flagged")
	}
	ok := "Puede pedir mediación (una reunión con una persona neutral que ayuda a llegar a un acuerdo)."
	if v := Lint("es", ok); len(v) != 0 {
		t.Errorf("explained 'mediación' must pass, got %v", v)
	}
	if v := Lint("es", "El desgaste normal no es su responsabilidad."); len(v) == 0 {
		t.Error("bare 'desgaste normal' must be flagged")
	}
	if v := Lint("es", "Usted puede pedir asistencia de renta hoy."); len(v) == 0 {
		t.Error("bare 'asistencia de renta' must be flagged")
	}
}
