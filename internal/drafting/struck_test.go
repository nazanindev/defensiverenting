package drafting

import "testing"

// A Mississippi bill page marks deleted words with <s> and added words with
// <u>. Only the law as amended may reach a quote.
func TestHTMLStripper_dropsStruckText(t *testing.T) {
	body := `<p>The landlord <s>may</s><u>shall</u> return the deposit within <strike>thirty</strike><u>forty-five</u> days.</p>` +
		`<p><del class="x">Old sentence.</del> <span>Kept</span> <strong>words</strong> <sup>1</sup> <section>here</section></p>`
	got := htmlStripper{}.extract(body)
	want := "The landlord shall return the deposit within forty-five days.\n\nKept words 1\nhere"
	if got != want {
		t.Fatalf("extract:\n got %q\nwant %q", got, want)
	}
	if QuoteAppearsIn(got, "landlord may return") || !QuoteAppearsIn(got, "landlord shall return") {
		t.Fatal("struck words must not match; added words must")
	}
}
