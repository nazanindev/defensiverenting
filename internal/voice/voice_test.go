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
		"Avoid paying cash without a receipt.",               // "avoid" must not trip the \bvoid\b rule
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

func TestMailRecordViolation(t *testing.T) {
	cases := []struct {
		name, text string
		flag       bool
	}{
		{"renter told certified mail only", "Send your letter by certified mail with a return receipt.", true},
		{"renter told certified mail and texts", "Send your letter by certified mail. Save texts and emails as well; they are also a record of what you sent.", false},
		{"landlord's duty", "Your landlord must mail the list by registered or certified mail within 30 days.", false},
		{"no mail", "Write down the date and keep a copy.", false},
	}
	for _, c := range cases {
		got := mailRecordViolation("en", c.text, Page{}) != ""
		if got != c.flag {
			t.Errorf("%s: flagged=%v, want %v", c.name, got, c.flag)
		}
	}
	if mailRecordViolation("es", "Envíe la carta por certified mail.", Page{}) != "" {
		t.Error("rule should run on English only")
	}
}

func TestLint_slangBanned(t *testing.T) {
	for _, txt := range []string{
		"Your landlord cannot punish you with a rent hike.",
		"Do not take your belongings without your written OK.",
		"If your fridge breaks, tell your landlord.",
		"Your landlord cannot kick you out without a court order.",
	} {
		if len(Lint("en", txt)) == 0 {
			t.Errorf("slang passed: %q", txt)
		}
	}
	if v := Lint("en", "Your landlord must fix a broken refrigerator."); len(v) != 0 {
		t.Errorf("plain text flagged: %v", v)
	}
}

func TestNoOweNeedsTheLawsuitReminder(t *testing.T) {
	bad := []string{
		"You owe no rent for the time after you move out.",
		"You do not owe future rent after you leave.",
		"The renter is released without penalty from any further rent.",
		"If you leave for abuse and follow the notice rules, you owe no early end fee.",
		"If you leave early, you may owe rent. You owe no more rent after you leave only in a few cases the law sets out.",
	}
	for _, s := range bad {
		if noOweViolation("en", s, Page{}) == "" {
			t.Errorf("no violation for %q", s)
		}
	}
	good := []string{
		"You owe no rent for the time after you move out. If your landlord sues and wins, you can still owe the money. Get legal help first.",
		"You owe no more rent after you leave. If a court later disagrees, you can owe the rent and face eviction. Get legal help first.",
		"If you leave early, you can owe rent until the home is rented again.",
	}
	for _, s := range good {
		if v := noOweViolation("en", s, Page{}); v != "" {
			t.Errorf("%q: unexpected %s", s, v)
		}
	}
}

func TestNoOweLeavesConditionsAndUnpaidRentAlone(t *testing.T) {
	for _, s := range []string{
		"If you do not pay rent on time, your landlord must send you a written notice.",
		"If you owe no rent and caused no damage, you get the whole deposit back.",
		"If you move out owing no rent, your landlord must mail you a notice.",
		"Your landlord must also give you a way to pay with no fee.",
		"You can still argue you do not owe the rent.",
	} {
		if v := noOweViolation("en", s, Page{}); v != "" {
			t.Errorf("%q: unexpected %s", s, v)
		}
	}
}

func TestNetIsBanned(t *testing.T) {
	if len(Lint("en", "The claim is reduced by the net rent.")) == 0 {
		t.Error(`"net" not flagged`)
	}
	if len(Lint("en", "Your landlord must try to rent the home again.")) != 0 {
		t.Error("clean sentence flagged")
	}
	if len(Lint("en", "Go to palegalaid.net to find help.")) != 0 {
		t.Error("web address flagged")
	}
}

