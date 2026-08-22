// Package voice enforces the editorial voice rules (.claude/skills/
// editorial-voice) as save-time invariants for renter-facing text.
// The audience may have weak reading skills and be mid-crisis; text must
// land on the first read and translate cleanly. Citation quotes are exempt
// everywhere: they must stay verbatim source text.
package voice

import (
	"fmt"
	"regexp"
	"strings"
)

// maxSentenceWords is the longest sentence the voice rules allow, for every
// supported language. Spanish prose for the same content often runs a bit
// longer than English; if that turns out to make this cap too strict once
// real Spanish drafts hit it, split it into a per-language value then, with
// evidence, rather than guessing an adjustment now.
const maxSentenceWords = 25

type bannedRule struct {
	re  *regexp.Regexp
	fix string
}

// ruleset is one language's banned-word and spelled-out-number rules. The
// dash, sentence-length, and percent-needs-dollar-example checks below are
// language-agnostic and run for every supported language unconditionally.
type ruleset struct {
	banned []bannedRule
	// spelledNum flags a spelled-out count before a time/count unit, e.g.
	// "thirty days" — renter-facing numbers must be digits.
	spelledNum *regexp.Regexp
	// allowedTerms are official names a renter will meet in real life (court
	// forms, program names). They are stripped before the banned-word scan so
	// naming them, with a plain-words explanation, stays legal.
	allowedTerms *regexp.Regexp
	// moneyMultiplier + moneyWord together flag multiplied money amounts
	// ("3 times the deposit", "double the rent") that carry no worked dollar
	// example — the same failure the percent rule catches. A stressed reader
	// should never have to do the multiplication themselves.
	moneyMultiplier *regexp.Regexp
	moneyWord       *regexp.Regexp
	// timeSpan matches one time period ("30 days"); three or more in one
	// block with no ordering cue (orderCue) is a pile of deadlines nobody can
	// act on: either the steps happen in an order that must be written out,
	// or they are separate cases that belong in separate statements.
	timeSpan *regexp.Regexp
	orderCue *regexp.Regexp
}

