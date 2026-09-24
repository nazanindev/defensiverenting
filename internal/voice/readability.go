package voice

import (
	"fmt"
	"regexp"
	"strings"
)

// Readability rules (2026-09-24). The word cap alone pushed agents to fit a
// statement under the line by reaching for denser, harder words, which the
// voice rules exist to prevent. These rules make that trade fail the lint, so
// the only ways to shorten a statement are to split it or cut a fact.
//
// Measured on 1,778 statements before the rules shipped: median grade 6.6,
// 7% above grade 10, 132 statements with two or more words of 4+ syllables.

// MaxStatementGrade is the highest Flesch-Kincaid grade a statement body may
// read at. The score is noisy for short statements, so the hard stop sits at
// 10 and agents aim for about 8.
const MaxStatementGrade = 10.0

// maxGradeRise is how much harder an edit may make a statement read, once the
// edit lands above targetGrade. Below the target a fix may add a condition
// freely; the rule is there to stop loops compressing text into density.
const (
	maxGradeRise = 1.0
	targetGrade  = 8.0
)

var (
	hardToken    = regexp.MustCompile(`[A-Za-z][A-Za-z'’-]*`)
	webAddress   = regexp.MustCompile(`(?i)https?://\S+|\S+\.(org|gov|com|net|us|info)(/\S*)?`)
	glossFollows = regexp.MustCompile(`^\s*\(`)
	readWord     = regexp.MustCompile(`[A-Za-z][A-Za-z'’]*`)
	readSent     = regexp.MustCompile(`[.!?:;]+(\s|$)|\n+`)
	vowelRuns    = regexp.MustCompile(`[aeiouy]+`)
)

// syllables estimates the syllables in one English word. It is a heuristic:
// good enough to rank text, not to hyphenate it.
func syllables(w string) int {
	w = strings.ToLower(w)
	w = strings.Trim(w, "'’")
	if len(w) <= 3 {
		return 1
	}
	for _, suf := range []string{"es", "ed", "e"} {
		if strings.HasSuffix(w, suf) && !strings.HasSuffix(w, "le") {
			w = strings.TrimSuffix(w, suf)
			break
		}
	}
	w = strings.TrimPrefix(w, "y")
	n := len(vowelRuns.FindAllString(w, -1))
	if n < 1 {
		return 1
	}
	return n
}

// ReadingGrade is the Flesch-Kincaid grade of a statement body.
func ReadingGrade(text string) float64 {
	text = strings.NewReplacer("**", " ", "(", " ", ")", " ").Replace(text)
	words := readWord.FindAllString(text, -1)
	if len(words) == 0 {
		return 0
	}
	sents := 0
	for _, s := range readSent.Split(text, -1) {
		if readWord.MatchString(s) {
			sents++
		}
	}
	if sents == 0 {
		sents = 1
	}
	syl := 0
	for _, w := range words {
		syl += syllables(w)
	}
	return 0.39*float64(len(words))/float64(sents) + 11.8*float64(syl)/float64(len(words)) - 15.59
}

