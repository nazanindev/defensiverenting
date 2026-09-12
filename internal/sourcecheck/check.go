// Package sourcecheck re-fetches cited primary sources and, where a verbatim
// quote we cited no longer appears — the precise signal that the law we
// relied on was amended — files a source-drift proposal against the statement
// citing it (ADR-014 D4). Checking our specific quotes (rather than hashing
// the whole page) avoids false alarms from dynamic page content: timestamps,
// banners, and session tokens change constantly; our cited statutory text
// does not, unless it actually changed.
//
// "Changed" is never inferred from "not found". Each fetch comes back as a
// drafting.Receipt saying which tier and extractor produced the text, and
// each citation carries the receipt recorded when its quote was last
// confirmed. A quote is declared missing only when the current text is
// readable (live, not a script shell or bot-check page) and comparable
// (produced by an extractor at least as good as the one that confirmed the
// quote). Anything else is reported as "could not check here", and the
// source keeps the stamp from the run that last actually read it.
package sourcecheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/nazanindev/defensiverenting/internal/drafting"
	"github.com/nazanindev/defensiverenting/internal/store"
)

// Result summarizes a check run.
type Result struct {
	Sources      int // sources read and examined
	Drifted      int // sources with at least one cited quote no longer found
	Proposed     int // source-drift proposals filed for the review queue
	Failed       int // sources that could not be fetched at all
	Unreadable   int // sources that answered with text too thin to examine (script shell, bot check)
	Incomparable int // citations whose quote was not found but whose baseline extractor outranks this run's, so nothing was concluded
	Skipped      int // citations carrying no quote, so nothing could be verified
	Unused       int // unused-source deletion proposals filed for the review queue
	Errored      int // sources whose check hit an error other than fetching; logged and skipped
}

// FetchFunc returns the readable text of a URL with its receipt (e.g.
// drafting.FetchExtract).
type FetchFunc func(url string) (drafting.Receipt, error)

// closeEnough is the similarity above which the nearest passage in the new
// fetch is offered as the replacement quote. Below it the proposal carries
// the passage as evidence only and the reviewer edits by hand.
const closeEnough = 0.6

// Run re-fetches every cited source once and confirms each verbatim quote cited
// from it still appears in the current page. Every confirmation is recorded:
// the source's last_checked_at and each confirmed citation's checked_at are
// stamped with this run's receipt, so "when was this last checked, and
// against what" is answerable per source and per statement, not just per run.
//
// A quote that went missing becomes a source-drift proposal against the
// statement citing it: the same statement with that citation's quote swapped
// for the nearest passage in the new text when the match is close, and a
// work item carrying the passage as evidence when it is not. The evidence
// shows the passage the quote sat in when it was confirmed beside the passage
// found now, and says whether the page text changed at all. The checker
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
	// Sources no page cites cannot drift, but each still costs a fetch and
	// clutters the picker. They are filed for deletion before the fetch loop
	// so the queue shows them the moment a run starts (ADR-014 D7).
	unused, err := db.FileUnusedSourceProposals(ctx, store.ActorSourceCheck)
	if err != nil {
		return Result{Skipped: skipped}, fmt.Errorf("file unused sources: %w", err)
	}
	if unused > 0 {
		logf("○ %d source(s) no page cites — filed for deletion under Proposed changes", unused)
	}
	rows, err := db.ListCitationsForCheck(ctx)
	if err != nil {
		return Result{Skipped: skipped, Unused: unused}, err
	}

	order, bySrc := groupBySource(rows)

	res := Result{Skipped: skipped, Unused: unused}
	for _, id := range order {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		// One source's failure is that source's problem. Until 2026-09-12 a
		// database error here ended the run, which is why weekly runs kept
		// stopping after a handful of sources and most were never rechecked.
		if err := checkSource(ctx, db, fetch, id, bySrc[id], logf, &res); err != nil {
			res.Errored++
			logf("✗ %s: %v (continuing)", bySrc[id].url, err)
		}
	}
	return res, nil
}

// agg is one source's cited quotes, gathered for a single fetch.
type agg struct {
	url, publisher string
	rows           []store.CitationCheckRow
}

// groupBySource groups cited quotes by source, preserving first-seen order.
func groupBySource(rows []store.CitationCheckRow) ([]int64, map[int64]*agg) {
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
	return order, bySrc
}

// RunSource is Run scoped to one source (ADR-018 D5): the review page's
// "recheck quotes" button. It files no unused-source proposals and counts
// no uncheckable citations; those are the whole-site run's concern.
func RunSource(ctx context.Context, db store.Store, fetch FetchFunc, sourceID int64, logf func(string, ...any)) (Result, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	rows, err := db.ListCitationsForCheck(ctx)
	if err != nil {
		return Result{}, err
	}
	var mine []store.CitationCheckRow
	for _, r := range rows {
		if r.SourceID == sourceID {
			mine = append(mine, r)
		}
	}
	var res Result
	if len(mine) == 0 {
		return res, nil
	}
	_, bySrc := groupBySource(mine)
	return res, checkSource(ctx, db, fetch, sourceID, bySrc[sourceID], logf, &res)
}

