package templates_test

import (
	"testing"

	tmpl "github.com/nazanindev/defensiverenting/web/templates"
)

// The words after a citation are the only signal telling a reader whether a
// statement is backed by a statute, a court, a government guide, or an
// organisation describing its own services (ADR-003, ADR-032 D2). These tests
// hold the two halves of it.

func TestSourceKindWords_everyKindHasItsOwnWords(t *testing.T) {
	seen := map[string]string{}
	for _, kind := range tmpl.SourceKinds {
		got := tmpl.SourceKindWords("en", kind)
		if got == "" {
			t.Errorf("kind %q renders as nothing", kind)
		}
		if other, dup := seen[got]; dup {
			t.Errorf("kind %q and %q both render as %q — a reader cannot tell them apart",
				kind, other, got)
		}
		seen[got] = kind
	}
}

// An unrecognised kind must not borrow a real kind's authority. This is the
// regression: the default arm used to render as the government kind, so every
// nonprofit source read as a government one.
func TestSourceKindWords_unknownKindClaimsNoAuthority(t *testing.T) {
	got := tmpl.SourceKindWords("en", "something-new")
	for _, kind := range tmpl.SourceKinds {
		if got == tmpl.SourceKindWords("en", kind) {
			t.Fatalf("unknown kind renders as %q, the same as %q", got, kind)
		}
	}
}

// The citation is one link: locator and publisher together, then the kind.
func TestSourceLineFor_locatorLeadsThePublisher(t *testing.T) {
	line := tmpl.SourceLineFor("en", tmpl.CitationChip{
		SourceID: 7, URL: "https://example.gov/law#s1", Label: "Example Legislature", Locator: "§ 1", SourceKind: "statute",
	})
	if line.Text != "§ 1, Example Legislature" {
		t.Errorf("link text %q, want the locator then the publisher", line.Text)
	}
	if line.Kind != "Law" {
		t.Errorf("kind %q, want Law", line.Kind)
	}
	if !line.External {
		t.Error("a law opens in a new tab")
	}
	bare := tmpl.SourceLineFor("en", tmpl.CitationChip{Label: "City of Boston", SourceKind: "gov_guidance"})
	if bare.Text != "City of Boston" {
		t.Errorf("without a locator the publisher is the link, got %q", bare.Text)
	}
}

// The editorial page is named for what it is and carries no kind after it.
func TestSourceLineFor_editorialIsOurRules(t *testing.T) {
	line := tmpl.SourceLineFor("en", tmpl.CitationChip{URL: "/editorial", Label: "RenterLaw editorial", SourceKind: "editorial"})
	if line.Text != "Our editorial rules" || line.Kind != "" || line.External {
		t.Errorf("editorial line = %+v", line)
	}
}

func TestSourceListFor_leadCountsTheSources(t *testing.T) {
	one := tmpl.SourceListFor("en", []tmpl.CitationChip{{Label: "A", SourceKind: "statute"}})
	two := tmpl.SourceListFor("en", []tmpl.CitationChip{{Label: "A", SourceKind: "statute"}, {Label: "B", SourceKind: "nonprofit"}})
	if one.Lead != "Source" || two.Lead != "Sources" {
		t.Errorf("leads %q and %q", one.Lead, two.Lead)
	}
}