func TestRiskWarningFitsTheStep(t *testing.T) {
	bad := []string{
		"If repairs are not made, you can end your lease and move out. If a court later disagrees, you can owe the rent and face eviction. Get legal help first.",
		"You may move out and stop paying rent if the home is unlivable. You could face eviction if a court disagrees.",
		"A court can find you owe no rent for the time after you move out. If a court later disagrees, you can owe the rent and face eviction. Get legal help first.",
	}
	for _, s := range bad {
		if riskViolation("en", s, Page{}) == "" {
			t.Errorf("eviction warning on a move-out step not flagged: %s", s)
		}
	}
	good := []string{
		"If repairs are not made, you can end your lease and move out. If a court later disagrees, you can still owe the rent. Get legal help first.",
		"You can withhold rent until repairs are made. If a court later disagrees, you can owe the rent and face eviction. Get legal help first.",
		"Once the landlord gets notice, you can end your lease and move. Keep paying rent until then. If you fall behind, your landlord can try to evict you.",
		"You can end your lease after this is known as constructive eviction. If a court later disagrees, you can still owe the rent. Get legal help first.",
	}
	// Repair and deduct and partial withholding are steps taken while the
	// renter stays, so they need the eviction warning (Nazanin 2026-10-29).
	stays := []string{
		"If your landlord does not fix it, you can pay a worker to fix it. You can then take the cost off your rent.",
		"For smaller problems, you can use repair and deduct (fixing it yourself and taking the cost off your rent).",
		"Repair and deduct (paying for the repair yourself, then taking that amount out of your rent) is allowed under California law.",
		"You may hold back part or all of your rent if your landlord does not fix a vital facility problem.",
		"After written notice, you can buy the service yourself and take the cost off your rent using paid receipts.",
		"Missouri has a law that lets you pay for repairs yourself and take the cost off your rent.",
		"Step 3 option: fix it yourself and subtract the cost from rent.",
		"You can repair and deduct (paying for a repair and taking the cost off your rent), hold back rent, or ask for a lower rent.",
		"After that, you can pay for reasonable amounts of those services yourself and subtract the actual, reasonable cost from your rent.",
		"Where adopted, repair and deduct (fixing the problem yourself and taking the cost off your rent) is one option.",
		"The most you can take off your rent for a repair is the lower of $500 or half a month's rent.",
		"Maine's repair and deduct rule has a licensing step.",
		"A Utah renter who uses repair and deduct must keep all receipts for the money paid.",
		"Before you fix the problem, tell your landlord in writing that you plan to fix it and take the cost off your rent.",
		"In Delaware, the rule that lets you take repair costs off your rent starts with a written notice to your landlord.",
	}
	for _, s := range stays {
		v := riskViolation("en", s, Page{})
		if !strings.Contains(v, "face eviction") {
			t.Errorf("repair-and-deduct step should need the eviction warning, got %q: %s", v, s)
		}
	}
	for _, s := range []string{
		"South Carolina law does not let you make general repairs yourself and take the cost off your rent.",
		"If you and your landlord cannot agree about a rent problem, Oklahoma law lets you take the issue to court.",
		"If the court finds a problem exists, it can order you to fix it and subtract the cost from your rent.",
		"Your landlord also cannot take a late fee out of your next rent payment.",
		"It does apply if you used Maine's repair-and-deduct rule (fixing a dangerous condition yourself and taking the cost out of your rent).",
		"You can subtract what your landlord owes you from the rent you owe them. If the court later rules in your landlord's favor, you will have to pay that rent back.",
		"A renter who is not in compliance gets no rent abatement or repair and deduct (take the cost off rent) choice in Utah.",
		"You can then sue in small claims court to make your landlord pay you back, or to take the cost off your future rent.",
		"If you are late paying rent, you may not use Delaware's rule that lets you take repair costs off your rent.",
		"You can withhold rent until repairs are made. If a court later disagrees, you can owe the rent and face eviction. Get legal help first.",
	} {
		if v := riskViolation("en", s, Page{}); v != "" {
			t.Errorf("flagged %q: %v", s, v)
		}
	}
	for _, s := range good {
		if v := riskViolation("en", s, Page{}); v != "" {
			t.Errorf("flagged %q: %v", s, v)
		}
	}
}

func TestPageWarningCoversRiskyStatements(t *testing.T) {
	owe := Page{Warns: map[string]bool{"owe": true}}
	leave := "You can end your lease if your landlord does not fix a serious problem after written notice."
	if v := riskViolation("en", leave, Page{}); v == "" {
		t.Fatal("a risky step with no warning anywhere should fail")
	}
	if v := riskViolation("en", leave, owe); v != "" {
		t.Errorf("the page warning should cover the step: %s", v)
	}
	stay := "You can withhold rent until the repair is made."
	if v := riskViolation("en", stay, owe); v == "" {
		t.Error("an owe warning does not cover withholding while staying")
	}
	if v := noOweViolation("en", "You owe no rent after you move out.", owe); v != "" {
		t.Errorf("the page warning should cover owe-nothing: %s", v)
	}
	if v := warnedTwice("en", leave+" If a court later disagrees, you can still owe the rent. Get legal help first.", owe); v == "" {
		t.Error("a statement repeating the page warning should be flagged")
	}
	if v := warnedTwice("en", leave+" If a court later disagrees, you can still owe the rent.", Page{}); v != "" {
		t.Error("with no page warning the statement keeps its own")
	}
}

