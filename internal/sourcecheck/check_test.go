package sourcecheck

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/drafting"
	"github.com/nazanindev/defensiverenting/internal/store"
)

type fakeStore struct {
	store.Store
	rows        []store.CitationCheckRow
	marks       map[int64]bool     // sourceID -> checked (read and examined)
	unreadable  map[int64]string   // sourceID -> note from a fetch that examined nothing
	stamped     map[int64][]string // sourceID -> quotes confirmed present and stamped
	receipts    map[int64]store.CheckReceipt
	uncheckable int
	statements  map[string]store.ProposedStatement // by key
	alreadyOpen map[string]bool                    // key -> a drift proposal is already waiting
	filed       []store.FileProposalParams
	unusedFiled int
}

// pages turns a url -> text map into a FetchFunc returning readable direct
// receipts. Test pages are a sentence long, which a real fetch would call
// thin; the thin path has its own tests below.
func pages(m map[string]string, failing ...string) FetchFunc {
	return func(u string) (drafting.Receipt, error) {
		for _, f := range failing {
			if u == f {
				return drafting.Receipt{}, errors.New("boom")
			}
		}
		return page(u, m[u]), nil
	}
}

func page(u, text string) drafting.Receipt {
	return drafting.Receipt{URL: u, Text: text, Tier: drafting.TierDirect, Extractor: drafting.ExtractorHTML, Chars: len(text), Hash: "hash:" + text}
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

func (f *fakeStore) MarkSourceChecked(_ context.Context, id int64, _ string) error {
	if f.marks == nil {
		f.marks = map[int64]bool{}
	}
	f.marks[id] = true
	return nil
}

func (f *fakeStore) MarkSourceUnreadable(_ context.Context, id int64, note string) error {
	if f.unreadable == nil {
		f.unreadable = map[int64]string{}
	}
	f.unreadable[id] = note
	return nil
}

func (f *fakeStore) MarkQuotesChecked(_ context.Context, id int64, rc store.CheckReceipt, quotes []store.QuoteConfirmation) error {
	if f.stamped == nil {
		f.stamped = map[int64][]string{}
		f.receipts = map[int64]store.CheckReceipt{}
	}
	for _, q := range quotes {
		f.stamped[id] = append(f.stamped[id], q.Quote)
	}
	if len(quotes) > 0 {
		f.receipts[id] = rc
	}
	return nil
}

func TestRun_FilesDriftProposalsForMissingQuotes(t *testing.T) {
	texts := map[string]string{
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
	res, err := Run(context.Background(), fs, pages(texts, "http://c"), nil)
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
	if !strings.Contains(fs.unreadable[3], "boom") {
		t.Errorf("source 3 must record why it could not be read, got %q", fs.unreadable[3])
	}
	if rc := fs.receipts[1]; rc.Via != drafting.TierDirect || rc.Extractor != drafting.ExtractorHTML || rc.Hash == "" {
		t.Errorf("source 1 stamps must carry the fetch receipt, got %+v", rc)
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
		func(u string) (drafting.Receipt, error) { return page(u, "still here"), nil },
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
		func(u string) (drafting.Receipt, error) { return page(u, "x"), nil }, nil)
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
	text := "Preamble text here. (b) The landlord shall return the security deposit within twenty-one days after the tenant vacates the premises. (c) Interest accrues."
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
	res, err := Run(context.Background(), fs, func(u string) (drafting.Receipt, error) { return page(u, text), nil }, nil)
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
	if !strings.Contains(got.Quote, "twenty-one days") || !got.Checked || !strings.Contains(got.CheckedVia, "direct fetch") {
		t.Errorf("replacement citation = %+v, want the new passage, checked, saying how", got)
	}
	for _, want := range []string{`"fetched_via":"direct fetch, html"`, `"text_changed":"unknown"`, `"new_context":"`} {
		if !strings.Contains(string(p.Evidence), want) {
			t.Errorf("evidence lacks %s: %s", want, p.Evidence)
		}
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
	res, err := Run(context.Background(), fs, func(u string) (drafting.Receipt, error) { return page(u, "replaced"), nil }, nil)
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

// The failure this guards against is the one that filled the queue with
// noise: a source answers 200 with a bot-check interstitial or a script
// shell, every cited quote is "missing" from it, and each becomes a drift
// proposal claiming the law may have been rewritten. A thin page proves
// nothing about its quotes.
func TestRun_thinPageIsUnreadableNotDrift(t *testing.T) {
	const key = "44444444-4444-4444-4444-444444444444"
	fs := &fakeStore{rows: []store.CitationCheckRow{
		{SourceID: 1, URL: "http://walled", Quote: "the landlord shall return the deposit", StatementKey: key},
	}}
	thin := func(u string) (drafting.Receipt, error) {
		rc := page(u, "Checking your browser before accessing the site.")
		rc.Thin = true
		return rc, nil
	}
	res, err := Run(context.Background(), fs, thin, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Drifted != 0 || res.Proposed != 0 || len(fs.filed) != 0 {
		t.Fatalf("result = %+v, filed %d; a thin page must file nothing", res, len(fs.filed))
	}
	if res.Unreadable != 1 || res.Sources != 0 {
		t.Errorf("result = %+v, want unreadable=1 sources=0", res)
	}
	if fs.marks[1] {
		t.Error("a source nothing was examined on must not be marked checked")
	}
	if !strings.Contains(fs.unreadable[1], "thin") {
		t.Errorf("the source must record why, got %q", fs.unreadable[1])
	}
}

// A quote found in a thin page is still found: a legitimately short page
// gets its stamp, and only the quotes that could not be judged are held.
func TestRun_quoteFoundInThinPageIsStamped(t *testing.T) {
	fs := &fakeStore{rows: []store.CitationCheckRow{
		{SourceID: 1, URL: "http://short", Quote: "returned within thirty days", StatementKey: "55555555-5555-5555-5555-555555555555"},
		{SourceID: 1, URL: "http://short", Quote: "something not on this page", StatementKey: "55555555-5555-5555-5555-555555555555"},
	}}
	thin := func(u string) (drafting.Receipt, error) {
		rc := page(u, "§ 2. The deposit is returned within thirty days.")
		rc.Thin = true
		return rc, nil
	}
	res, err := Run(context.Background(), fs, thin, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := fs.stamped[1]; len(got) != 1 || got[0] != "returned within thirty days" {
		t.Errorf("stamped %v, want only the quote that was found", got)
	}
	if res.Drifted != 0 || len(fs.filed) != 0 || res.Unreadable != 1 {
		t.Errorf("result = %+v, filed %d; the absent quote is unjudged, not drifted", res, len(fs.filed))
	}
}

// When the page text is exactly what the quote was confirmed in, the quote
// is present by construction. No matcher runs, so no matcher can disagree —
// which is what protects a fused-PDF quote from a checker that normalizes
// differently than the save path did.
func TestRun_unchangedTextIsPresentWithoutMatching(t *testing.T) {
	text := "THELANDLORDANDTENANTACT fused text a matcher might not find the quote in"
	fs := &fakeStore{rows: []store.CitationCheckRow{
		{SourceID: 1, URL: "http://pdf", Quote: "the landlord and tenant act", StatementKey: "66666666-6666-6666-6666-666666666666",
			CheckedExtractor: drafting.ExtractorPDFToText, CheckedHash: "hash:" + text, CheckedContext: "…the passage as recorded…"},
	}}
	res, err := Run(context.Background(), fs, func(u string) (drafting.Receipt, error) {
		rc := page(u, text)
		rc.Extractor = drafting.ExtractorPDFToText
		return rc, nil
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Drifted != 0 || len(fs.stamped[1]) != 1 {
		t.Errorf("result = %+v, stamped %v; equal hashes mean unchanged", res, fs.stamped[1])
	}
}

// A quote confirmed under pdftotext that the pure-Go reader cannot find is
// not drift: the reader is the weaker instrument. The run says so and moves
// on, leaving the older stamp in place.
func TestRun_weakerExtractorCannotDeclareDrift(t *testing.T) {
	fs := &fakeStore{rows: []store.CitationCheckRow{
		{SourceID: 1, URL: "http://pdf", Quote: "the landlord and tenant act", StatementKey: "77777777-7777-7777-7777-777777777777",
			CheckedExtractor: drafting.ExtractorPDFToText, CheckedHash: "hash:as-pdftotext-saw-it"},
	}}
	var logged []string
	res, err := Run(context.Background(), fs, func(u string) (drafting.Receipt, error) {
		rc := page(u, "garbled glyph soup from the pure go reader that shares no words")
		rc.Extractor = drafting.ExtractorPDFGo
		return rc, nil
	}, func(format string, a ...any) { logged = append(logged, fmt.Sprintf(format, a...)) })
	if err != nil {
		t.Fatal(err)
	}
	if res.Drifted != 0 || len(fs.filed) != 0 || res.Incomparable != 1 {
		t.Errorf("result = %+v, filed %d; want incomparable=1 and no drift", res, len(fs.filed))
	}
	if !fs.marks[1] {
		t.Error("the source was read; it is checked even though one quote could not be judged")
	}
	var said bool
	for _, l := range logged {
		if strings.Contains(l, "not comparable") {
			said = true
		}
	}
	if !said {
		t.Error("the run must say the comparison was skipped")
	}
}

// Drift evidence shows what changed, not only that something did: the
// passage the quote sat in when confirmed beside the passage found now, and
// whether the page text differs from the confirmed text at all.
func TestRun_driftEvidenceShowsOldAndNewPassages(t *testing.T) {
	const key = "88888888-8888-8888-8888-888888888888"
	old := "the landlord shall return the security deposit within thirty days"
	now := "Preamble. (b) The landlord shall return the security deposit within twenty-one days after the tenant vacates. (c) Interest."
	fs := &fakeStore{
		rows: []store.CitationCheckRow{{SourceID: 1, URL: "http://a", Quote: old, StatementKey: key,
			CheckedExtractor: drafting.ExtractorHTML, CheckedHash: "hash:the old page", CheckedContext: "…(b) " + old + " after the tenant vacates…"}},
		statements: map[string]store.ProposedStatement{key: {BodyMD: "b", Citations: []store.ProposedCitation{{URL: "http://a", Quote: old}}}},
	}
	if _, err := Run(context.Background(), fs, func(u string) (drafting.Receipt, error) { return page(u, now), nil }, nil); err != nil {
		t.Fatal(err)
	}
	if len(fs.filed) != 1 {
		t.Fatalf("filed %d", len(fs.filed))
	}
	ev := string(fs.filed[0].Evidence)
	for _, want := range []string{`"text_changed":"yes"`, `"old_context":"…(b) the landlord`, `"new_context":"Preamble. (b) The landlord`, `"confirmed_via":"html"`} {
		if !strings.Contains(ev, want) {
			t.Errorf("evidence lacks %s: %s", want, ev)
		}
	}
}
