package drafting

import (
	"regexp"
	"strings"
	"unicode"
)

// SameWords reports whether two readings of a passage carry the same words
// in the same order, ignoring what a change of reading alone changes:
// punctuation and typography, whitespace and line breaks, the U+FFFD a
// mis-decoded page leaves where a curly quote or a section sign was, and the
// subsection number a quote opens with ("1.", "(a)"). A source-drift item
// whose old and new quote pass this is a reading repaired, not a law changed
// (the Nevada chapters read as UTF-8 before ff2adbd), so the queue lets it be
// applied without being read on its own.
func SameWords(a, b string) bool {
	wa, wb := bareWords(a), bareWords(b)
	if len(wa) == 0 || len(wa) != len(wb) {
		return false
	}
	for i := range wa {
		if wa[i] != wb[i] {
			return false
		}
	}
	return true
}

var leadingNumber = regexp.MustCompile(`^(\(?[0-9]{1,3}[a-z]?[.)]|\([a-z]{1,4}\))$`)

func bareWords(s string) []string {
	var out []string
	for _, w := range strings.Fields(FoldTypography(s)) {
		if len(out) == 0 && leadingNumber.MatchString(strings.ReplaceAll(w, "�", "")) {
			continue
		}
		bare := strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				return unicode.ToLower(r)
			}
			return -1
		}, w)
		if bare != "" {
			out = append(out, bare)
		}
	}
	return out
}
