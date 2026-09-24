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
		"Your landlord cannot enter except in an emergency.", // plain "except" stays
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
		{"New buildings are exempt from the cap.", "does not apply to"},
		{"There is an exemption for small landlords.", "does not apply to"},
		{"There are 2 exceptions to this rule.", "does not apply to"},
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
// harassment, grace period, presumption) may be named, but the same text block must
// explain them: a parenthetical right after, or "<term> means ...".
func TestLint_explainRequired(t *testing.T) {
	bare := []string{
		"The court can enter a judgment against you.",
		"You can ask the court for mediation.",
		"Apply for rental assistance today.",
		"Your landlord cannot charge you for normal wear and tear.",
		"This counts as harassment.",
		"Your lease may give you a grace period.",
		"The law presumes the eviction is retaliation.",
		"There is a presumption of retaliation.",
		"The eviction is presumed not to be retaliation.",
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
		"Normal wear and tear (normal use over time) is not damage you pay for.",
		"Harassment means repeated pressure to make you move out. Write down each time it happens.",
		"A grace period (extra days to pay before late fees start) is not required by law.",
		"The eviction is presumed (the court treats it as true unless your landlord proves it is not) to be retaliation.",
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

// Ending the lease, moving out and stopping rent, and withholding rent are
// judged by a court only afterwards, so the statement must say the risk.
func TestLint_riskyStepNeedsWarning(t *testing.T) {
	bare := []string{
		"If the landlord does not fix it, you can end your lease and move out.",
		"If the problem is serious, you may stop paying rent until it is fixed.",
		"You can move out and stop paying rent from that day.",
		"This is called rent withholding.",
		"You may move out and stop paying rent. Courts call this constructive eviction.",
	}
	for _, s := range bare {
		found := false
		for _, v := range LintAll("en", map[string]string{"body_md": s}) {
			if strings.Contains(v, "court judges only afterwards") {
				found = true
			}
		}
		if !found {
			t.Errorf("LintAll(en, body_md %q) wants a risk warning", s)
		}
	}
	warned := []string{
		"You can end your lease and move out. If a court later disagrees, you can owe the rent. Get legal help first.",
		"You may withhold rent, but your landlord can try to evict you.",
		"Your landlord cannot end your lease for complaining.",
		"The Rent Withholding Act covers cities only, like Philadelphia and Pittsburgh.",
	}
	for _, s := range warned {
		for _, v := range LintAll("en", map[string]string{"body_md": s}) {
			if strings.Contains(v, "court judges only afterwards") {
				t.Errorf("LintAll(en, body_md %q) = %v, want no risk violation", s, v)
			}
		}
	}
	intro := map[string]string{"intro_md": "This guide covers when you can end your lease early."}
	for _, v := range LintAll("en", intro) {
		if strings.Contains(v, "court judges only afterwards") {
			t.Errorf("an intro naming the step must not need the warning, got %v", v)
		}
	}
}

// Calling the inspector is the right step, and for very bad conditions it can
// end with the home condemned and everyone ordered out. The statement that
// sends the renter there says so (editorial guidance, 2026-09-22).
func TestLint_inspectionNeedsCondemnationWarning(t *testing.T) {
	has := func(s string) bool {
		for _, v := range LintAll("en", map[string]string{"body_md": s}) {
			if strings.Contains(v, "sends the renter to an inspector") {
				return true
			}
		}
		return false
	}
	if !has("If your landlord does not fix it, call 311 and ask for an inspection.") || !has("Call the code office to report the problem.") {
		t.Error("a call-the-inspector statement with no warning must be flagged")
	}
	if has("Call 311 and ask for an inspection. For very bad conditions, a code office can condemn the home and make everyone leave.") {
		t.Error("a statement carrying the warning must pass")
	}
	if has("For free legal advice, call 311 and ask for the Tenant Helpline.") {
		t.Error("311 used for something other than an inspection must pass")
	}
	if has("Ask your landlord for an initial inspection before you move out.") {
		t.Error("a landlord's move-out deposit inspection is not a code inspection")
	}
	if has("Before you move out, you can ask for a walk through inspection.") {
		t.Error("a walk through inspection, spelled as two words, is a deposit inspection")
	}
	if has("The inspector writes a report after the visit.") {
		t.Error("mentioning an inspector without sending the renter there must pass")
	}
	if v := LintAll("en", map[string]string{"intro_md": "This guide explains when to call the code office for an inspection."}); len(v) > 0 {
		for _, x := range v {
			if strings.Contains(x, "sends the renter to an inspector") {
				t.Error("intros are exempt")
			}
		}
	}
}

// The site never tells a renter to call the police: for some renters police
// make things worse, and the call is theirs to judge. It may offer the police
// as a choice beside a route without them. Immediate danger is set aside.
func TestLint_policeIsTheRentersChoice(t *testing.T) {
	flagged := func(s string) bool {
		for _, v := range LintAll("en", map[string]string{"body_md": s}) {
			if strings.Contains(v, "police") {
				return true
			}
		}
		return false
	}
	for _, s := range []string{
		"If your landlord locks you out, call the police.",
		"Your landlord cannot change the locks. Contact the police and ask for a report.",
		"A police report can help prove an illegal lockout.",
	} {
		if !flagged(s) {
			t.Errorf("want a violation for %q", s)
		}
	}
	for _, s := range []string{
		"If you feel safe doing so, you can ask the police to write a report. A report can be evidence. You can also take photos and call legal aid.",
		"If you are in danger, call 911. Then write down what happened and call legal aid.",
		"Your landlord cannot lock you out without a court order.",
	} {
		if flagged(s) {
			t.Errorf("want no police violation for %q", s)
		}
	}
}

// Money a court or agency awards against the landlord arrives only if the
// renter wins and the landlord pays; a renter reading "you can get 2 times
// your deposit" should not think it simply arrives (2026-09-22).
func TestLint_awardSaysYouMustWinAndBePaid(t *testing.T) {
	flagged := func(s string) bool {
		for _, v := range LintAll("en", map[string]string{"body_md": s}) {
			if strings.Contains(v, "money a court or agency awards") {
				return true
			}
		}
		return false
	}
	for _, s := range []string{
		"If your landlord keeps your deposit, you can get 2 times the deposit. For a $1,000 deposit that is $2,000.",
		"The landlord can owe you up to $2,000 for an illegal lockout.",
		"You can sue in small claims court to get the money back.",
	} {
		if !flagged(s) {
			t.Errorf("want a violation for %q", s)
		}
	}
	for _, s := range []string{
		"If the court finds your landlord kept your deposit in bad faith, you can get 2 times the deposit. For a $1,000 deposit that is $2,000. You get this money only if you win your case and your landlord pays.",
		"Your landlord must return your deposit within 30 days.",
		"Your landlord must pay you interest on the deposit each year.",
		"If you skip your last month's rent, a court can order you to pay 3 times the rent withheld.",
		"The city's rental assistance program helps with a crisis. You may qualify for up to $6,000 toward back rent.",
		"Your lease may set a late penalty. You can get the lease terms in writing.",
	} {
		if flagged(s) {
			t.Errorf("want no award violation for %q", s)
		}
	}
}
