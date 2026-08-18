package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// A reviewer can now type a quote into the form. These tests cover the check
// that stands between what they typed and what a renter is told the law says.

// fakeQuoteStore reports which (url, quote) pairs are already stored.
type fakeQuoteStore struct {
	known map[string]bool
	calls int
}

func (f *fakeQuoteStore) CitationQuoteExists(_ context.Context, url, quote string) (bool, error) {
	f.calls++
	return f.known[url+"|"+quote], nil
}

func newTestVerifier(known map[string]bool, fetch func(string) (string, error)) (*quoteVerifier, *fakeQuoteStore) {
	fs := &fakeQuoteStore{known: known}
	qv := &quoteVerifier{fetch: fetch, quotes: fs, cache: newSourceFetchCache(time.Minute)}
	return qv, fs
}

func TestQuoteVerifier_acceptsAQuotePresentInTheSource(t *testing.T) {
	qv, _ := newTestVerifier(nil, func(string) (string, error) {
		return "The lessor shall, within thirty days after the termination of occupancy, return the deposit.", nil
	})
	if res := qv.check(context.Background(), 1, "https://law.example/15b",
		"within thirty days after the termination of occupancy"); res.Msg != "" {
		t.Fatalf("want accepted, got %q", res.Msg)
	}
}

func TestQuoteVerifier_rejectsAQuoteNotInTheSource(t *testing.T) {
	qv, _ := newTestVerifier(nil, func(string) (string, error) {
		return "The lessor shall return the deposit within thirty days.", nil
	})
	res := qv.check(context.Background(), 3, "https://law.example/15b", "within fourteen days")
	if res.Msg == "" {
		t.Fatal("a quote absent from the source must be refused")
	}
	if !strings.Contains(res.Msg, "Statement 3") {
		t.Errorf("message should name the statement so it can be found, got %q", res.Msg)
	}
	if res.Overridable {
		t.Error("a confirmed mismatch must never be overridable — the text was read and it is simply wrong")
	}
}

func TestQuoteVerifier_toleratesDifferentWhitespace(t *testing.T) {
	// Source text arrives wrapped and indented; a quote pasted from a browser
	// does not match it character for character, but the words are the same.
	qv, _ := newTestVerifier(nil, func(string) (string, error) {
		return "The lessor shall,\n   within thirty days\nafter the termination, return it.", nil
	})
	if res := qv.check(context.Background(), 1, "https://law.example/15b",
		"within thirty days after the termination"); res.Msg != "" {
		t.Fatalf("whitespace differences must not fail the check, got %q", res.Msg)
	}
}

func TestQuoteVerifier_skipsAQuoteAlreadyStored(t *testing.T) {
	// An unchanged quote was verified when it was written. Re-fetching every
	// source on every save would be slow, and would block edits whenever a
	// source is unreachable.
	fetched := false
	qv, _ := newTestVerifier(
		map[string]bool{"https://law.example/15b|already verified": true},
		func(string) (string, error) { fetched = true; return "", nil },
	)
	if res := qv.check(context.Background(), 1, "https://law.example/15b", "already verified"); res.Msg != "" {
		t.Fatalf("want accepted, got %q", res.Msg)
	}
	if fetched {
		t.Error("a quote already on file must not trigger a fetch")
	}
}

func TestQuoteVerifier_refusesWhenTheSourceCannotBeRead(t *testing.T) {
	// Some sources block the fetcher outright, such as the Massachusetts
	// sanitary code PDF. We cannot confirm the quote, so we do not accept it —
	// unless the reviewer overrides it, which is what Overridable signals.
	qv, _ := newTestVerifier(nil, func(string) (string, error) {
		return "", errors.New("status 403")
	})
	res := qv.check(context.Background(), 2, "https://mass.example/pdf", "68 degrees")
	if res.Msg == "" {
		t.Fatal("an unverifiable quote must be refused, not saved on trust")
	}
	if !strings.Contains(res.Msg, "403") {
		t.Errorf("message should say why it could not be checked, got %q", res.Msg)
	}
	if !res.Overridable {
		t.Error("a source that could not be reached at all must be overridable by a reviewer who checked it by hand")
	}
}

