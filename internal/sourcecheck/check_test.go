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
	marks       map[int64]bool
	stamped     map[int64][]string // sourceID -> quotes confirmed present and stamped
	uncheckable int
}

func (f *fakeStore) ListCitationsForCheck(context.Context) ([]store.CitationCheckRow, error) {
	return f.rows, nil
}

func (f *fakeStore) CountUncheckableCitations(context.Context) (int, error) {
	return f.uncheckable, nil
}

func (f *fakeStore) MarkSourceReviewed(_ context.Context, id int64, changed bool) error {
	if f.marks == nil {
		f.marks = map[int64]bool{}
	}
	f.marks[id] = changed
	return nil
}

func (f *fakeStore) MarkQuotesChecked(_ context.Context, id int64, quotes []string) error {
	if f.stamped == nil {
		f.stamped = map[int64][]string{}
	}
	f.stamped[id] = append(f.stamped[id], quotes...)
	return nil
}

func TestRun_FlagsSourcesWithMissingQuotes(t *testing.T) {
	pages := map[string]string{
		// source 1: both cited quotes still present (with different whitespace)
		"http://a": "A lessor shall,   within thirty days\nafter the termination, return the deposit. A receipt shall be given.",
		// source 2: cited quote is gone (statute rewritten)
		"http://b": "This section was repealed and replaced with entirely new language.",
	}
	fs := &fakeStore{rows: []store.CitationCheckRow{
		{SourceID: 1, URL: "http://a", Quote: "within thirty days after the termination"},
		{SourceID: 1, URL: "http://a", Quote: "A receipt shall be given"},
		{SourceID: 2, URL: "http://b", Quote: "the deposit must be returned within fourteen days"},
		{SourceID: 3, URL: "http://c", Quote: "anything"}, // fetch fails
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
	if res.Sources != 2 || res.Flagged != 1 || res.Failed != 1 {
		t.Fatalf("result = %+v, want sources=2 flagged=1 failed=1", res)
	}
	if fs.marks[1] {
		t.Error("source 1 (all quotes present, whitespace-tolerant) must not be flagged")
	}
	if !fs.marks[2] {
		t.Error("source 2 (cited quote missing) must be flagged")
	}
	if _, ok := fs.marks[3]; ok {
		t.Error("source 3 (fetch failed) must not be marked reviewed")
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
	if res.Sources != 1 || res.Flagged != 0 {
		t.Errorf("result = %+v, want sources=1 flagged=0", res)
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
