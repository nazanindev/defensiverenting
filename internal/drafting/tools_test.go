package drafting

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// fakeStore implements only the store.Store methods SaveDraft touches; the
// embedded nil interface panics if any other method is called (none are here).
type fakeStore struct {
	store.Store
	ingested       *store.IngestPlaybookParams
	nextSrcID      int64
	publishedTopic bool // when true, GetPlaybook reports an existing published playbook
	unknownTopic   bool // when true, GetTopicBySlug reports the slug is not in the registry
}

func (f *fakeStore) GetPlaybook(_ context.Context, _, _, _ string) (store.PlaybookWithStatements, error) {
	if f.publishedTopic {
		return store.PlaybookWithStatements{}, nil
	}
	return store.PlaybookWithStatements{}, store.ErrNotFound
}

func (f *fakeStore) GetJurisdictionBySlug(_ context.Context, slug string) (store.Jurisdiction, error) {
	if slug == "boston" {
		return store.Jurisdiction{ID: 1, Kind: "city", Name: "Boston", Slug: "boston"}, nil
	}
	return store.Jurisdiction{}, store.ErrNotFound
}

func (f *fakeStore) UpsertSource(_ context.Context, p store.UpsertSourceParams) (store.Source, error) {
	f.nextSrcID++
	return store.Source{ID: f.nextSrcID, URL: p.URL, Publisher: p.Publisher, Kind: p.Kind}, nil
}

// Topics are a closed registry: SaveDraft looks one up, it never creates one.
// unknownTopic makes the lookup miss, so the rejection path can be tested.
func (f *fakeStore) GetTopicBySlug(_ context.Context, slug string) (store.Topic, error) {
	if f.unknownTopic {
		return store.Topic{}, store.ErrNotFound
	}
	return store.Topic{ID: 7, Slug: slug, Name: "Security Deposits"}, nil
}

func (f *fakeStore) IngestPlaybook(_ context.Context, p store.IngestPlaybookParams) error {
	f.ingested = &p
	return nil
}

func newTestToolbelt(fs *fakeStore, pages map[string]string) *Toolbelt {
	tb := &Toolbelt{db: fs, cache: newFetchCache(), extract: htmlStripper{}}
	tb.fetch = func(u string) (fetched, error) {
		body, ok := pages[u]
		if !ok {
			return fetched{}, fmt.Errorf("no such page")
		}
		return fetched{Text: tb.extract.extract(body)}, nil
	}
	return tb
}

func mustFetch(t *testing.T, tb *Toolbelt, u string) {
	t.Helper()
	if _, err := tb.FetchSource(context.Background(), FetchSourceInput{URL: u}); err != nil {
		t.Fatalf("FetchSource(%s): %v", u, err)
	}
}

func isRejection(err error) bool {
	var re *RejectionError
	return errors.As(err, &re)
}

const depositURL = "https://malegislature.gov/Laws/GeneralLaws/PartII/TitleI/Chapter186/Section15B"

func stmt(body, url, quote string) StatementInput {
	return StatementInput{
		BodyMD:    body,
		Citations: []CitationInput{{URL: url, Publisher: "MA Legislature", Kind: "statute", Locator: "§ 15B", Quote: quote}},
	}
}