// checkSource fetches one source and confirms, files, or stamps each quote
// cited from it, accumulating into res.
func checkSource(ctx context.Context, db store.Store, fetch FetchFunc, id int64, a *agg, logf func(string, ...any), res *Result) error {
	rc, err := fetch(a.url)
	if err != nil {
		res.Failed++
		logf("✗ %s: %v", a.url, err)
		if err := db.MarkSourceUnreadable(ctx, id, "fetch failed: "+err.Error()); err != nil {
			return err
		}
		return nil
	}
	if !rc.Live() {
		// FetchExtract never returns a snapshot; a test fetcher might.
		res.Failed++
		logf("✗ %s: only a %s was available, which says nothing about the live page", a.url, rc.Describe())
		if err := db.MarkSourceUnreadable(ctx, id, "only a snapshot was available ("+rc.Describe()+")"); err != nil {
			return err
		}
		return nil
	}

	var present []store.QuoteConfirmation
	var missing, unchecked []store.CitationCheckRow
	for _, r := range a.rows {
		switch {
		case r.CheckedHash != "" && r.CheckedHash == rc.Hash:
			// The page text is byte-for-byte what the quote was confirmed
			// in. No matching needed, and no matcher can disagree.
			present = append(present, store.QuoteConfirmation{Quote: r.Quote, Context: r.CheckedContext})
		case drafting.QuoteAppearsIn(rc.Text, r.Quote):
			present = append(present, store.QuoteConfirmation{Quote: r.Quote, Context: drafting.Context(rc.Text, r.Quote)})
		case rc.Thin:
			// A quote absent from a script shell or a bot-check page is
			// not evidence of anything.
			unchecked = append(unchecked, r)
		case !drafting.Comparable(r.CheckedExtractor, rc.Extractor):
			res.Incomparable++
			logf("  ? %s: quote confirmed under %s cannot be judged missing by %s — not comparable here: %q", a.url, r.CheckedExtractor, rc.Extractor, clip(r.Quote, 80))
		default:
			missing = append(missing, r)
		}
	}

	fetchReceipt := store.CheckReceipt{Via: rc.Tier, Extractor: rc.Extractor, Hash: rc.Hash}
	// Stamp the quotes this fetch confirmed, even on a drifted or thin
	// source: the quotes found were checked and found intact, and only
	// the others keep their older stamp.
	if err := db.MarkQuotesChecked(ctx, id, fetchReceipt, present); err != nil {
		return err
	}
	if len(unchecked) > 0 {
		res.Unreadable++
		note := fmt.Sprintf("page too thin to examine (%s): %d of %d quote(s) could not be checked", rc.Describe(), len(unchecked), len(a.rows))
		logf("⚠ %s — %s", a.url, note)
		if err := db.MarkSourceUnreadable(ctx, id, note); err != nil {
			return err
		}
		if len(present) == 0 {
			return nil
		}
	}
	if len(unchecked) == 0 {
		if err := db.MarkSourceChecked(ctx, id, rc.Describe()); err != nil {
			return err
		}
		res.Sources++
	}
	if len(missing) == 0 {
		if len(unchecked) == 0 {
			logf("· %s — all %d quote(s) still present (%s)", a.url, len(a.rows), rc.Describe())
		}
		return nil
	}
	res.Drifted++
	logf("⚑ %s — %d of %d cited quote(s) no longer found (%s)", a.url, len(missing), len(a.rows), rc.Describe())
	filed, err := fileDrift(ctx, db, a.url, a.publisher, rc, missing, logf)
	if err != nil {
		return err
	}
	res.Proposed += filed
	return nil
}

// fileDrift files one proposal per (statement, missing quote) on a source.
func fileDrift(ctx context.Context, db store.Store, url, publisher string, rc drafting.Receipt, missing []store.CitationCheckRow, logf func(string, ...any)) (int, error) {
	hay := normalize(rc.Text)
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
			"old_context": r.CheckedContext, "new_context": drafting.Context(rc.Text, passage),
			"fetched_via": rc.Describe(), "confirmed_via": r.CheckedExtractor,
			"text_changed": textChanged(r.CheckedHash, rc.Hash),
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
					// The passage is verbatim in the live text this run
					// fetched, so the approval may fall back to the
					// checker's word if it cannot read the source itself.
					st.Citations[i].Quote = passage
					st.Citations[i].Checked = true
					st.Citations[i].CheckedVia = rc.Describe()
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
		_, err = db.FileProposal(ctx, store.FileProposalParams{
			StatementKey: r.StatementKey, Reason: "source-drift", Proposed: proposed,
			Evidence: ev, ProposedBy: store.ActorSourceCheck,
		})
		if errors.Is(err, store.ErrNotFound) {
			// The statement left every live and draft page between the
			// listing and this filing: a save or a delete mid-run. Nothing
			// to file against, and no reason to abandon the other sources.
			logf("  · statement %s is no longer on any page; nothing filed", r.StatementKey)
			continue
		}
		if err != nil {
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

// textChanged says what the hashes prove about the page: "yes" when the text
// the quote was confirmed in differs from the text fetched now, and "unknown"
// when no baseline was recorded (the quote was confirmed before receipts, or
// attested by hand). It is never "no": a quote missing from unchanged text
// cannot happen, since an equal hash is accepted before any matching.
func textChanged(baseline, now string) string {
	if baseline == "" {
		return "unknown"
	}
	if baseline != now {
		return "yes"
	}
	return "no"
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

// normalize collapses whitespace so Nearest tolerates layout changes.
func normalize(s string) string { return strings.Join(strings.Fields(s), " ") }

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