var enRuleset = ruleset{
	banned: []bannedRule{
		// Legal jargon: say what happens instead.
		{regexp.MustCompile(`(?i)\bvoid\b`), `say what happens: "the court will not enforce it" or "does not count, even if you signed it"`},
		{regexp.MustCompile(`(?i)\bunenforceable\b`), `say what happens: "the court will not enforce it"`},
		{regexp.MustCompile(`(?i)\bwaiv(e|es|ed|er|ers|ing)\b`), `use "give up"`},
		{regexp.MustCompile(`(?i)\bremed(y|ies)\b`), `use "what you can do about it" or name the options`},
		{regexp.MustCompile(`(?i)\bpursuant\b`), `use "under" or "because of"`},
		{regexp.MustCompile(`(?i)\bprovisions?\b`), `use "part of the lease" or "rule"`},
		{regexp.MustCompile(`(?i)\bnotwithstanding\b`), `use "even if" or "despite"`},
		{regexp.MustCompile(`(?i)\b(herein|hereby|thereof|aforementioned|forthwith)\b`), `plain words only`},
		{regexp.MustCompile(`(?i)\bprior to\b`), `use "before"`},
		{regexp.MustCompile(`(?i)\butiliz(e|es|ed|ing)\b`), `use "use"`},
		{regexp.MustCompile(`(?i)\bshall\b`), `use "must"`},
		{regexp.MustCompile(`(?i)\bcommenc(e|es|ed|ing|ement)\b`), `use "start"`},
		{regexp.MustCompile(`(?i)\bterminat(e|es|ed|ing|ion)\b`), `use "end", like "end your lease" or "a notice ending your tenancy"`},
		{regexp.MustCompile(`(?i)\bdwellings?\b`), `use "home"`},
		{regexp.MustCompile(`(?i)\bpremises\b`), `use "the home" or "the property"`},
		{regexp.MustCompile(`(?i)\bin the event (that|of)\b`), `use "if"`},
		{regexp.MustCompile(`(?i)\b(thereafter|subsequently|subsequent to)\b`), `use "after" or "after that"`},
		{regexp.MustCompile(`(?i)\bin accordance with\b`), `use "under"`},
		{regexp.MustCompile(`(?i)\bremit(s|ted|ting|tance)?\b`), `use "pay" or "send"`},
		{regexp.MustCompile(`(?i)\bmonies\b`), `use "money"`},
		{regexp.MustCompile(`(?i)\bhabitab(le|ility)\b`), `use "fit to live in" (naming the warranty of habitability once, with a plain explanation, stays legal)`},
		{regexp.MustCompile(`(?i)\bfacilitat(e|es|ed|ing)\b`), `use "help"`},
		{regexp.MustCompile(`(?i)\bendeavor(s|ed|ing)?\b`), `use "try"`},
		// Figurative language: breaks in translation.
		{regexp.MustCompile(`(?i)\bmental model\b`), `figurative; say "this page explains"`},
		{regexp.MustCompile(`(?i)\bnavigat(e|es|ed|ing|ion)\b`), `figurative; name the concrete action`},
		{regexp.MustCompile(`(?i)\blandscape\b`), `figurative; name the concrete thing`},
		{regexp.MustCompile(`(?i)\bleverag(e|es|ed|ing)\b`), `figurative; use "use"`},
		{regexp.MustCompile(`(?i)\bempower(s|ed|ing|ment)?\b`), `figurative; say what the reader can do`},
		{regexp.MustCompile(`(?i)\bjourney\b`), `figurative; name the concrete process`},
		{regexp.MustCompile(`(?i)\brule of thumb\b`), `figurative; state the rule plainly`},
		{regexp.MustCompile(`(?i)\bkeep in mind\b`), `drop it; state the fact directly`},
	},
	spelledNum:      regexp.MustCompile(`(?i)\b(two|three|four|five|six|seven|eight|nine|ten|eleven|twelve|twenty|thirty|sixty|ninety)[- ](day|week|month|year|hour|time)s?\b`),
	allowedTerms:    regexp.MustCompile(`(?i)\b(fee waivers?|warrant(y|ies)? of habitability)\b`),
	moneyMultiplier: regexp.MustCompile(`(?i)\b(double|triple|twice|\d+\s*(x|times))\b`),
	moneyWord:       regexp.MustCompile(`(?i)\b(deposit|rent|damages|amount|penalty)\b`),
	timeSpan:        regexp.MustCompile(`(?i)\b\d+\s*(business\s+)?(day|days|hour|hours|week|weeks|month|months)\b`),
	orderCue:        regexp.MustCompile(`(?i)\b(first|then|next|after|before|step|until|once|start(s|ing)?|count(s|ing)?)\b`),
}

