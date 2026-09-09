// Package sourcecheck re-fetches cited primary sources and, where a verbatim
// quote we cited no longer appears — the precise signal that the law we
// relied on was amended — files a source-drift proposal against the statement
// citing it (ADR-014 D4). Checking our specific quotes (rather than hashing
// the whole page) avoids false alarms from dynamic page content: timestamps,
// banners, and session tokens change constantly; our cited statutory text
// does not, unless it actually changed.
package sourcecheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// Result summarizes a check run.
type Result struct {
	Sources  int // sources checked
	Drifted  int // sources with at least one cited quote no longer found
	Proposed int // source-drift proposals filed for the review queue
	Failed   int // sources that could not be fetched
	Skipped  int // citations carrying no quote, so nothing could be verified
}

// FetchFunc returns the readable text of a URL (e.g. drafting.FetchExtract).
type FetchFunc func(url string) (string, error)

// closeEnough is the similarity above which the nearest passage in the new
// fetch is offered as the replacement quote. Below it the proposal carries
// the passage as evidence only and the reviewer edits by hand.
const closeEnough = 0.6

// Run re-fetches every cited source once and confirms each verbatim quote cited
// from it still appears in the current page. Every confirmation is recorded:
// the source's last_checked_at and each confirmed citation's checked_at are
// stamped, so "when was this last checked" is answerable per source and per
// statement, not just per run.
//
// A quote that went missing becomes a source-drift proposal against the
// statement citing it: the same statement with that citation's quote swapped
// for the nearest passage in the new text when the match is close, and a
// work item carrying the passage as evidence when it is not. The checker
// knows the quote vanished; it does not know the law. A person decides on
// the review queue. A statement whose drift is already waiting there, or was
// rejected for this same quote, is not filed again.
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
		url, publisher string
		rows           []store.CitationCheckRow
	}
	var order []int64
	bySrc := map[int64]*agg{}
	for _, r := range rows {
		a, ok := bySrc[r.SourceID]
		if !ok {
			a = &agg{url: r.URL, publisher: r.Publisher}
			bySrc[r.SourceID] = a
			order = append(order, r.SourceID)
		}
		a.rows = append(a.rows, r)
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
		var present []string
		var missing []store.CitationCheckRow
		for _, r := range a.rows {
			if strings.Contains(hay, normalize(r.Quote)) {
				present = append(present, r.Quote)
			} else {
				missing = append(missing, r)
			}
		}
		// Stamp the quotes this fetch confirmed, even on a drifted source: the
		// quotes still present were checked and found intact, and only the
		// missing ones keep their older stamp.
		if err := db.MarkQuotesChecked(ctx, id, present); err != nil {
			return res, err
		}
		if err := db.MarkSourceReviewed(ctx, id); err != nil {
			return res, err
		}
		res.Sources++
		if len(missing) == 0 {
			logf("· %s — all %d quote(s) still present", a.url, len(a.rows))
			continue
		}
		res.Drifted++
		logf("⚑ %s — %d of %d cited quote(s) no longer found", a.url, len(missing), len(a.rows))
		filed, err := fileDrift(ctx, db, a.url, a.publisher, hay, missing, logf)
		if err != nil {
			return res, err
		}
		res.Proposed += filed
	}
	return res, nil
}

// fileDrift files one proposal per (statement, missing quote) on a source.
func fileDrift(ctx context.Context, db store.Store, url, publisher, hay string, missing []store.CitationCheckRow, logf func(string, ...any)) (int, error) {
	filed := 0
	seen := map[string]bool{}
	for _, r := range missing {
		if r.StatementKey == "" || seen[r.StatementKey+"\x00"+r.Quote] {
			continue
		}
		seen[r.StatementKey+"\x00"+r.Quote] = true
		done, err := db.DriftAlreadyFiled(ctx, r.StatementKey, r.Quote)
		if err != nil {
			return filed, fmt.Errorf("check earlier drift proposals: %w", err)
		}
		if done {
			continue
		}
		passage, score := Nearest(hay, r.Quote)
		evidence := map[string]any{
			"source_url": url, "publisher": publisher, "locator": r.Locator,
			"old_quote": r.Quote, "new_quote": passage, "similarity": score,
		}
		var proposed *store.ProposedStatement
		if score >= closeEnough {
			st, err := db.StatementByKey(ctx, r.StatementKey)
			if errors.Is(err, store.ErrNotFound) {
				continue
			}
			if err != nil {
				return filed, fmt.Errorf("read statement %s: %w", r.StatementKey, err)
			}
			for i := range st.Citations {
				if st.Citations[i].URL == url && st.Citations[i].Quote == r.Quote {
					// The passage is verbatim in the text this run fetched,
					// so the approval can stamp it on the checker's word.
					st.Citations[i].Quote = passage
					st.Citations[i].Checked = true
				}
			}
			proposed = &st
			evidence["note"] = "The quoted text no longer appears at the source. The nearest passage in the current page is offered as the replacement quote; check that the statement is still true under it."
		} else {
			evidence["note"] = "The quoted text no longer appears at the source and nothing close to it was found. The law may have been rewritten or moved; re-verify the statement by hand."
		}
		ev, err := json.Marshal(evidence)
		if err != nil {
			return filed, err
		}
		if _, err := db.FileProposal(ctx, store.FileProposalParams{
			StatementKey: r.StatementKey, Reason: "source-drift", Proposed: proposed,
			Evidence: ev, ProposedBy: store.ActorSourceCheck,
		}); err != nil {
			return filed, fmt.Errorf("file drift proposal for %s: %w", r.StatementKey, err)
		}
		filed++
		if proposed != nil {
			logf("  → proposed replacement quote for statement %s (similarity %.2f)", r.StatementKey, score)
		} else {
			logf("  → filed as a finding for statement %s (nearest similarity %.2f)", r.StatementKey, score)
		}
	}
	return filed, nil
}

// Nearest finds the window of text closest to quote, comparing whitespace-
// normalized word bags, and returns it with a similarity in [0, 1]. Windows
// are the quote's length in words, so a rewrite that kept most of the words
// scores high and a repeal scores near zero. text is expected normalized.
func Nearest(text, quote string) (string, float64) {
	words := strings.Fields(text)
	qw := strings.Fields(normalize(quote))
	if len(words) == 0 || len(qw) == 0 {
		return "", 0
	}
	want := map[string]int{}
	for _, w := range qw {
		want[fold(w)]++
	}
	n := len(qw)
	if n > len(words) {
		n = len(words)
	}
	have := map[string]int{}
	overlap := 0 // sum over words of min(have, want)
	best, bestAt := -1.0, 0
	for i, w := range words {
		f := fold(w)
		have[f]++
		if have[f] <= want[f] {
			overlap++
		}
		if i >= n {
			g := fold(words[i-n])
			if have[g] <= want[g] {
				overlap--
			}
			have[g]--
		}
		if i >= n-1 {
			// Jaccard on multisets: |A∩B| / (|A|+|B|-|A∩B|).
			score := float64(overlap) / float64(len(qw)+n-overlap)
			if score > best {
				best, bestAt = score, i-n+1
			}
		}
	}
	return strings.Join(words[bestAt:bestAt+n], " "), best
}

// fold makes the word comparison forgiving of case and trailing punctuation,
// which a rewrite changes freely without changing the words.
func fold(w string) string {
	return strings.ToLower(strings.Trim(w, ".,;:()[]\"'“”‘’"))
}

// normalize collapses whitespace so the quote match tolerates layout changes,
// matching how the drafting guardrail verifies quotes at save time.
func normalize(s string) string { return strings.Join(strings.Fields(s), " ") }
