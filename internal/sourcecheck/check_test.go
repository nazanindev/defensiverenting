package sourcecheck

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

type fakeStore struct {
	store.Store
	rows        []store.CitationCheckRow
	marks       map[int64]bool     // sourceID -> reviewed
	stamped     map[int64][]string // sourceID -> quotes confirmed present and stamped
	uncheckable int
	statements  map[string]store.ProposedStatement // by key
	alreadyOpen map[string]bool                    // key -> a drift proposal is already waiting
	filed       []store.FileProposalParams
	unusedFiled int
}

func (f *fakeStore) FileUnusedSourceProposals(context.Context, string) (int, error) {
	return f.unusedFiled, nil
}

func (f *fakeStore) StatementByKey(_ context.Context, key string) (store.ProposedStatement, error) {
	st, ok := f.statements[key]
	if !ok {
		return st, store.ErrNotFound
	}
	return st, nil
}

func (f *fakeStore) DriftAlreadyFiled(_ context.Context, key, _ string) (bool, error) {
	return f.alreadyOpen[key], nil
}

func (f *fakeStore) FileProposal(_ context.Context, p store.FileProposalParams) (int64, error) {
	f.filed = append(f.filed, p)
	return int64(len(f.filed)), nil
}

func (f *fakeStore) ListCitationsForCheck(context.Context) ([]store.CitationCheckRow, error) {
	return f.rows, nil
}

func (f *fakeStore) CountUncheckableCitations(context.Context) (int, error) {
	return f.uncheckable, nil
}

func (f *fakeStore) MarkSourceReviewed(_ context.Context, id int64) error {
	if f.marks == nil {
		f.marks = map[int64]bool{}
	}
	f.marks[id] = true
	return nil
}

func (f *fakeStore) MarkQuotesChecked(_ context.Context, id int64, quotes []string) error {
	if f.stamped == nil {
		f.stamped = map[int64][]string{}
	}
	f.stamped[id] = append(f.stamped[id], quotes...)
	return nil
}