// esRuleset is a first-pass Spanish translation of enRuleset's intent, not a
// ruleset an editor has signed off on. Treat every fix message here as a
// draft to correct rather than a finished rule:
//   - "recurso legal" and "disposición" are narrowed to the legal-jargon
//     phrase where possible, because their bare forms ("recurso", "disposición")
//     are ordinary Spanish words far more often than they are legal jargon —
//     even narrowed, they may still over-fire.
//   - "aprovechar" (leverage) and "trayectoria" (journey) are guesses at where
//     English figurative language lands in Spanish; either may turn out to be
//     plain, common Spanish that shouldn't be flagged at all.
//   - allowedTerms guesses "exención de cuota/tarifa" for "fee waiver"; the
//     actual term varies by state court form and needs confirming.
//
// A Spanish-speaking editor should read this list against real drafted text
// before trusting its rejections at face value, the same way the English
// list above reflects editorial judgment this project already trusts.
var esRuleset = ruleset{
	banned: []bannedRule{
		// Legal jargon: say what happens instead.
		{regexp.MustCompile(`(?i)\bnulo\b`), `say what happens: "el tribunal no lo hará valer" or "no cuenta, aunque usted lo haya firmado"`},
		{regexp.MustCompile(`(?i)\binaplicable\b`), `say what happens: "el tribunal no lo hará valer"`},
		{regexp.MustCompile(`(?i)\brenunci(a|ar|ando|ado|as|an)\b`), `use "usted deja de tener este derecho" or "usted pierde este derecho"`},
		{regexp.MustCompile(`(?i)\brecursos? legales?\b`), `use "lo que usted puede hacer al respecto" or name the options`},
		{regexp.MustCompile(`(?i)\bde conformidad con\b`), `use "según" or "de acuerdo con"`},
		{regexp.MustCompile(`(?i)\bdisposici(ón|ones) del contrato\b`), `use "parte del contrato" or "regla"`},
		{regexp.MustCompile(`(?i)\bno obstante\b`), `use "aunque" or "aun así"`},
		{regexp.MustCompile(`(?i)\b(en lo sucesivo|por la presente|antes mencionado|anteriormente citado)\b`), `plain words only`},
		{regexp.MustCompile(`(?i)\b(previo a|con anterioridad a)\b`), `use "antes de"`},
		{regexp.MustCompile(`(?i)\butiliz(ar|a|an|ando|ado)\b`), `use "usar"`},
		// Figurative language: breaks in translation.
		{regexp.MustCompile(`(?i)\bmodelo mental\b`), `figurative; say "esta página explica"`},
		{regexp.MustCompile(`(?i)\bnavegar\b`), `figurative (unless literally about navigation); name the concrete action`},
		{regexp.MustCompile(`(?i)\bpanorama\b`), `figurative; name the concrete thing`},
		{regexp.MustCompile(`(?i)\baprovechar\b`), `figurative; use "usar" — verify this isn't just plain Spanish before trusting the flag`},
		{regexp.MustCompile(`(?i)\bempoderar\b`), `figurative; say what the reader can do`},
		{regexp.MustCompile(`(?i)\btrayectoria\b`), `figurative; name the concrete process`},
		{regexp.MustCompile(`(?i)\bregla general\b`), `figurative; state the rule plainly`},
		{regexp.MustCompile(`(?i)\btenga (en cuenta|presente)\b`), `drop it; state the fact directly`},
	},
	spelledNum:   regexp.MustCompile(`(?i)\b(dos|tres|cuatro|cinco|seis|siete|ocho|nueve|diez|once|doce|veinte|treinta|sesenta|noventa)[- ](día|días|semana|semanas|mes|meses|año|años|hora|horas|vez|veces)\b`),
	allowedTerms: regexp.MustCompile(`(?i)\bexenci(ón|ones) de (cuota|cuotas|tarifa|tarifas)\b`),
	// Same draft-quality caveat as the rest of this ruleset: these mirror the
	// English money-example and deadline-pile rules and need a native
	// speaker's read against real drafts.
	moneyMultiplier: regexp.MustCompile(`(?i)\b(el doble|el triple|\d+\s*veces)\b`),
	moneyWord:       regexp.MustCompile(`(?i)\b(depósito|renta|alquiler|fianza|monto|multa)\b`),
	timeSpan:        regexp.MustCompile(`(?i)\b\d+\s*(día|días|hora|horas|semana|semanas|mes|meses|días hábiles)\b`),
	orderCue:        regexp.MustCompile(`(?i)\b(primero|luego|después|antes|paso|hasta|una vez|desde|a partir de)\b`),
}

var rulesets = map[string]ruleset{"en": enRuleset, "es": esRuleset}

// Supported returns the language codes Lint has a ruleset for, sorted. This
// is the canonical list of languages the drafting toolbelt accepts — see
// drafting.ResolveLanguage — so a language gains support in one place.
func Supported() []string {
	return []string{"en", "es"} // keep sorted; extend rulesets above first
}

