package discover

import "testing"

// fakeProvider lets tests inject candidates regardless of slug.
type fakeProvider struct{ cands []Candidate }

func (f fakeProvider) Discover(string) []Candidate { return f.cands }

func TestRun_DedupKeepsHighestConfidenceAndSorts(t *testing.T) {
	a := fakeProvider{[]Candidate{
		{URL: "https://b.example", Confidence: 0.4},
		{URL: "https://dup.example", Confidence: 0.3},
	}}
	b := fakeProvider{[]Candidate{
		{URL: "https://a.example", Confidence: 0.9},
		{URL: "https://dup.example", Confidence: 0.8}, // higher conf wins
	}}

	got := Run("anything", a, b)

	if len(got) != 3 {
		t.Fatalf("want 3 deduped candidates, got %d: %+v", len(got), got)
	}
	// Sorted by confidence desc.
	if got[0].URL != "https://a.example" || got[1].URL != "https://dup.example" {
		t.Errorf("unexpected order: %s, %s", got[0].URL, got[1].URL)
	}
	if got[1].Confidence != 0.8 {
		t.Errorf("dedup should keep higher confidence 0.8, got %v", got[1].Confidence)
	}
}

func TestRun_UnknownSlugIsEmpty(t *testing.T) {
	if got := Run("nowhere-ville", DefaultProviders()...); len(got) != 0 {
		t.Errorf("want no candidates for unknown slug, got %d", len(got))
	}
}

func TestSeedProvider_Chicago(t *testing.T) {
	got := Run("chicago", DefaultProviders()...)
	if len(got) == 0 {
		t.Fatal("expected curated Chicago candidates, got none")
	}

	validKind := map[string]bool{
		"statute": true, "regulation": true, "gov_guidance": true,
		"nonprofit": true, "editorial": true, "court_ruling": true,
	}
	seenURL := map[string]bool{}
	for _, c := range got {
		if !validKind[c.KindGuess] {
			t.Errorf("invalid kind_guess %q for %s", c.KindGuess, c.URL)
		}
		if c.Confidence <= 0 || c.Confidence > 1 {
			t.Errorf("confidence %v out of (0,1] for %s", c.Confidence, c.URL)
		}
		if c.URL == "" || c.Publisher == "" || c.Title == "" {
			t.Errorf("candidate missing required field: %+v", c)
		}
		if seenURL[c.URL] {
			t.Errorf("duplicate candidate URL %s", c.URL)
		}
		seenURL[c.URL] = true
	}

	// Highest-confidence seed is the RLTO (the primary law) at the top.
	if got[0].KindGuess != "statute" {
		t.Errorf("expected a statute ranked first, got %q (%s)", got[0].KindGuess, got[0].URL)
	}
}

func TestSeedProvider_ReturnsCopy(t *testing.T) {
	first := Run("chicago", DefaultProviders()...)
	first[0].Title = "MUTATED"
	second := Run("chicago", DefaultProviders()...)
	if second[0].Title == "MUTATED" {
		t.Error("Discover leaked the package-level seed slice; callers can mutate shared state")
	}
}

// Reference-only domains never leave discovery, and the matcher takes
// subdomains but not suffix lookalikes.
func TestReferenceOnly(t *testing.T) {
	for url, want := range map[string]bool{
		"https://www.nolo.com/legal-encyclopedia/x": true,
		"https://blog.zillow.com/rentals":           true,
		"https://www.notnolo.com/x":                 false,
		"https://malegislature.gov/Laws":            false,
		"https://www.justia.com/real-estate/":       true,
	} {
		if got := ReferenceOnly(url); got != want {
			t.Errorf("ReferenceOnly(%q) = %v, want %v", url, got, want)
		}
	}
}

type refOnlyProvider struct{}

func (refOnlyProvider) Discover(string) []Candidate {
	return []Candidate{
		{URL: "https://www.nolo.com/x", Confidence: 0.9},
		{URL: "https://malegislature.gov/Laws", Confidence: 0.5},
	}
}

func TestRun_DropsReferenceOnlyCandidates(t *testing.T) {
	got := Run("boston", refOnlyProvider{})
	if len(got) != 1 || got[0].URL != "https://malegislature.gov/Laws" {
		t.Errorf("reference-only candidate must be dropped, got %+v", got)
	}
}
