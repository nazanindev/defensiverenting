package templates

import "testing"

// A directory page lists organisations, and one organisation takes several
// statements to describe. These tests cover the grouping that keeps those
// statements together as one entry.

func stmt(url, label, locator string) RenderedStatement {
	return RenderedStatement{
		Citations: []CitationChip{{URL: url, Label: label, Locator: locator, SourceKind: "nonprofit"}},
	}
}

func TestGroupByOrg_collapsesConsecutiveStatementsSharingASource(t *testing.T) {
	got := groupByOrg([]RenderedStatement{
		stmt("https://a.org/", "A Org", "Contact"),
		stmt("https://a.org/", "A Org", "Hours"),
		stmt("https://a.org/", "A Org", "Eligibility"),
		stmt("https://b.org/", "B Org", "Help"),
	})

	if len(got) != 2 {
		t.Fatalf("want 2 organisations, got %d", len(got))
	}
	if n := len(got[0].Statements); n != 3 {
		t.Errorf("first organisation should carry all 3 of its statements, got %d", n)
	}
	if got[0].Chip.Label != "A Org" || got[1].Chip.Label != "B Org" {
		t.Errorf("groups should keep author order, got %q then %q", got[0].Chip.Label, got[1].Chip.Label)
	}
}

func TestGroupByOrg_dropsTheLocatorFromTheGroupChip(t *testing.T) {
	// The group spans statements whose locators differ, so showing one of them
	// would label the whole organisation with a section that only applies to
	// part of it. The locator stays on every stored citation regardless.
	got := groupByOrg([]RenderedStatement{
		stmt("https://a.org/", "A Org", "Contact"),
		stmt("https://a.org/", "A Org", "Hours"),
	})
	if got[0].Chip.Locator != "" {
		t.Errorf("group chip should carry no locator, got %q", got[0].Chip.Locator)
	}
}

func TestGroupByOrg_startsANewEntryWhenAnOrganisationRecurs(t *testing.T) {
	// Grouping is by consecutive run, so returning to an organisation later
	// produces a second entry rather than reordering what the author wrote.
	got := groupByOrg([]RenderedStatement{
		stmt("https://a.org/", "A Org", ""),
		stmt("https://b.org/", "B Org", ""),
		stmt("https://a.org/", "A Org", ""),
	})
	if len(got) != 3 {
		t.Fatalf("want 3 entries preserving author order, got %d", len(got))
	}
}

func TestGroupByOrg_skipsStatementsWithNoCitation(t *testing.T) {
	// The handler guarantees every statement is cited, so this is defensive:
	// an uncited statement must not panic or invent an empty organisation.
	got := groupByOrg([]RenderedStatement{
		{Citations: nil},
		stmt("https://a.org/", "A Org", ""),
	})
	if len(got) != 1 {
		t.Fatalf("want 1 entry, got %d", len(got))
	}
	if got[0].Chip.Label != "A Org" {
		t.Errorf("want the cited statement kept, got %q", got[0].Chip.Label)
	}
}

func TestGroupByOrg_ignoresTheLocatorAnchorOnTheChipURL(t *testing.T) {
	// A chip URL is SourceURL + "#locator" so a reader lands on the right part
	// of a long statute. That made every statement citing one source carry a
	// different URL, and grouping on it grouped nothing: a directory of three
	// organizations rendered as seven entries.
	got := groupByOrg([]RenderedStatement{
		stmt("https://a.org/#one", "A Org", "one"),
		stmt("https://a.org/#two", "A Org", "two"),
		stmt("https://a.org/#three", "A Org", "three"),
	})
	if len(got) != 1 {
		t.Fatalf("want 1 organization, got %d", len(got))
	}
	if n := len(got[0].Statements); n != 3 {
		t.Errorf("want all 3 statements in it, got %d", n)
	}
	if got[0].Chip.URL != "https://a.org/" {
		t.Errorf("the group chip should link to the source, not one statement's anchor: %q", got[0].Chip.URL)
	}
}

func TestGroupByOrg_handlesNoStatements(t *testing.T) {
	if got := groupByOrg(nil); len(got) != 0 {
		t.Errorf("want no groups, got %d", len(got))
	}
}
