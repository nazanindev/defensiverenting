package templates_test

import (
	"os"
	"strings"
	"testing"

	tmpl "github.com/nazanindev/defensiverenting/web/templates"
)

// The citation chip is the only signal telling a reader whether a statement is
// backed by a statute, a court, a government program, or an organisation
// describing its own services. ADR-003 makes that signal part of the citation
// guarantee. These tests hold the two halves of it.

func TestChipClass_everyKindHasItsOwnClass(t *testing.T) {
	seen := map[string]string{}
	for _, kind := range tmpl.SourceKinds {
		got := tmpl.ChipClass(kind)
		if other, dup := seen[got]; dup {
			t.Errorf("kind %q and %q both render as %q — a reader cannot tell them apart",
				kind, other, got)
		}
		seen[got] = kind
	}
}

// An unrecognised kind must not borrow a real kind's authority. This is the
// regression: the default arm used to return chip--gov, so every nonprofit
// source rendered as a government one.
func TestChipClass_unknownKindClaimsNoAuthority(t *testing.T) {
	got := tmpl.ChipClass("something-new")
	for _, kind := range tmpl.SourceKinds {
		if got == tmpl.ChipClass(kind) {
			t.Fatalf("unknown kind renders as %q, the same as %q", got, kind)
		}
	}
}

// A class with no stylesheet rule renders unstyled, which is the same failure
// wearing different clothes: the reader gets no authority signal.
func TestChipClass_everyClassIsStyled(t *testing.T) {
	css, err := os.ReadFile("../static/site.css")
	if err != nil {
		t.Fatal(err)
	}
	sheet := string(css)

	kinds := append([]string{}, tmpl.SourceKinds...)
	kinds = append(kinds, "something-new") // exercises the default arm too
	for _, kind := range kinds {
		modifier := strings.TrimPrefix(tmpl.ChipClass(kind), "chip ")
		if !strings.Contains(sheet, "."+modifier+" ") && !strings.Contains(sheet, "."+modifier+"{") {
			t.Errorf("kind %q maps to .%s, which site.css does not define", kind, modifier)
		}
		if !strings.Contains(sheet, "."+modifier+"::before") {
			t.Errorf("kind %q maps to .%s with no ::before prefix — colour would be its only cue (ADR-006 D7)",
				kind, modifier)
		}
	}
}
