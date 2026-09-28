package drafting

import "testing"

func TestSameWords(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		// A Nevada quote stored while the page read as UTF-8, and the same
		// passage read as Windows-1252.
		{"� 1.  If the landlord unlawfully removes the\ntenant from the premises, the tenant�s remedy", "If the landlord unlawfully removes the tenant from the premises, the tenant’s remedy", true},
		{"2.  Except as otherwise provided in NRS 118A.315:\n\n� (a) Any term", "Except as otherwise provided in NRS 118A.315: (a) Any term", true},
		{"After applying, it may take several weeks.", "After applying it may take several weeks.", true},
		// A word dropped is a change in what the source says.
		{"debt collectors can’t use unfair, deceptive, or abusive practices", "debt collectors can’t use unfair or deceptive practices", false},
		// So is a heading pulled in, or a sentence lost.
		{"1. Except as otherwise provided", "NRS 118A.510 Retaliatory conduct. 1. Except as otherwise provided", false},
		{"(a) A finding. Notwithstanding the forgoing", "Notwithstanding the forgoing", false},
		// A subsection number mid-passage is a word like any other.
		{"the tenant may (a) sue", "the tenant may (b) sue", false},
		{"", "", false},
	} {
		if got := SameWords(c.a, c.b); got != c.want {
			t.Errorf("SameWords(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
