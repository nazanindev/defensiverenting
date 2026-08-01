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

// maxSentenceWords is the longest sentence the voice rules allow.
const maxSentenceWords = 25

type bannedRule struct {
	re  *regexp.Regexp
	fix string
}

var bannedRules = []bannedRule{
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
	// Figurative language: breaks in translation.
	{regexp.MustCompile(`(?i)\bmental model\b`), `figurative; say "this page explains"`},
	{regexp.MustCompile(`(?i)\bnavigat(e|es|ed|ing|ion)\b`), `figurative; name the concrete action`},
	{regexp.MustCompile(`(?i)\blandscape\b`), `figurative; name the concrete thing`},
	{regexp.MustCompile(`(?i)\bleverag(e|es|ed|ing)\b`), `figurative; use "use"`},
	{regexp.MustCompile(`(?i)\bempower(s|ed|ing|ment)?\b`), `figurative; say what the reader can do`},
	{regexp.MustCompile(`(?i)\bjourney\b`), `figurative; name the concrete process`},
	{regexp.MustCompile(`(?i)\brule of thumb\b`), `figurative; state the rule plainly`},
	{regexp.MustCompile(`(?i)\bkeep in mind\b`), `drop it; state the fact directly`},
}

var (
	dashRe = regexp.MustCompile(`[—–]`)
	// Spelled-out counts before a time/count unit must be digits.
	spelledNumRe = regexp.MustCompile(`(?i)\b(two|three|four|five|six|seven|eight|nine|ten|eleven|twelve|twenty|thirty|sixty|ninety)[- ](day|week|month|year|hour|time)s?\b`)
	percentRe    = regexp.MustCompile(`%|(?i)\bpercent\b`)
	dollarRe     = regexp.MustCompile(`\$\d`)
	sentenceEnd  = regexp.MustCompile(`[.!?]\s`)
	wordRe       = regexp.MustCompile(`\S+`)
)

// Lint checks one piece of renter-facing text (a title, intro, or statement
// body) against the voice rules. It returns one message per violation; empty
// means the text passes. Never call it on citation quotes.
func Lint(text string) []string {
	var out []string

	if dashRe.MatchString(text) {
		out = append(out, `contains an em or en dash: use a period, a comma, a colon, or "to" for ranges`)
	}

	for _, s := range sentenceEnd.Split(text, -1) {
		if n := len(wordRe.FindAllString(s, -1)); n > maxSentenceWords {
			out = append(out, fmt.Sprintf("sentence with %d words (max %d), split it: %q", n, maxSentenceWords, truncate(s, 80)))
		}
	}

	for _, r := range bannedRules {
		if m := r.re.FindString(text); m != "" {
			out = append(out, fmt.Sprintf("banned word %q: %s", m, r.fix))
		}
	}

	if percentRe.MatchString(text) && !dollarRe.MatchString(text) {
		out = append(out, `mentions a percentage with no worked dollar example: add one, like "5% of $1,000 rent is $50"`)
	}

	if m := spelledNumRe.FindString(text); m != "" {
		out = append(out, fmt.Sprintf("spelled-out number %q: use digits", m))
	}

	return out
}

// LintAll lints several labeled texts and returns violations prefixed with
// their label, capped so a rejection message stays readable.
func LintAll(labeled map[string]string) []string {
	const maxViolations = 10
	var out []string
	for _, label := range sortedKeys(labeled) {
		for _, v := range Lint(labeled[label]) {
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
