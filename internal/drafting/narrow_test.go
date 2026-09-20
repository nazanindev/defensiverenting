package drafting

import (
	"strings"
	"testing"
)

func TestNarrowQuote(t *testing.T) {
	short := "the landlord shall return the deposit within thirty days"
	long := strings.Repeat("word ", NarrowQuoteWords)
	if got := NarrowQuote("statute", "§ 1950.5(g)", short); !strings.Contains(got, "§ 1950.5(g)") || !strings.Contains(got, "9 words") {
		t.Errorf("short statute quote: %q", got)
	}
	if got := NarrowQuote("regulation", "", short); !strings.Contains(got, "the provision") {
		t.Errorf("short regulation quote without locator: %q", got)
	}
	if got := NarrowQuote("statute", "§ 1", long); got != "" {
		t.Errorf("a whole subsection is not narrow: %q", got)
	}
	if got := NarrowQuote("statute", "§ 1947.12(l)", "(l) Any waiver of the rights under this section shall be void as contrary to public policy."); got != "" {
		t.Errorf("a short subdivision quoted whole is not narrow: %q", got)
	}
	if got := NarrowQuote("statute", "§ 1161(6)", "6. A landlord or its agent shall not charge a tenant a fee for serving any notice."); got != "" {
		t.Errorf("a dotted subdivision quoted whole is not narrow: %q", got)
	}
	if got := NarrowQuote("gov_guidance", "Deposits", short); got != "" {
		t.Errorf("guidance is quoted for the line: %q", got)
	}
}
