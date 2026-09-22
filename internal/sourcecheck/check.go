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
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/nazanindev/defensiverenting/internal/drafting"
	"github.com/nazanindev/defensiverenting/internal/store"
)

// Result summarizes a check run.
type Result struct {
	Sources      int // sources read and examined
	Drifted      int // sources with at least one cited quote no longer found
	Proposed     int // source-drift proposals filed for the review queue
	Closed       int // waiting source-drift proposals closed because the quote was found again
	Failed       int // sources that could not be fetched at all
	Unreadable   int // sources that answered with text too thin to examine (script shell, bot check)
	Incomparable int // citations whose quote was not found but whose baseline extractor outranks this run's, so nothing was concluded
	Skipped      int // citations carrying no quote, so nothing could be verified
	Moved        int // quotes that survived but whose surrounding text changed; a note was filed (ADR-024)
	Stale        int // statements whose dated claim is about to pass; a note was filed (ADR-024)
	Unused       int // unused-source deletion proposals filed for the review queue
	Errored      int // sources whose check hit an error other than fetching; logged and skipped
}

// StaleWindow is how far ahead of its date a dated claim is raised. Long
// enough that a person has time to decide and a page never goes wrong
// quietly, short enough that the queue is not full of next year's work.
const StaleWindow = 30 * 24 * time.Hour

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

	// Sources are checked Concurrency at a time. A fetch is mostly waiting
	// on a remote server, and a run over three hundred sources took most of
	// an hour one at a time. Renders still queue behind one Chrome (see
	// drafting.renderTier), so the overlap is in the direct fetches. Each
	// worker counts into its own Result and the log is serialised, so the
	// totals and the lines are the same as a sequential run would give,
	// only not in source order.
	var (
		mu   sync.Mutex
		res  = Result{Skipped: skipped, Unused: unused}
		wg   sync.WaitGroup
		slot = make(chan struct{}, max(Concurrency, 1))
	)
	log := func(format string, a ...any) {
		mu.Lock()
		defer mu.Unlock()
		logf(format, a...)
	}
	for _, id := range order {
		if err := ctx.Err(); err != nil {
			wg.Wait()
			return res, err
		}
		slot <- struct{}{}
		wg.Add(1)
		go func(id int64, a *agg) {
			defer wg.Done()
			defer func() { <-slot }()
			var local Result
			// One source's failure is that source's problem. Until 2026-09-12
			// a database error here ended the run, which is why weekly runs
			// kept stopping after a handful of sources and most were never
			// rechecked.
			if err := checkSource(ctx, db, fetch, id, a, log, &local); err != nil {
				local.Errored++
				log("✗ %s: %v (continuing)", a.url, err)
			}
			mu.Lock()
			res.add(local)
			mu.Unlock()
		}(id, bySrc[id])
	}
	wg.Wait()
	// A claim that depends on a date goes stale with no source change at
	// all, so it is checked here rather than per source (ADR-024). The
	// window is ahead of the date: the fix belongs in the queue while the
	// page is still right.
	stale, serr := db.StatementsGoingStale(ctx, StaleWindow)
	if serr != nil {
		logf("stale statements: %v", serr)
	}
	for _, st := range stale {
		note := fmt.Sprintf("This statement's claim stops being true on %s. Read it against the law as it will read then, and say what replaces it.", st.StaleAfter.Format("2006-01-02"))
		if err := db.FileCheckerNote(ctx, st.Key, note); err != nil {
			logf("  stale note on %s: %v", st.Key, err)
			continue
		}
		res.Stale++
	}
	return res, nil
}

// Concurrency is how many sources a run fetches at once.
var Concurrency = 4