// commonLong lists everyday words of 4+ syllables a renter reads without
// trouble, and the official terms the site names on purpose (each official
// term that needs it already carries a plain gloss under the explain rules).
// Add to it when a flagged word is plainly common; never add a word only
// because an agent used it.
var commonLong = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`
		everything everyday everybody anybody anything
		refrigerator temperature confidential seriously generally usually actually
		automatically immediately especially definitely probably certainly
		responsible ordinary electrical electricity improvements delivery differently
		information application applications organization organizations
		community communities available alternative alternatives necessary
		apartment apartments utilities utility emergency emergencies security
		television identification original particular individual individuals
		january february
		judgment judgments mediation mediator habitability eviction evictions
		retaliation retaliatory discrimination disability disabilities accommodation
		accommodations assistance municipal ordinance ordinances documentation
		inspection inspector registration department departments authority
		association agreement agreements representative representation
		unreasonable unreasonably reasonable reasonably illegally unlawfully
		enforcement immigration stabilization certificate certification
		conditioning ventilation temporary understanding
		affordable carelessness completely everywhere conversation replacement
		elevator directory development management emotional military supervisor
		education physically temperatures permanently temporarily communication
		opportunity activity ability facility facilities officially organizing
		participating associations disagreement significant voluntary residential
		corporation interpreter citizenship investigate investigates investigator
		investigation negotiating invitation orientation relocation automatic
		privately peacefully unusable unlivable refundable nonrefundable
		requirements foreclosure foreclosures amenity amenities approximate
		interfering interference anonymous environmental competition operator
		deliberately intentionally federally homelessness sanitation sanitary
		manufactured legitimate discriminate discriminated stability advocacy
		infestations interviewers resolution alterations
		identify identity separately regularly typically carefully carelessly
		repeatedly delivering arrangement arrangements responsibility liability
		exercising unnecessary professional conversations categories anniversary
		generator conditioner photographing reschedule depositing authorities
		indirectly preferably localities territories relative's decorations
		evacuation eligibility availability equivalent retaliating discriminates
		discriminatory unauthorized disconnection inability commissioners
		coordinators multilingual consultation organisations ventilating
		intentional dishonestly recalculates characteristics terminations
	`) {
		commonLong[w] = true
	}
}

// HardWords returns the words in text of 4+ syllables that are not on the
// common list. Capitalized words (names of places, programs, courts), web
// addresses, and a term followed at once by a gloss in parentheses are
// skipped; a hyphenated compound is judged by its parts.
func HardWords(text string) []string {
	var out []string
	seen := map[string]bool{}
	text = webAddress.ReplaceAllString(text, " ")
	for _, loc := range hardToken.FindAllStringIndex(text, -1) {
		tok := text[loc[0]:loc[1]]
		if tok[0] >= 'A' && tok[0] <= 'Z' {
			continue
		}
		// An official term the renter will meet on court papers may stay
		// when its plain meaning follows at once: "restitution (getting the
		// home back)".
		if glossFollows.MatchString(text[loc[1]:]) {
			continue
		}
		for _, w := range strings.Split(tok, "-") {
			lw := strings.ToLower(strings.Trim(w, "'’"))
			if lw == "" || commonLong[lw] || seen[lw] || syllables(lw) < 4 {
				continue
			}
			seen[lw] = true
			out = append(out, lw)
		}
	}
	return out
}

// readabilityViolations is the statement-only readability rule.
func readabilityViolations(lang, text string) []string {
	if lang != "en" {
		return nil
	}
	var out []string
	if g := ReadingGrade(text); g > MaxStatementGrade {
		out = append(out, fmt.Sprintf("reads at grade %.1f (max %.0f): use shorter sentences and plainer words, or split it into separate statements. Do not cut words by swapping in harder ones", g, MaxStatementGrade))
	}
	if hw := HardWords(text); len(hw) > 0 {
		out = append(out, fmt.Sprintf("hard words %q: use plainer words a renter reads without stopping", hw))
	}
	return out
}

// HarderThan reports why a replacement body reads harder than the body it
// replaces, or "" when it does not. An edit may fix a fact, but it may not
// make the statement harder to read: that is how review loops compress text.
func HarderThan(lang, oldBody, newBody string) string {
	if lang != "en" || strings.TrimSpace(oldBody) == "" {
		return ""
	}
	if og, ng := ReadingGrade(oldBody), ReadingGrade(newBody); ng > targetGrade && ng > og+maxGradeRise {
		return fmt.Sprintf("the replacement reads at grade %.1f, harder than the current %.1f by more than %.0f: keep the words as plain as they were", ng, og, maxGradeRise)
	}
	had := map[string]bool{}
	for _, w := range HardWords(oldBody) {
		had[w] = true
	}
	var added []string
	for _, w := range HardWords(newBody) {
		if !had[w] {
			added = append(added, w)
		}
	}
	if len(added) > 0 {
		return fmt.Sprintf("the replacement adds hard words %q the current text does not have", added)
	}
	return ""
}