func TestPageNotesAndTipsCoverTheirSentences(t *testing.T) {
	award := "If your landlord keeps your deposit without a legal right to it, you can sue and get double the deposit. Twice a $1,000 deposit is $2,000."
	if v := awardViolation("en", award, Page{}); v == "" {
		t.Fatal("award money with no win-and-pays line should fail")
	}
	if v := awardViolation("en", award, Page{Warns: map[string]bool{"award": true}}); v != "" {
		t.Errorf("the award note on the page should cover it: %s", v)
	}
	inspect := "You can ask the city to send a code inspector."
	if v := inspectViolation("en", inspect, Page{}); v == "" {
		t.Fatal("an inspector step with no caution should fail")
	}
	if v := inspectViolation("en", inspect, Page{Warns: map[string]bool{"condemn": true}}); v != "" {
		t.Errorf("the condemn note on the page should cover it: %s", v)
	}
	mail := "Send the notice by certified mail."
	if v := mailRecordViolation("en", mail, Page{}); v == "" {
		t.Fatal("certified mail with no record line should fail")
	}
	if v := mailRecordViolation("en", mail, Page{Tips: map[string]bool{"keep-a-record": true}}); v != "" {
		t.Errorf("the keep-a-record tip on the page should cover it: %s", v)
	}
	both := Page{Warns: map[string]bool{"award": true, "condemn": true}, Tips: map[string]bool{"keep-a-record": true}}
	for _, text := range []string{
		award + " You get this money only if you win your case and your landlord pays.",
		inspect + " For very bad conditions, an inspector can condemn the home and make everyone leave.",
		mail + " Save texts and emails as well; they are also a record of what you sent.",
	} {
		if v := warnedTwice("en", text, both); v == "" {
			t.Errorf("a statement repeating what the page says once should be flagged: %q", text)
		}
		if v := warnedTwice("en", text, Page{}); v != "" {
			t.Errorf("with nothing on the page the statement keeps its line: %s", v)
		}
	}
}

func TestNeedsWarning(t *testing.T) {
	cases := map[string]string{
		"If your landlord keeps your deposit without a legal right to it, you can sue and get double the deposit. Twice a $1,000 deposit is $2,000.": "award",
		"You can ask the city to send a code inspector.":                                  "condemn",
		"You can end your lease if the home is not usable.":                               "owe",
		"You can withhold rent until the repair is made.":                                 "evict",
		"You owe no rent after you move out.":                                             "owe",
		"Your landlord must return your deposit within 30 days.":                          "",
		"You can end your lease. If a court later disagrees, you can still owe the rent.": "",
	}
	for s, want := range cases {
		if got := NeedsWarning("en", s); got != want {
			t.Errorf("%q: got %q, want %q", s, got, want)
		}
	}
}

func TestMateriallyIsGlossedAsSeriously(t *testing.T) {
	if len(Lint("en", "The landlord materially breaks the lease.")) == 0 {
		t.Error("bare materially not flagged")
	}
	if len(Lint("en", "The landlord materially (in a big way) breaks the lease.")) == 0 {
		t.Error("materially with another gloss not flagged")
	}
	if v := Lint("en", "The landlord materially (seriously) breaks the lease."); len(v) != 0 {
		t.Errorf("materially (seriously) flagged: %v", v)
	}
}

