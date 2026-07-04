package mcpserver

import (
	"context"
	"fmt"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// fakeStore implements only the store.Store methods save_draft_playbook touches;
// the embedded nil interface panics if any other method is called (none are here).
type fakeStore struct {
	store.Store
	ingested  *store.IngestPlaybookParams
	nextSrcID int64
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

func (f *fakeStore) UpsertTopic(_ context.Context, p store.UpsertTopicParams) (store.Topic, error) {
	return store.Topic{ID: 7, Slug: p.Slug, Name: p.Name}, nil
}

func (f *fakeStore) IngestPlaybook(_ context.Context, p store.IngestPlaybookParams) error {
	f.ingested = &p
	return nil
}

func newTestServer(fs *fakeStore, pages map[string]string) *Server {
	s := &Server{db: fs, cache: newFetchCache(), extract: htmlStripper{}}
	s.fetch = func(u string) (string, error) {
		body, ok := pages[u]
		if !ok {
			return "", fmt.Errorf("no such page")
		}
		return s.extract.extract(body), nil
	}
	return s
}

func mustFetch(t *testing.T, s *Server, u string) {
	t.Helper()
	res, _, err := s.fetchSource(context.Background(), nil, FetchSourceInput{URL: u})
	if err != nil {
		t.Fatalf("fetch_source(%s) transport error: %v", u, err)
	}
	if res != nil && res.IsError {
		t.Fatalf("fetch_source(%s) rejected: %v", u, res.Content)
	}
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
	s := newTestServer(fs, map[string]string{
		depositURL: `<p>A lessor shall, within thirty days after the termination of the tenancy, return the security deposit.</p>`,
	})
	mustFetch(t, s, depositURL)

	res, out, err := s.saveDraftPlaybook(context.Background(), nil, SaveDraftInput{
		CitySlug:  "boston",
		TopicSlug: "security-deposits",
		TopicName: "Security Deposits",
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
	if res != nil && res.IsError {
		t.Fatalf("expected success, got rejection: %v", res.Content)
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
	s := newTestServer(fs, map[string]string{
		depositURL: `<p>A lessor shall, within thirty days, return the security deposit.</p>`,
	})
	mustFetch(t, s, depositURL)

	res, _, err := s.saveDraftPlaybook(context.Background(), nil, SaveDraftInput{
		CitySlug: "boston", TopicSlug: "security-deposits", Title: "T",
		Statements: []StatementInput{
			// This quote never appears in the fetched text — the guardrail must reject it.
			stmt("Deposits must be returned within 14 days.", depositURL, "within fourteen days"),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected the fabricated quote to be rejected, but save succeeded")
	}
	if fs.ingested != nil {
		t.Fatal("IngestPlaybook must NOT be called when a quote fails the verbatim check")
	}
}

func TestSaveDraft_RejectsUnfetchedURL(t *testing.T) {
	fs := &fakeStore{}
	s := newTestServer(fs, map[string]string{depositURL: `text`})
	// Note: no mustFetch — the URL is cited without ever being fetched.
	res, _, err := s.saveDraftPlaybook(context.Background(), nil, SaveDraftInput{
		CitySlug: "boston", TopicSlug: "security-deposits", Title: "T",
		Statements: []StatementInput{stmt("Claim.", depositURL, "text")},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected rejection when citing a URL that was never fetched")
	}
	if fs.ingested != nil {
		t.Fatal("IngestPlaybook must not be called")
	}
}

func TestSaveDraft_WhitespaceTolerantMatch(t *testing.T) {
	fs := &fakeStore{}
	s := newTestServer(fs, map[string]string{
		// Source renders the clause across lines / with odd spacing.
		depositURL: "<div>within thirty days\n   after the   termination</div>",
	})
	mustFetch(t, s, depositURL)

	res, _, err := s.saveDraftPlaybook(context.Background(), nil, SaveDraftInput{
		CitySlug: "boston", TopicSlug: "security-deposits", Title: "T",
		// Quote uses single spaces; must still match despite layout differences.
		Statements: []StatementInput{stmt("Claim.", depositURL, "within thirty days after the termination")},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != nil && res.IsError {
		t.Fatalf("whitespace-normalized quote should match, got rejection: %v", res.Content)
	}
	if fs.ingested == nil {
		t.Fatal("expected save to succeed")
	}
}

func TestSaveDraft_RejectsUnknownCity(t *testing.T) {
	fs := &fakeStore{}
	s := newTestServer(fs, nil)
	res, _, _ := s.saveDraftPlaybook(context.Background(), nil, SaveDraftInput{
		CitySlug: "atlantis", TopicSlug: "security-deposits", Title: "T",
		Statements: []StatementInput{stmt("Claim.", depositURL, "x")},
	})
	if res == nil || !res.IsError {
		t.Fatal("expected rejection for unknown city slug")
	}
}