func TestRun_FilesDriftProposalsForMissingQuotes(t *testing.T) {
	pages := map[string]string{
		// source 1: both cited quotes still present (with different whitespace)
		"http://a": "A lessor shall,   within thirty days\nafter the termination, return the deposit. A receipt shall be given.",
		// source 2: cited quote is gone (statute rewritten)
		"http://b": "This section was repealed and replaced with entirely new language.",
	}
	const keyB = "11111111-1111-1111-1111-111111111111"
	fs := &fakeStore{rows: []store.CitationCheckRow{
		{SourceID: 1, URL: "http://a", Quote: "within thirty days after the termination", StatementKey: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"},
		{SourceID: 1, URL: "http://a", Quote: "A receipt shall be given", StatementKey: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"},
		{SourceID: 2, URL: "http://b", Quote: "the deposit must be returned within fourteen days", StatementKey: keyB},
		{SourceID: 3, URL: "http://c", Quote: "anything", StatementKey: "cccccccc-cccc-cccc-cccc-cccccccccccc"}, // fetch fails
	}, statements: map[string]store.ProposedStatement{
		keyB: {BodyMD: "Your deposit comes back within 14 days.", Citations: []store.ProposedCitation{{URL: "http://b", Quote: "the deposit must be returned within fourteen days"}}},
	}}
	fetch := func(u string) (string, error) {
		if u == "http://c" {
			return "", errors.New("boom")
		}
		return pages[u], nil
	}

	res, err := Run(context.Background(), fs, fetch, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Sources != 2 || res.Drifted != 1 || res.Proposed != 1 || res.Failed != 1 {
		t.Fatalf("result = %+v, want sources=2 drifted=1 proposed=1 failed=1", res)
	}
	if !fs.marks[1] || !fs.marks[2] {
		t.Error("both fetched sources must be marked reviewed")
	}
	if _, ok := fs.marks[3]; ok {
		t.Error("source 3 (fetch failed) must not be marked reviewed")
	}
	if len(fs.filed) != 1 {
		t.Fatalf("filed %d proposals, want 1 (source 2's missing quote)", len(fs.filed))
	}
	p := fs.filed[0]
	if p.StatementKey != keyB || p.Reason != "source-drift" || p.ProposedBy != store.ActorSourceCheck {
		t.Errorf("proposal = key %s reason %s by %s", p.StatementKey, p.Reason, p.ProposedBy)
	}
	// A repeal shares no words with the old quote: no replacement is
	// offered, the passage rides along as evidence only.
	if p.Proposed != nil {
		t.Errorf("a repealed section got a proposed replacement %+v; the reviewer must do this one by hand", p.Proposed)
	}
	if !strings.Contains(string(p.Evidence), `"old_quote":"the deposit must be returned within fourteen days"`) {
		t.Errorf("evidence lacks the missing quote: %s", p.Evidence)
	}
	if got := len(fs.stamped[1]); got != 2 {
		t.Errorf("source 1: %d quote(s) stamped checked, want both", got)
	}
	if got := len(fs.stamped[2]); got != 0 {
		t.Errorf("source 2: %d quote(s) stamped checked, want none — its quote is gone and must keep its old stamp", got)
	}
	if got := len(fs.stamped[3]); got != 0 {
		t.Errorf("source 3: %d quote(s) stamped checked, want none — the fetch failed, nothing was examined", got)
	}
}

// The failure this guards against is a silent one: quote-less citations are
// excluded from the check, so before Skipped existed a run that examined
// nothing reported exactly what a clean run reported.
func TestRun_ReportsCitationsItCouldNotCheck(t *testing.T) {
	fs := &fakeStore{
		uncheckable: 93,
		rows: []store.CitationCheckRow{
			{SourceID: 1, URL: "http://a", Quote: "still here"},
		},
	}
	var logged []string
	res, err := Run(context.Background(), fs,
		func(string) (string, error) { return "still here", nil },
		func(format string, a ...any) { logged = append(logged, fmt.Sprintf(format, a...)) },
	)
	if err != nil {
		t.Fatal(err)
	}
	if res.Skipped != 93 {
		t.Errorf("Skipped = %d, want 93", res.Skipped)
	}
	if res.Sources != 1 || res.Drifted != 0 {
		t.Errorf("result = %+v, want sources=1 drifted=0", res)
	}
	var warned bool
	for _, line := range logged {
		if strings.Contains(line, "93") {
			warned = true
		}
	}
	if !warned {
		t.Error("a run that skipped 93 citations must say so, not just report what it checked")
	}
}

func TestRun_NoSkipWhenEverythingIsQuoted(t *testing.T) {
	fs := &fakeStore{rows: []store.CitationCheckRow{{SourceID: 1, URL: "http://a", Quote: "x"}}}
	res, err := Run(context.Background(), fs,
		func(string) (string, error) { return "x", nil }, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Skipped != 0 {
		t.Errorf("Skipped = %d, want 0", res.Skipped)
	}
}

// A quote that moved slightly — a renumbered subsection, a word swapped —
// gets the nearest passage offered as the replacement, in the same statement
// with everything else untouched, and marked checked because it is verbatim
// in the text this run fetched.
func TestRun_OffersNearestPassageWhenClose(t *testing.T) {
	const key = "22222222-2222-2222-2222-222222222222"
	old := "the landlord shall return the security deposit within thirty days after the tenant vacates"
	page := "Preamble text here. (b) The landlord shall return the security deposit within twenty-one days after the tenant vacates the premises. (c) Interest accrues."
	fs := &fakeStore{
		rows: []store.CitationCheckRow{{SourceID: 1, URL: "http://a", Publisher: "State", Quote: old, StatementKey: key, Locator: "§ 5"}},
		statements: map[string]store.ProposedStatement{key: {
			BodyMD: "Your deposit comes back within 30 days.",
			Citations: []store.ProposedCitation{
				{URL: "http://a", Publisher: "State", Kind: "statute", Locator: "§ 5", Quote: old, Checked: true},
				{Editorial: true},
			},
		}},
	}
	res, err := Run(context.Background(), fs, func(string) (string, error) { return page, nil }, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Drifted != 1 || res.Proposed != 1 || len(fs.filed) != 1 {
		t.Fatalf("result = %+v, filed %d", res, len(fs.filed))
	}
	p := fs.filed[0]
	if p.Proposed == nil {
		t.Fatalf("no replacement offered; evidence: %s", p.Evidence)
	}
	got := p.Proposed.Citations[0]
	if !strings.Contains(got.Quote, "twenty-one days") || !got.Checked {
		t.Errorf("replacement citation = %+v, want the new passage, checked", got)
	}
	if p.Proposed.BodyMD != "Your deposit comes back within 30 days." || len(p.Proposed.Citations) != 2 || !p.Proposed.Citations[1].Editorial {
		t.Errorf("the rest of the statement changed: %+v", p.Proposed)
	}
	if len(fs.stamped[1]) != 0 {
		t.Error("the missing quote must not be stamped checked")
	}
}

func TestRun_DoesNotRefileDriftAPersonHasSeen(t *testing.T) {
	const key = "33333333-3333-3333-3333-333333333333"
	fs := &fakeStore{
		rows:        []store.CitationCheckRow{{SourceID: 1, URL: "http://a", Quote: "gone", StatementKey: key}},
		alreadyOpen: map[string]bool{key: true},
	}
	res, err := Run(context.Background(), fs, func(string) (string, error) { return "replaced", nil }, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Drifted != 1 || res.Proposed != 0 || len(fs.filed) != 0 {
		t.Errorf("result = %+v, filed %d; a waiting or rejected drift must not be filed again", res, len(fs.filed))
	}
}

func TestNearest(t *testing.T) {
	text := "alpha beta gamma delta epsilon zeta eta theta"
	got, score := Nearest(text, "Gamma, delta epsilon")
	if got != "gamma delta epsilon" || score != 1 {
		t.Errorf("Nearest = %q %.2f, want the exact window at 1.00", got, score)
	}
	_, score = Nearest(text, "completely unrelated words")
	if score != 0 {
		t.Errorf("unrelated quote scored %.2f, want 0", score)
	}
	if _, s := Nearest("", "x"); s != 0 {
		t.Error("empty text must score 0")
	}
}
