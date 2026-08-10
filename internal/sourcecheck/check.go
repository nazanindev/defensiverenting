// Package sourcecheck re-fetches cited primary sources and flags any where a
// verbatim quote we cited no longer appears — the precise signal that the law
// we relied on was amended. Checking our specific quotes (rather than hashing
// the whole page) avoids false alarms from dynamic page content: timestamps,
// banners, and session tokens change constantly; our cited statutory text does
// not, unless it actually changed.
package sourcecheck

import (
	"context"
	"fmt"
	"strings"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// Result summarizes a check run.
type Result struct {
	Sources int // sources checked
	Flagged int // sources with at least one cited quote no longer found
	Failed  int // sources that could not be fetched
	Skipped int // citations carrying no quote, so nothing could be verified
}

// FetchFunc returns the readable text of a URL (e.g. drafting.FetchExtract).
type FetchFunc func(url string) (string, error)

// Run re-fetches every cited source once and confirms each verbatim quote cited
// from it still appears in the current page. A source with any missing quote is
// flagged for the author to re-verify. Fetch failures are counted and skipped.
//
// Result.Skipped reports the citations that carry no quote and so cannot be
// verified at all. They are counted separately and up front, because the
// alternative — reporting only what was examined — makes a run that checked
// nothing indistinguishable from a clean one.
func Run(ctx context.Context, db store.Store, fetch FetchFunc, logf func(string, ...any)) (Result, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	skipped, err := db.CountUncheckableCitations(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("count uncheckable citations: %w", err)
	}
	if skipped > 0 {
		logf("⚠ %d citation(s) carry no verbatim quote and cannot be checked — they are not covered by this run", skipped)
	}
	rows, err := db.ListCitationsForCheck(ctx)
	if err != nil {
		return Result{Skipped: skipped}, err
	}

	// Group cited quotes by source, preserving first-seen order.
	type agg struct {
		url    string
		quotes []string
	}
	var order []int64
	bySrc := map[int64]*agg{}
	for _, r := range rows {
		a, ok := bySrc[r.SourceID]
		if !ok {
			a = &agg{url: r.URL}
			bySrc[r.SourceID] = a
			order = append(order, r.SourceID)
		}
		a.quotes = append(a.quotes, r.Quote)
	}

	res := Result{Skipped: skipped}
	for _, id := range order {
		a := bySrc[id]
		text, err := fetch(a.url)
		if err != nil {
			res.Failed++
			logf("✗ %s: %v", a.url, err)
			continue
		}
		hay := normalize(text)
		missing := 0
		for _, q := range a.quotes {
			if !strings.Contains(hay, normalize(q)) {
				missing++
			}
		}
		changed := missing > 0
		if err := db.MarkSourceReviewed(ctx, id, changed); err != nil {
			return res, err
		}
		res.Sources++
		if changed {
			res.Flagged++
			logf("⚑ %s — %d of %d cited quote(s) no longer found", a.url, missing, len(a.quotes))
		} else {
			logf("· %s — all %d quote(s) still present", a.url, len(a.quotes))
		}
	}
	return res, nil
}

// normalize collapses whitespace so the quote match tolerates layout changes,
// matching how the drafting guardrail verifies quotes at save time.
func normalize(s string) string { return strings.Join(strings.Fields(s), " ") }