// add folds another Result's counts into r.
func (r *Result) add(o Result) {
	r.Sources += o.Sources
	r.Drifted += o.Drifted
	r.Proposed += o.Proposed
	r.Closed += o.Closed
	r.Failed += o.Failed
	r.Unreadable += o.Unreadable
	r.Incomparable += o.Incomparable
	r.Skipped += o.Skipped
	r.Moved += o.Moved
	r.Stale += o.Stale
	r.Unused += o.Unused
	r.Errored += o.Errored
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
			now := drafting.Context(rc.Text, r.Quote)
			present = append(present, store.QuoteConfirmation{Quote: r.Quote, Context: now})
			// The quote survived, but the law around it may not have. An
			// amendment that adds an exception leaves every stored quote
			// intact and makes the statement incomplete, which no
			// quote-presence check can see (ADR-024). Compare the passage
			// the quote sat in; when it moved, the statement is re-read.
			if moved(r, rc, now) {
				if err := db.FileCheckerNote(ctx, r.StatementKey, store.NoteSourceMoved); err != nil {
					logf("  note on %s: %v", r.StatementKey, err)
				} else {
					res.Moved++
					logf("  the text around a quote moved: %s", r.URL)
				}
			}
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

	// A quote found again closes the drift finding it once raised: the
	// queue should not ask a person to decide what the page has answered.
	for _, r := range a.rows {
		if r.StatementKey == "" || !slices.ContainsFunc(present, func(q store.QuoteConfirmation) bool { return q.Quote == r.Quote }) {
			continue
		}
		n, err := db.CloseDriftFoundAgain(ctx, r.StatementKey, r.Quote, rc.Describe())
		if err != nil {
			return fmt.Errorf("close drift found again: %w", err)
		}
		if n > 0 {
			res.Closed += n
			logf("  ✓ %s: quote is on the page again; %d waiting drift item(s) closed", a.url, n)
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
// moved reports whether the passage a quote sits in changed since the last
// check. It compares only what is comparable: both baselines present, the
// same extractor family, and the page text not identical to what was stored
// (that case is handled above). Whitespace and typography are folded, since
// extractors differ on both without the law differing.
func moved(r store.CitationCheckRow, rc drafting.Receipt, now string) bool {
	if r.CheckedContext == "" || now == "" || r.CheckedHash == "" {
		return false // no baseline to compare against
	}
	if r.CheckedHash == rc.Hash {
		return false // the same page text
	}
	if !drafting.Comparable(r.CheckedExtractor, rc.Extractor) {
		return false // a different extractor reads the same page differently
	}
	return foldPassage(r.CheckedContext) != foldPassage(now)
}

// foldPassage normalizes a passage for comparison: typography folded the way
// the verbatim matcher folds it, whitespace collapsed. What is left is the
// words, so a difference is a difference in the law's text.
func foldPassage(s string) string {
	return strings.Join(strings.Fields(drafting.FoldTypography(s)), " ")
}

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

// Nearest finds the passage in text closest to quote and returns it with a
// similarity in [0, 1]. Windows the quote's length in words are compared as
// word bags, so a rewrite that kept most of the words scores high and a
// repeal scores near zero; the best window is then widened or trimmed to
// the nearest sentence boundaries (snapToSentences) so what is offered as
// the new quote reads as the page's own sentences rather than a run of
// words cut mid-thought. The similarity is the window's, before snapping.
// text is expected normalized.
//
// Text whose words arrive fused ("THELANDLORDANDTENANTACT", a PDF that
// encodes spacing by glyph position) has no words to compare, and the
// nearest window of it is noise. Nearest returns nothing for it.
func Nearest(text, quote string) (string, float64) {
	words := strings.Fields(text)
	qw := strings.Fields(normalize(quote))
	if len(words) == 0 || len(qw) == 0 || fused(words) {
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
	start, end := snapToSentences(words, bestAt, bestAt+n)
	return strings.Join(words[start:end], " "), best
}

// fused reports text whose words were never separated: more than a fifth
// of them longer than any English word has a right to be.
func fused(words []string) bool {
	long := 0
	for _, w := range words {
		if len(w) > 30 {
			long++
		}
	}
	return long*5 > len(words)
}

// snapToSentences moves the window [start, end) to sentence boundaries when
// one is near. Each edge looks up to slack words in both directions and
// takes the nearer boundary, preferring to widen on a tie, so a window that
// ends three words short of a full stop grows to include it and one that
// opens with the tail of the previous sentence sheds it. An edge with no
// boundary within reach stays where the match put it. The window never
// collapses: if snapping would empty it, the original is returned.
func snapToSentences(words []string, start, end int) (int, int) {
	n := end - start
	slack := 15 + n/2
	ends := func(i int) bool { return i >= 0 && i < len(words) && sentenceEnd(words, i) }
	if !anySentenceEnd(words) {
		// Text with no sentences (a list, a fixture) has nothing to snap to.
		return start, end
	}
	// A start is a boundary when the word before it ended a sentence.
	newStart := start
	if start > 0 && !ends(start-1) {
		back, fwd := -1, -1
		for d := 1; d <= slack && start-1-d >= 0; d++ {
			if ends(start - 1 - d) {
				back = start - d
				break
			}
		}
		if back < 0 && start <= slack {
			back = 0 // the text opens within reach
		}
		for d := 1; d <= slack && start-1+d < end-1; d++ {
			if ends(start - 1 + d) {
				fwd = start + d
				break
			}
		}
		switch {
		case back >= 0 && (fwd < 0 || start-back <= fwd-start):
			newStart = back
		case fwd >= 0:
			newStart = fwd
		}
	}
	newEnd := end
	if end < len(words) && !ends(end-1) {
		back, fwd := -1, -1
		for d := 1; d <= slack && end-1+d < len(words); d++ {
			if ends(end - 1 + d) {
				fwd = end + d
				break
			}
		}
		if fwd < 0 && len(words)-end <= slack {
			fwd = len(words) // the text closes within reach
		}
		for d := 1; d <= slack && end-1-d > newStart; d++ {
			if ends(end - 1 - d) {
				back = end - d
				break
			}
		}
		switch {
		case fwd >= 0 && (back < 0 || fwd-end <= end-back):
			newEnd = fwd
		case back >= 0:
			newEnd = back
		}
	}
	if newEnd <= newStart {
		return start, end
	}
	return newStart, newEnd
}

func anySentenceEnd(words []string) bool {
	for i := range words {
		if sentenceEnd(words, i) {
			return true
		}
	}
	return false
}

// sentenceEnd reports whether words[i] closes a sentence: it ends in a full
// stop, question mark, or exclamation mark (closing quotes and brackets
// allowed after), and the next word, if any, opens with a capital or a
// bracket. Statutory text is thick with "Sec. 5", "P.L. 69" and "No. 20",
// so a number after a stop does not split.
func sentenceEnd(words []string, i int) bool {
	w := strings.TrimRight(words[i], "\"'”’)]")
	if w == "" || !strings.ContainsRune(".!?", rune(w[len(w)-1])) {
		return false
	}
	if i+1 >= len(words) {
		return true
	}
	next := strings.TrimLeft(words[i+1], "\"'“‘([")
	if next == "" {
		return false
	}
	r := []rune(next)[0]
	return unicode.IsUpper(r) || r == '(' || r == '['
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