func TestQuoteVerifier_fetchesEachSourceOnce(t *testing.T) {
	n := 0
	qv, _ := newTestVerifier(nil, func(string) (string, error) {
		n++
		return "alpha beta gamma delta", nil
	})
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if res := qv.check(ctx, i+1, "https://law.example/x", "beta gamma"); res.Msg != "" {
			t.Fatalf("statement %d: %s", i+1, res.Msg)
		}
	}
	if n != 1 {
		t.Errorf("a page citing one source many times should fetch it once, got %d fetches", n)
	}
}

func TestQuoteVerifier_reportsAFailedFetchWithoutRetrying(t *testing.T) {
	n := 0
	qv, _ := newTestVerifier(nil, func(string) (string, error) {
		n++
		return "", errors.New("timeout")
	})
	ctx := context.Background()
	for i := 0; i < 4; i++ {
		if res := qv.check(ctx, i+1, "https://dead.example/x", "anything"); res.Msg == "" {
			t.Fatal("want refusal")
		}
	}
	if n != 1 {
		t.Errorf("a dead source should be tried once per save, got %d attempts", n)
	}
}

func TestQuoteVerifier_ignoresAnEmptyQuote(t *testing.T) {
	// A draft may be part-finished. Absence is caught at publish, where it
	// matters, rather than blocking a save mid-edit.
	qv, _ := newTestVerifier(nil, func(string) (string, error) {
		t.Fatal("an empty quote must not trigger a fetch")
		return "", nil
	})
	if res := qv.check(context.Background(), 1, "https://law.example/x", "   "); res.Msg != "" {
		t.Fatalf("want accepted, got %q", res.Msg)
	}
}

func TestQuoteVerifier_checkQuoteOmitsTheStatementPrefix(t *testing.T) {
	// The live per-citation check (fired on blur) has no statement to name —
	// check adds that prefix on top of checkQuote, not checkQuote itself.
	qv, _ := newTestVerifier(nil, func(string) (string, error) {
		return "The lessor shall return the deposit within thirty days.", nil
	})
	res := qv.checkQuote(context.Background(), "https://law.example/15b", "within fourteen days")
	if res.Msg == "" {
		t.Fatal("a quote absent from the source must be refused")
	}
	if strings.Contains(res.Msg, "Statement") {
		t.Errorf("checkQuote must not carry a statement number, got %q", res.Msg)
	}
}

func TestSourceFetchCache_sharedAcrossVerifierInstances(t *testing.T) {
	// The live check and the save-time check construct separate quoteVerifier
	// instances per request; sharing one cache is what keeps a source checked
	// live moments before save from being fetched a second time at save.
	n := 0
	fetch := func(string) (string, error) {
		n++
		return "alpha beta gamma delta", nil
	}
	cache := newSourceFetchCache(time.Minute)
	ctx := context.Background()

	live := &quoteVerifier{fetch: fetch, quotes: &fakeQuoteStore{}, cache: cache}
	if res := live.checkQuote(ctx, "https://law.example/shared", "beta gamma"); res.Msg != "" {
		t.Fatalf("live check: %s", res.Msg)
	}

	atSave := &quoteVerifier{fetch: fetch, quotes: &fakeQuoteStore{}, cache: cache}
	if res := atSave.check(ctx, 1, "https://law.example/shared", "beta gamma"); res.Msg != "" {
		t.Fatalf("save check: %s", res.Msg)
	}

	if n != 1 {
		t.Errorf("a source checked live and then saved should be fetched once total, got %d fetches", n)
	}
}

func TestSourceFetchCache_expiresAfterTTL(t *testing.T) {
	n := 0
	c := newSourceFetchCache(0) // expires immediately
	qv := &quoteVerifier{fetch: func(string) (string, error) {
		n++
		return "alpha beta gamma delta", nil
	}, quotes: &fakeQuoteStore{}, cache: c}
	ctx := context.Background()
	qv.checkQuote(ctx, "https://law.example/ttl", "beta gamma")
	qv.checkQuote(ctx, "https://law.example/ttl", "beta gamma")
	if n != 2 {
		t.Errorf("an expired cache entry should be re-fetched, got %d fetches for 2 checks", n)
	}
}