// languageLabels renders each supported code as prose, for prompt text and
// UI labels. Extend alongside Supported() and the rulesets above when a new
// language is added.
var languageLabels = map[string]string{"en": "English", "es": "Spanish"}

// Label renders a language code as a human-readable name ("es" -> "Spanish"),
// falling back to the code itself for anything Supported doesn't recognize.
func Label(code string) string {
	if l, ok := languageLabels[code]; ok {
		return l
	}
	return code
}

var (
	dashRe      = regexp.MustCompile(`[—–]`)
	percentRe   = regexp.MustCompile(`%|(?i)\bpercent\b`)
	dollarRe    = regexp.MustCompile(`\$\d`)
	sentenceEnd = regexp.MustCompile(`[.!?]\s`)
	wordRe      = regexp.MustCompile(`\S+`)
)

// Lint checks one piece of renter-facing text (a title, intro, or statement
// body) against the voice rules for lang. It returns one message per
// violation; empty means the text passes. Never call it on citation quotes.
// lang must be one of Supported(); callers validate that upstream (see
// drafting.resolveLanguage) so an unsupported lang here falls back to "en"
// defensively rather than skipping the lint entirely.
func Lint(lang, text string) []string {
	rs, ok := rulesets[lang]
	if !ok {
		rs = enRuleset
	}

	var out []string
	text = rs.allowedTerms.ReplaceAllString(text, " ")

	if dashRe.MatchString(text) {
		out = append(out, `contains an em or en dash: use a period, a comma, a colon, or "to" for ranges`)
	}

	for _, s := range sentenceEnd.Split(text, -1) {
		if n := len(wordRe.FindAllString(s, -1)); n > maxSentenceWords {
			out = append(out, fmt.Sprintf("sentence with %d words (max %d), split it: %q", n, maxSentenceWords, truncate(s, 80)))
		}
	}

	for _, r := range rs.banned {
		if m := r.re.FindString(text); m != "" {
			out = append(out, fmt.Sprintf("banned word %q: %s", m, r.fix))
		}
	}

	if percentRe.MatchString(text) && !dollarRe.MatchString(text) {
		out = append(out, `mentions a percentage with no worked dollar example: add one, like "5% of $1,000 rent is $50"`)
	}

	// Multiplied money is arithmetic the reader should never have to do:
	// "3 times the deposit" means nothing at 2am; "$4,500 on a $1,500
	// deposit" means everything.
	if rs.moneyMultiplier != nil && rs.moneyMultiplier.MatchString(text) &&
		rs.moneyWord.MatchString(text) && !dollarRe.MatchString(text) {
		out = append(out, `multiplies a money amount with no worked dollar example: add one, like "3 times a $1,000 deposit is $3,000"`)
	}

	// A pile of deadlines with no ordering words is unactionable. Either the
	// periods happen in a sequence, which must be written out, or they are
	// separate cases, which belong in separate statements.
	if rs.timeSpan != nil {
		if n := len(rs.timeSpan.FindAllString(text, -1)); n >= 3 && !rs.orderCue.MatchString(text) {
			out = append(out, fmt.Sprintf("%d time periods in one block with no ordering words: if they happen in sequence, write the order (first, then, after that) and say what starts each clock; if they are separate cases, split them into separate statements", n))
		}
	}

	if m := rs.spelledNum.FindString(text); m != "" {
		out = append(out, fmt.Sprintf("spelled-out number %q: use digits", m))
	}

	return out
}

// LintAll lints several labeled texts against lang's ruleset and returns
// violations prefixed with their label, capped so a rejection message stays
// readable.
func LintAll(lang string, labeled map[string]string) []string {
	const maxViolations = 10
	var out []string
	for _, label := range sortedKeys(labeled) {
		for _, v := range Lint(lang, labeled[label]) {
			if len(out) == maxViolations {
				out = append(out, "…and more; fix these first and retry")
				return out
			}
			out = append(out, label+": "+v)
		}
	}
	return out
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// insertion sort; the map is tiny
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
