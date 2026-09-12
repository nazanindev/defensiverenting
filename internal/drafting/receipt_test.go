package drafting

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestReceipt_readabilityByTier(t *testing.T) {
	long := strings.Repeat("statute text ", 300)
	cases := []struct {
		name           string
		r              Receipt
		live, readable bool
	}{
		{"direct full page", newReceipt("u", long, TierDirect, ExtractorHTML), true, true},
		{"render full page", newReceipt("u", long, TierRender, ExtractorRender), true, true},
		{"direct thin shell", newReceipt("u", "Checking your browser.", TierDirect, ExtractorHTML), true, false},
		{"archive full page", newReceipt("u", long, TierArchive, ExtractorHTML), false, false},
	}
	for _, c := range cases {
		if c.r.Live() != c.live || c.r.Readable() != c.readable {
			t.Errorf("%s: live=%v readable=%v, want %v/%v", c.name, c.r.Live(), c.r.Readable(), c.live, c.readable)
		}
	}
}

func TestReceipt_hashIgnoresLayout(t *testing.T) {
	a := newReceipt("u", "The lessor\n\tshall   return", TierDirect, ExtractorHTML)
	b := newReceipt("u", "The lessor shall return", TierRender, ExtractorRender)
	if a.Hash != b.Hash {
		t.Error("the same words with different whitespace must hash the same: the hash is what says 'unchanged'")
	}
	c := newReceipt("u", "The lessor shall not return", TierDirect, ExtractorHTML)
	if a.Hash == c.Hash {
		t.Error("different words must hash differently")
	}
}

func TestComparable(t *testing.T) {
	if Comparable(ExtractorPDFToText, ExtractorPDFGo) {
		t.Error("the pure-Go reader may not declare a pdftotext-confirmed quote missing")
	}
	for _, ok := range [][2]string{
		{ExtractorPDFGo, ExtractorPDFToText}, {ExtractorHTML, ExtractorRender}, {"", ExtractorPDFGo}, {ExtractorPDFToText, ExtractorPDFToText},
	} {
		if !Comparable(ok[0], ok[1]) {
			t.Errorf("Comparable(%q, %q) = false, want true", ok[0], ok[1])
		}
	}
}

func TestContext(t *testing.T) {
	text := strings.Repeat("before ", 60) + "the  landlord\nshall return the deposit" + strings.Repeat(" after", 60)
	got := Context(text, "the landlord shall return the deposit")
	if !strings.Contains(got, "the landlord shall return the deposit") {
		t.Fatalf("context lacks the quote: %q", got)
	}
	if !strings.HasPrefix(got, "…") || !strings.HasSuffix(got, "…") {
		t.Errorf("a passage cut from the middle of a page must show it was cut: %q", got)
	}
	if len(got) > 2*contextRadius+len("the landlord shall return the deposit")+8 {
		t.Errorf("context too long: %d chars", len(got))
	}
	if Context(text, "not on the page") != "" {
		t.Error("a quote not in the text has no context")
	}
	if got := Context("short page with the quote", "the quote"); got != "short page with the quote" {
		t.Errorf("a short page is its own context, got %q", got)
	}
}

func TestDescribe(t *testing.T) {
	r := newReceipt("u", "tiny", TierDirect, ExtractorPDFToText)
	if got := r.Describe(); got != "direct fetch, pdftotext (thin: 4 chars)" {
		t.Errorf("Describe() = %q", got)
	}
	if got := newReceipt("u", strings.Repeat("x ", 2000), TierRender, ExtractorRender).Describe(); got != "headless render" {
		t.Errorf("Describe() = %q", got)
	}
}

func TestNewReceipt_makesTextValidUTF8(t *testing.T) {
	// 0xa0 alone is a Latin-1 non-breaking space, not UTF-8.
	rc := newReceipt("https://x", "before\xa0after", TierDirect, ExtractorHTML)
	if !utf8.ValidString(rc.Text) {
		t.Fatalf("receipt text is not valid UTF-8: %q", rc.Text)
	}
	if !QuoteAppearsIn(rc.Text, "after") {
		t.Error("the readable words must survive")
	}
}
