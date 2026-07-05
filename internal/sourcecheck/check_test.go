package sourcecheck

import (
	"context"
	"errors"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

type fakeStore struct {
	store.Store
	rows  []store.CitationCheckRow
	marks map[int64]bool
}

func (f *fakeStore) ListCitationsForCheck(context.Context) ([]store.CitationCheckRow, error) {
	return f.rows, nil
}

func (f *fakeStore) MarkSourceReviewed(_ context.Context, id int64, changed bool) error {
	if f.marks == nil {
		f.marks = map[int64]bool{}
	}
	f.marks[id] = changed
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
}