func TestPlainLayoutRules(t *testing.T) {
	plain := Page{Plain: true}
	long := strings.Repeat("Your landlord must fix the heat. ", 14)
	if len(plainViolations("en", long, plain)) == 0 {
		t.Error("a long statement on a plain page should fail")
	}
	if len(plainViolations("en", long, Page{Warns: map[string]bool{"owe": true}})) != 0 {
		t.Error("a topic not switched to the plain layout is not held to it")
	}
	if len(plainViolations("en", "Your landlord may fail A.R.S. 33-1324.", plain)) == 0 {
		t.Error("a code section in the text should fail")
	}
	if len(plainViolations("en", "Your landlord has 10 days to fix it.", plain)) == 0 {
		t.Error("a deadline with no start should fail")
	}
	if v := plainViolations("en", "The law gives your landlord 10 days after they get your notice to fix it. If they do not fix it, your lease ends.", plain); len(v) != 0 {
		t.Errorf("the Arizona deadline form should pass: %v", v)
	}
	if len(Lint("en", "You owe no more rent only in a few cases, as the law sets out.")) == 0 {
		t.Error(`"as the law sets out" should be banned`)
	}
}

func TestStripSaidOnce(t *testing.T) {
	page := Page{Warns: map[string]bool{"evict": true, "award": true}, Tips: map[string]bool{"keep-a-record": true}}
	in := "You can withhold rent until it is fixed. If a court later disagrees, you can owe the rent and face eviction. Get legal help first. Send it by certified mail. Save texts and emails as well; they are also a record of what you sent. You can sue for twice the deposit. You get this money only if you win your case and your landlord pays."
	got, removed := StripSaidOnce("en", in, page)
	want := "You can withhold rent until it is fixed. Send it by certified mail. You can sue for twice the deposit."
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
	if len(removed) != 3 {
		t.Errorf("removed %d sentences, want 3 (the warning carries its legal-help tail): %v", len(removed), removed)
	}
	if got, _ := StripSaidOnce("en", in, Page{}); got != in {
		t.Error("with nothing on the page the text stays")
	}
	// Only the warning the page's note names comes out: an owe note leaves
	// the eviction warning on a withholding statement.
	stay := "You can withhold rent until it is fixed. If a court later disagrees, you can owe the rent and face eviction. Get legal help first."
	if got, _ := StripSaidOnce("en", stay, Page{Warns: map[string]bool{"owe": true}}); got != stay {
		t.Errorf("an owe note must not strip the eviction warning: %q", got)
	}
	if v := warnedTwice("en", stay, Page{Warns: map[string]bool{"owe": true}}); v != "" {
		t.Errorf("an owe note does not make the eviction warning a repeat: %s", v)
	}
	left := "You can end your lease. If a court later disagrees, you can still owe the rent. Get legal help first."
	if got, _ := StripSaidOnce("en", left, Page{Warns: map[string]bool{"owe": true}}); got != "You can end your lease." {
		t.Errorf("the owe warning should come out: %q", got)
	}
}

func TestSaidOnceKinds(t *testing.T) {
	got := SaidOnceKinds("en", "You can sue. You get this money only if you win your case and your landlord pays. If a court later disagrees, you can owe the rent and face eviction. Get legal help first.")
	if len(got) != 2 || got[0] != "award" && got[1] != "award" {
		t.Errorf("got %v, want award and evict", got)
	}
	if got := SaidOnceKinds("en", "Your landlord must fix the heat."); len(got) != 0 {
		t.Errorf("a plain rule names no risk: %v", got)
	}
}

func TestRemovalOnlySkipsHarderThan(t *testing.T) {
	old := "You can sue for twice the deposit. You get this money only if you win your case and your landlord pays. Get legal help first."
	cut := "You can sue for twice the deposit."
	if !RemovalOnly(old, cut) {
		t.Fatal("taking whole sentences out is removal only")
	}
	if RemovalOnly(old, "You can sue for triple the deposit.") {
		t.Error("a changed sentence is not removal only")
	}
	if RemovalOnly(old, old) {
		t.Error("the same text is no removal")
	}
	if v := HarderThan("en", old, cut); v != "" {
		t.Errorf("a removal-only edit is never harder: %s", v)
	}
}

func TestLintEditRemovalOnlyIgnoresTheScoreCap(t *testing.T) {
	old := "Your landlord must remedy a substantial noncompliance affecting habitability within fourteen days of written notification. Get legal help first."
	cut := "Your landlord must remedy a substantial noncompliance affecting habitability within fourteen days of written notification."
	for _, v := range LintEdit("en", old, cut, Page{}) {
		if strings.Contains(v, "reads too hard") {
			t.Errorf("a removal-only edit is not held to the score cap: %s", v)
		}
	}
}
