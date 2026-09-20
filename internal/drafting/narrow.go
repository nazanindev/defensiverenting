package drafting

import (
	"fmt"
	"regexp"
	"strings"
)

// NarrowQuoteWords is the length under which a statute or regulation quote
// is judged too narrow to monitor the provision it cites. A quote is the
// tripwire for the law changing: only the quoted text is watched, so a
// one-sentence quote of a subsection lets an amendment to any other
// sentence in it pass unnoticed. The public site never shows the quote,
// so quoting the whole subsection costs the reader nothing.
const NarrowQuoteWords = 25

// NarrowQuote reports, as a reviewer note, a statute or regulation quote
// too short to monitor its provision. Empty when the citation is fine or
// is not a legal text: guidance and directory pages are quoted for the
// line that supports the claim, and a paragraph there is enough.
func NarrowQuote(kind, locator, quote string) string {
	if kind != "statute" && kind != "regulation" {
		return ""
	}
	n := len(strings.Fields(quote))
	if n >= NarrowQuoteWords || quotedWhole(quote) {
		return ""
	}
	where := "the provision"
	if l := strings.TrimSpace(locator); l != "" {
		where = l
	}
	return fmt.Sprintf("Quote monitor: the citation of %s quotes %d words, so a change elsewhere in it would pass unnoticed. Quote the whole subsection, from its marker to its end.", where, n)
}

// wholeRE is the shape of a subsection quoted whole: it opens with its
// marker, "(c) " or "2. ", and closes on the punctuation that ends it.
var wholeRE = regexp.MustCompile(`^(?:\([A-Za-z0-9]+\)|[0-9]+\.) .*[.;:]$`)

// quotedWhole reports a quote that is a subsection from marker to end. A
// short subdivision ("(l) Any waiver of the rights under this section
// shall be void.") is the whole provision and monitors it entirely.
func quotedWhole(quote string) bool {
	return wholeRE.MatchString(strings.TrimSpace(strings.Join(strings.Fields(quote), " ")))
}
