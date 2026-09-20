package drafting

import (
	"fmt"
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
	if n >= NarrowQuoteWords {
		return ""
	}
	where := "the provision"
	if l := strings.TrimSpace(locator); l != "" {
		where = l
	}
	return fmt.Sprintf("Quote monitor: the citation of %s quotes %d words, so a change elsewhere in it would pass unnoticed. Quote the whole subsection, from its marker to its end.", where, n)
}