func TestSaveDraft_HappyPath(t *testing.T) {
	fs := &fakeStore{}
	tb := newTestToolbelt(fs, map[string]string{
		depositURL: `<p>A lessor shall, within thirty days after the termination of the tenancy, return the security deposit.</p>`,
	})
	mustFetch(t, tb, depositURL)

	out, err := tb.SaveDraft(context.Background(), SaveDraftInput{
		CitySlug:  "boston",
		TopicSlug: "security-deposits",
		Title:     "Boston Security Deposits",
		IntroMD:   "What Boston renters should know.",
		Statements: []StatementInput{
			stmt("Your landlord must return your deposit within 30 days of the tenancy ending.",
				depositURL, "within thirty days after the termination of the tenancy, return the security deposit"),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fs.ingested == nil {
		t.Fatal("IngestPlaybook was not called")
	}
	if fs.ingested.Status != "draft" {
		t.Errorf("status = %q, want draft", fs.ingested.Status)
	}
	if got := fs.ingested.Statements[0].Sources[0].Quote; got == "" {
		t.Error("citation quote was not plumbed through to ingest")
	}
	if got := fs.ingested.Statements[0].Sources[0].SourceID; got == 0 {
		t.Error("citation source id was not set")
	}
	if out.CitationCount != 1 || out.Status != "draft" {
		t.Errorf("output = %+v", out)
	}
}

func TestSaveDraft_RejectsFabricatedQuote(t *testing.T) {
	fs := &fakeStore{}
	tb := newTestToolbelt(fs, map[string]string{
		depositURL: `<p>A lessor shall, within thirty days, return the security deposit.</p>`,
	})
	mustFetch(t, tb, depositURL)

	_, err := tb.SaveDraft(context.Background(), SaveDraftInput{
		CitySlug: "boston", TopicSlug: "security-deposits", Title: "T",
		Statements: []StatementInput{
			// This quote never appears in the fetched text — the guardrail must reject it.
			stmt("Deposits must be returned within 14 days.", depositURL, "within fourteen days"),
		},
	})
	if !isRejection(err) {
		t.Fatalf("expected the fabricated quote to be rejected, got err=%v", err)
	}
	if fs.ingested != nil {
		t.Fatal("IngestPlaybook must NOT be called when a quote fails the verbatim check")
	}
}

func TestSaveDraft_RejectsUnfetchedURL(t *testing.T) {
	fs := &fakeStore{}
	tb := newTestToolbelt(fs, map[string]string{depositURL: `text`})
	// Note: no mustFetch — the URL is cited without ever being fetched.
	_, err := tb.SaveDraft(context.Background(), SaveDraftInput{
		CitySlug: "boston", TopicSlug: "security-deposits", Title: "T",
		Statements: []StatementInput{stmt("Claim.", depositURL, "text")},
	})
	if !isRejection(err) {
		t.Fatalf("expected rejection when citing a URL that was never fetched, got err=%v", err)
	}
	if fs.ingested != nil {
		t.Fatal("IngestPlaybook must not be called")
	}
}

func TestSaveDraft_WhitespaceTolerantMatch(t *testing.T) {
	fs := &fakeStore{}
	tb := newTestToolbelt(fs, map[string]string{
		// Source renders the clause across lines / with odd spacing.
		depositURL: "<div>within thirty days\n   after the   termination</div>",
	})
	mustFetch(t, tb, depositURL)

	_, err := tb.SaveDraft(context.Background(), SaveDraftInput{
		CitySlug: "boston", TopicSlug: "security-deposits", Title: "T",
		// Quote uses single spaces; must still match despite layout differences.
		Statements: []StatementInput{stmt("Claim.", depositURL, "within thirty days after the termination")},
	})
	if err != nil {
		t.Fatalf("whitespace-normalized quote should match, got err=%v", err)
	}
	if fs.ingested == nil {
		t.Fatal("expected save to succeed")
	}
}

// A research pass over a live page saves a proposed revision beside it. This
// used to be refused outright, because one row per slot meant writing a draft
// would have overwritten live legal content. Since migration 000015 a draft and
// a published page coexist, so the pass is allowed — but it must still write a
// DRAFT, leaving the live page untouched until a human publishes.
func TestSaveDraft_RevisesPublishedWithoutTouchingIt(t *testing.T) {
	fs := &fakeStore{publishedTopic: true}
	tb := newTestToolbelt(fs, map[string]string{depositURL: "within thirty days"})
	mustFetch(t, tb, depositURL)

	out, err := tb.SaveDraft(context.Background(), SaveDraftInput{
		CitySlug: "boston", TopicSlug: "security-deposits", Title: "T",
		Statements: []StatementInput{stmt("Claim.", depositURL, "within thirty days")},
	})
	if err != nil {
		t.Fatalf("revising a published page should be allowed, got err=%v", err)
	}
	if fs.ingested == nil {
		t.Fatal("the revision was not written")
	}
	if fs.ingested.Status != "draft" {
		t.Errorf("wrote status %q; a revision must be a draft or it would replace the live page",
			fs.ingested.Status)
	}
	if !strings.Contains(out.Message, "revision") {
		t.Errorf("the caller should be told this replaces a live page, got: %s", out.Message)
	}
}

func TestSaveDraft_RejectsUnknownCity(t *testing.T) {
	fs := &fakeStore{}
	tb := newTestToolbelt(fs, nil)
	_, err := tb.SaveDraft(context.Background(), SaveDraftInput{
		CitySlug: "atlantis", TopicSlug: "security-deposits", Title: "T",
		Statements: []StatementInput{stmt("Claim.", depositURL, "x")},
	})
	if !isRejection(err) {
		t.Fatalf("expected rejection for unknown city slug, got err=%v", err)
	}
}

// Topics are a closed registry, so a draft may reuse one but never invent one.
// A drafting run against a brand-new city used to get an empty list_topics and
// mint its own slug, which is how a second vocabulary for the same subjects
// came to exist. See docs/ADRs/ADR-005 D5.
func TestSaveDraft_RejectsTopicNotInRegistry(t *testing.T) {
	fs := &fakeStore{unknownTopic: true}
	tb := newTestToolbelt(fs, map[string]string{
		depositURL: `<p>A lessor shall, within thirty days after the termination of the tenancy, return the security deposit.</p>`,
	})
	mustFetch(t, tb, depositURL)

	_, err := tb.SaveDraft(context.Background(), SaveDraftInput{
		CitySlug:  "boston",
		TopicSlug: "deposit-return-rules",
		Title:     "Boston Deposit Rules",
		IntroMD:   "What Boston renters should know.",
		Statements: []StatementInput{
			stmt("Your landlord must return your deposit within 30 days of the tenancy ending.",
				depositURL, "within thirty days after the termination of the tenancy, return the security deposit"),
		},
	})
	var rej *RejectionError
	if !errors.As(err, &rej) {
		t.Fatalf("err = %v, want a RejectionError", err)
	}
	if !strings.Contains(rej.Error(), "list_topics") {
		t.Errorf("rejection should point the agent at list_topics, got: %s", rej.Error())
	}
	if fs.ingested != nil {
		t.Error("nothing should have been written")
	}
}
