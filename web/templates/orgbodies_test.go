package templates

import (
	"html/template"
	"strings"
	"testing"
)

func orgEntry(bodies ...string) OrgGroup {
	g := OrgGroup{Chip: CitationChip{Label: "DC Office of the Tenant Advocate", URL: "https://ota.dc.gov/", SourceKind: "gov_guidance"}}
	for _, b := range bodies {
		g.Statements = append(g.Statements, RenderedStatement{BodyHTML: template.HTML(b)}) // #nosec G203 -- test fixture
	}
	return g
}

const (
	otaTel   = `<a href="tel:+12027196560" data-out="7">(202) 719-6560</a>`
	otherTel = `<a href="tel:+12025550100" data-out="7">(202) 555-0100</a>`
)

// The call button already shows the number, so a sentence that only says
// "Call <number>." is not repeated above it.
func TestOrgBodies_dropsABareCallSentenceTheButtonRepeats(t *testing.T) {
	g := orgEntry(`<p>The office takes questions on landlord disputes. Call ` + otaTel + `. Phone intake runs Monday to Friday.</p>`)
	calls := g.Calls()
	if len(calls) != 1 || calls[0].Label != "(202) 719-6560" {
		t.Fatalf("call buttons = %+v", calls)
	}
	got := string(g.Bodies()[0])
	want := `<p>The office takes questions on landlord disputes. Phone intake runs Monday to Friday.</p>`
	if got != want {
		t.Errorf("body:\n got %s\nwant %s", got, want)
	}
}

// Every distinct number gets its own button, in the order the text gives
// them, and a number repeated in the text gets one button.
func TestOrgCalls_oneButtonPerNumber(t *testing.T) {
	g := orgEntry(`<p>Call `+otaTel+`.</p>`, `<p>For Spanish, call `+otherTel+`. Its main line is `+otaTel+`.</p>`)
	calls := g.Calls()
	if len(calls) != 2 || calls[0].Label != "(202) 719-6560" || calls[1].Label != "(202) 555-0100" {
		t.Fatalf("call buttons = %+v", calls)
	}
	// Sentences that say what a number is for stay; the bare one goes.
	bodies := g.Bodies()
	if len(bodies) != 1 {
		t.Fatalf("bodies = %d, want 1 (the bare call statement leaves nothing)", len(bodies))
	}
	if got := string(bodies[0]); !strings.Contains(got, "For Spanish, call") || !strings.Contains(got, "Its main line is") {
		t.Errorf("a sentence with meaning was cut: %s", got)
	}
}

// A statement that was only the call sentence leaves no empty block.
func TestOrgBodies_dropsAStatementLeftEmpty(t *testing.T) {
	g := orgEntry(`<p>The office helps renters.</p>`, `<p>Call `+otaTel+`.</p>`)
	if n := len(g.Bodies()); n != 1 {
		t.Fatalf("bodies = %d, want 1", n)
	}
}

// A sentence that says what the number is for keeps it.
func TestOrgBodies_keepsASentenceThatSaysMore(t *testing.T) {
	g := orgEntry(`<p>Its Homeless Services Hotline is ` + otaTel + `.</p>`)
	if got := string(g.Bodies()[0]); !strings.Contains(got, "Hotline is") || !strings.Contains(got, "719-6560") {
		t.Errorf("a meaningful sentence was cut: %s", got)
	}
}

// An entry with no number has no call button.
func TestOrgCalls_noneWithoutANumber(t *testing.T) {
	if calls := orgEntry(`<p>Apply online.</p>`).Calls(); len(calls) != 0 {
		t.Fatalf("call buttons = %+v", calls)
	}
}
