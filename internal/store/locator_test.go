package store

import "testing"

func TestLooksLikeSection(t *testing.T) {
	cases := []struct {
		locator string
		want    bool
		why     string
	}{
		// Points at a provision.
		{"§ 15B", true, "section sign"},
		{"§§ 15B-15C", true, "double section sign"},
		{"Mass. Gen. Laws ch. 186, § 15B", true, "full citation with section sign"},
		{"RCW 59.18.060", true, "all-caps code plus number"},
		{"USC 42 § 1437f", true, "federal code"},
		{"59.18.060", true, "bare provision number"},
		{"15B", true, "short provision number"},
		{"Section 8-107", true, "spelled-out section word"},
		{"Sec. 12", true, "abbreviated section word"},
		{"Ch. 186", true, "chapter"},
		{"Mass. Gen. Laws ch. 186", true, "citation word deeper in the string"},
		{"Article 7", true, "article"},
		{"Title 14, Chapter 5", true, "title"},

		// Points at a document, not a provision. These are the ones the rule exists for.
		{"", false, "empty"},
		{"   ", false, "whitespace only"},
		{"March 2026", false, "a date is not a provision"},
		{"2026", false, "a bare year is not a provision"},
		{"County announces new tenant protections", false, "a headline"},
		{"Press release", false, "no number at all"},
		{"Landlord-Tenant Act", false, "an act name without a section"},
		{"Fact sheet", false, "guidance document"},
	}

	for _, c := range cases {
		if got := looksLikeSection(c.locator); got != c.want {
			t.Errorf("looksLikeSection(%q) = %v, want %v (%s)", c.locator, got, c.want, c.why)
		}
	}
}

func TestIsBareYear(t *testing.T) {
	for _, s := range []string{"2026", "1998"} {
		if !isBareYear(s) {
			t.Errorf("isBareYear(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"15B", "59.18.060", "202", "20266"} {
		if isBareYear(s) {
			t.Errorf("isBareYear(%q) = true, want false", s)
		}
	}
}

func TestIsCodeAbbrev(t *testing.T) {
	for _, s := range []string{"RCW", "USC", "CFR", "MGL", "ORS"} {
		if !isCodeAbbrev(s) {
			t.Errorf("isCodeAbbrev(%q) = false, want true", s)
		}
	}
	// "March" is the case that matters: a capitalised word followed by a year
	// must not read as a code followed by a provision number.
	for _, s := range []string{"March", "Laws", "A", "TOOLONGCODE", "Rcw"} {
		if isCodeAbbrev(s) {
			t.Errorf("isCodeAbbrev(%q) = true, want false", s)
		}
	}
}
