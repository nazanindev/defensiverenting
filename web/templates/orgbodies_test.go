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

const otaTel = `<a href="tel:+12027196560" data-out="7">(202) 719-6560</a>`

// The call button already shows the number, so a sentence that only says
// "Call <number>." is not repeated above it.
func TestOrgBodies_dropsABareCallSentenceTheButtonRepeats(t *testing.T) {
	g := orgEntry(`<p>The office takes questions on landlord disputes. Call ` + otaTel + `. Phone intake runs Monday to Friday.</p>`)
	if c := g.Call(); c == nil || c.Label != "(202) 719-6560" {
		t.Fatalf("call button = %+v", c)
	}
	got := string(g.Bodies()[0])
	if strings.Contains(got, "719-6560") {
		t.Errorf("the number is still in the text: %s", got)
	}
	want := `<p>The office takes questions on landlord disputes. Phone intake runs Monday to Friday.</p>`
	if got != want {
		t.Errorf("body:\n got %s\nwant %s", got, want)
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

// A second number is not the button's number, so its sentence stays.
func TestOrgBodies_keepsACallSentenceForAnotherNumber(t *testing.T) {
	other := `<a href="tel:+12025550100" data-out="7">(202) 555-0100</a>`
	g := orgEntry(`<p>The office helps renters. Call `+otaTel+`.</p>`, `<p>For Spanish, call `+other+`. Call `+other+`.</p>`)
	got := string(g.Bodies()[1])
	if strings.Count(got, "555-0100") != 2 {
		t.Errorf("a sentence for a different number was cut: %s", got)
	}
}
