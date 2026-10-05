package templates

import (
	"html/template"
	"strings"
	"testing"
)

func orgEntry(bodies ...string) OrgGroup {
	g := OrgGroup{Chip: CitationChip{Label: "Los Angeles Housing Department", URL: "https://housing.lacity.gov/", SourceKind: "gov_guidance"}}
	for _, b := range bodies {
		g.Statements = append(g.Statements, RenderedStatement{BodyHTML: template.HTML(b)}) // #nosec G203 -- test fixture
	}
	return g
}

const (
	laTel    = `<a href="tel:+18665577368" data-out="7">1-866-557-7368</a>`
	otherTel = `<a href="tel:+12025550100" data-out="7">(202) 555-0100</a>`
)

// ADR-031 D6 amended: numbers stay in the text. Every sentence that names a
// number keeps it, including a bare "Call <number>.".
func TestOrgBodies_keepsEveryNumberInTheText(t *testing.T) {
	g := orgEntry(`<p>The office helps renters. Call ` + laTel + `. Its Spanish line is ` + otherTel + `.</p>`)
	got := string(g.Bodies("en")[0])
	if !strings.Contains(got, "Call "+laTel+".") || !strings.Contains(got, "Spanish line is "+otherTel) {
		t.Errorf("a number left the text: %s", got)
	}
}

// The bold "Name, phone." lead goes because the name is the heading, but its
// number stays, as "Call <number>.".
func TestOrgBodies_leadKeepsItsNumber(t *testing.T) {
	g := orgEntry(`<p><strong>Los Angeles Housing Department, ` + laTel + `.</strong> The city housing office offers services.</p>`)
	got := string(g.Bodies("en")[0])
	want := `<p>Call ` + laTel + `. The city housing office offers services.</p>`
	if got != want {
		t.Errorf("body:\n got %s\nwant %s", got, want)
	}
	if es := string(g.Bodies("es")[0]); !strings.HasPrefix(es, `<p>Llame al `+laTel+`.`) {
		t.Errorf("Spanish lead: %s", es)
	}
}

// Two numbers in the lead both stay, joined by "or".
func TestOrgBodies_leadWithTwoNumbers(t *testing.T) {
	g := orgEntry(`<p><strong>Los Angeles Housing Department, ` + laTel + ` or ` + otherTel + `.</strong> It helps renters.</p>`)
	got := string(g.Bodies("en")[0])
	if !strings.HasPrefix(got, `<p>Call `+laTel+` or `+otherTel+`. It helps renters.`) {
		t.Errorf("body: %s", got)
	}
}

// A lead with no number (a website) goes entirely: the website is the button.
func TestOrgBodies_leadWithoutANumberGoes(t *testing.T) {
	g := orgEntry(`<p><strong>Los Angeles Housing Department, housing.lacity.gov.</strong> You may file a complaint.</p>`)
	if got := string(g.Bodies("en")[0]); got != `<p>You may file a complaint.</p>` {
		t.Errorf("body: %s", got)
	}
}

// A lead naming a different organization is not the heading's, so it stays.
func TestOrgBodies_otherLeadStays(t *testing.T) {
	g := orgEntry(`<p><strong>211 LA, ` + laTel + `.</strong> Free help.</p>`)
	if got := string(g.Bodies("en")[0]); !strings.Contains(got, "<strong>211 LA") {
		t.Errorf("body: %s", got)
	}
}
