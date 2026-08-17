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

	// coveredLanguage, when set, is the only language GetPlaybook/
	// ListTopicsByJurisdiction report as having a page — lets a test tell a
	// published English page from a not-yet-translated Spanish one.
	coveredLanguage string
	lastLanguage    string // records the language GetPlaybook was last called with
}

func (f *fakeStore) GetPlaybook(_ context.Context, _, _, lang string) (store.PlaybookWithStatements, error) {
	f.lastLanguage = lang
	covered := f.coveredLanguage
	if covered == "" && f.publishedTopic {
		covered = "en" // legacy tests set publishedTopic without caring about language
	}
	if covered != "" && lang == covered {
		return store.PlaybookWithStatements{Playbook: store.Playbook{Title: "T", Language: lang}}, nil
	}
	return store.PlaybookWithStatements{}, store.ErrNotFound
}

func (f *fakeStore) ListTopicRegistry(_ context.Context) ([]store.Topic, error) {
	return []store.Topic{{ID: 7, Slug: "security-deposits", Name: "Security Deposits", IsCore: true}}, nil
}

func (f *fakeStore) ListTopicsByJurisdiction(_ context.Context, _ int64, lang string) ([]store.Topic, error) {
	if f.coveredLanguage != "" && lang == f.coveredLanguage {
		return []store.Topic{{ID: 7, Slug: "security-deposits", Name: "Security Deposits", IsCore: true}}, nil
	}
	return nil, nil
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

// ---- language ---------------------------------------------------------

// A draft that omits language must still default to English, so every
// existing caller (before language existed as a field) keeps working.
func TestSaveDraft_DefaultsToEnglishLanguage(t *testing.T) {
	fs := &fakeStore{}
	tb := newTestToolbelt(fs, map[string]string{
		depositURL: `<p>A lessor shall, within thirty days after the termination of the tenancy, return the security deposit.</p>`,
	})
	mustFetch(t, tb, depositURL)

	out, err := tb.SaveDraft(context.Background(), SaveDraftInput{
		CitySlug: "boston", TopicSlug: "security-deposits", Title: "T",
		Statements: []StatementInput{
			stmt("Your landlord must return your deposit within 30 days of the tenancy ending.",
				depositURL, "within thirty days after the termination of the tenancy, return the security deposit"),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Language != "en" || fs.ingested.Language != "en" {
		t.Errorf("output.Language=%q ingested.Language=%q, want en for both", out.Language, fs.ingested.Language)
	}
	if fs.ingested.Statements[0].Language != "en" {
		t.Errorf("statement language = %q, want en", fs.ingested.Statements[0].Language)
	}
}

// The citation-verbatim guardrail must not care what language body_md is
// written in: the quote still has to match the fetched source (which stays
// in whatever language it was actually published in, usually English).
func TestSaveDraft_SpanishDraftKeepsVerbatimGuardrail(t *testing.T) {
	fs := &fakeStore{}
	tb := newTestToolbelt(fs, map[string]string{
		depositURL: `<p>A lessor shall, within thirty days after the termination of the tenancy, return the security deposit.</p>`,
	})
	mustFetch(t, tb, depositURL)

	out, err := tb.SaveDraft(context.Background(), SaveDraftInput{
		CitySlug: "boston", TopicSlug: "security-deposits", Title: "El plazo de 30 días", Language: "es",
		Statements: []StatementInput{
			stmt("Su arrendador debe devolver el depósito en 30 días después de que termine el contrato.",
				depositURL, "within thirty days after the termination of the tenancy, return the security deposit"),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Language != "es" || fs.ingested.Language != "es" {
		t.Errorf("output.Language=%q ingested.Language=%q, want es for both", out.Language, fs.ingested.Language)
	}
	// The English citation quote must survive untouched inside a Spanish draft.
	if got := fs.ingested.Statements[0].Sources[0].Quote; got != "within thirty days after the termination of the tenancy, return the security deposit" {
		t.Errorf("citation quote = %q, want the verbatim English source text", got)
	}

	// Same statement, but the quote is fabricated: language must not bypass the guardrail.
	fs2 := &fakeStore{}
	tb2 := newTestToolbelt(fs2, map[string]string{depositURL: `<p>within thirty days</p>`})
	mustFetch(t, tb2, depositURL)
	_, err = tb2.SaveDraft(context.Background(), SaveDraftInput{
		CitySlug: "boston", TopicSlug: "security-deposits", Title: "T", Language: "es",
		Statements: []StatementInput{stmt("Reclamo.", depositURL, "dentro de treinta días")},
	})
	if !isRejection(err) {
		t.Fatalf("expected the fabricated (non-verbatim) quote to be rejected regardless of language, got err=%v", err)
	}
	if fs2.ingested != nil {
		t.Error("nothing should have been written")
	}
}

// A typo'd language code must fail loudly rather than silently drafting
// under the wrong full-text-search config with no lint applied — the same
// closed-registry discipline ADR-005 applies to slugs.
func TestSaveDraft_RejectsUnknownLanguage(t *testing.T) {
	fs := &fakeStore{}
	tb := newTestToolbelt(fs, map[string]string{depositURL: `<p>within thirty days</p>`})
	mustFetch(t, tb, depositURL)

	_, err := tb.SaveDraft(context.Background(), SaveDraftInput{
		CitySlug: "boston", TopicSlug: "security-deposits", Title: "T", Language: "sp",
		Statements: []StatementInput{stmt("Claim.", depositURL, "within thirty days")},
	})
	if !isRejection(err) {
		t.Fatalf("expected rejection for unsupported language %q, got err=%v", "sp", err)
	}
	if fs.ingested != nil {
		t.Error("nothing should have been written")
	}
}

// The Spanish voice lint is a separate ruleset from the English one, and it
// must actually run — not be silently skipped because the draft isn't English.
func TestSaveDraft_SpanishVoiceLintCatchesBannedWord(t *testing.T) {
	fs := &fakeStore{}
	tb := newTestToolbelt(fs, map[string]string{depositURL: `<p>within thirty days</p>`})
	mustFetch(t, tb, depositURL)

	_, err := tb.SaveDraft(context.Background(), SaveDraftInput{
		CitySlug: "boston", TopicSlug: "security-deposits", Title: "T", Language: "es",
		Statements: []StatementInput{
			stmt("Usted renuncia a este derecho si no responde.", depositURL, "within thirty days"),
		},
	})
	var rej *RejectionError
	if !errors.As(err, &rej) {
		t.Fatalf("err = %v, want a RejectionError from the Spanish voice lint", err)
	}
	if !strings.Contains(rej.Error(), "renunci") {
		t.Errorf("rejection should flag the banned word, got: %s", rej.Error())
	}
	if fs.ingested != nil {
		t.Error("nothing should have been written")
	}
}

// GetPlaybook and ListTopics both default to English and thread an explicit
// language through to the store, rather than silently always reading "en" —
// otherwise a translate workflow could never check whether a Spanish version
// already exists.
func TestGetPlaybook_LanguageDefaultsAndThreadsThrough(t *testing.T) {
	fs := &fakeStore{coveredLanguage: "en"}
	tb := newTestToolbelt(fs, nil)

	if _, err := tb.GetPlaybook(context.Background(), GetPlaybookInput{CitySlug: "boston", TopicSlug: "security-deposits"}); err != nil {
		t.Fatalf("default language: unexpected error: %v", err)
	}
	if fs.lastLanguage != "en" {
		t.Errorf("default language sent to store = %q, want en", fs.lastLanguage)
	}

	out, err := tb.GetPlaybook(context.Background(), GetPlaybookInput{CitySlug: "boston", TopicSlug: "security-deposits", Language: "es"})
	if !isRejection(err) {
		t.Fatalf("Spanish version doesn't exist yet: want a not-found rejection, got out=%+v err=%v", out, err)
	}
	if fs.lastLanguage != "es" {
		t.Errorf("explicit language sent to store = %q, want es", fs.lastLanguage)
	}
}

func TestListTopics_LanguageControlsHasPage(t *testing.T) {
	fs := &fakeStore{coveredLanguage: "en"}
	tb := newTestToolbelt(fs, nil)

	out, err := tb.ListTopics(context.Background(), ListTopicsInput{CitySlug: "boston"})
	if err != nil {
		t.Fatalf("default language: unexpected error: %v", err)
	}
	if !out.Topics[0].HasPage {
		t.Error("security-deposits has an English page and default language is en; want has_page=true")
	}

	out, err = tb.ListTopics(context.Background(), ListTopicsInput{CitySlug: "boston", Language: "es"})
	if err != nil {
		t.Fatalf("es language: unexpected error: %v", err)
	}
	if out.Topics[0].HasPage {
		t.Error("no Spanish page exists yet; want has_page=false when asking about es")
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

// ---- page kinds ------------------------------------------------------------

const helpOrgURL = "https://example.org/legal-aid"

func orgStmt(body, quote string) StatementInput {
	return StatementInput{
		BodyMD: body,
		Citations: []CitationInput{{
			URL: helpOrgURL, Publisher: "Example Legal Aid", Kind: "nonprofit", Quote: quote,
		}},
	}
}

func directoryToolbelt(t *testing.T, fs *fakeStore) *Toolbelt {
	t.Helper()
	tb := newTestToolbelt(fs, map[string]string{
		helpOrgURL: `<p>We provide free legal help to tenants in Allegheny County.</p>`,
	})
	mustFetch(t, tb, helpOrgURL)
	return tb
}

// A directory page is the whole point of page_kind: "where to get help" belongs
// on one page per city rather than repeated at the foot of every playbook.
func TestSaveDraft_SavesADirectoryPage(t *testing.T) {
	fs := &fakeStore{}
	tb := directoryToolbelt(t, fs)

	out, err := tb.SaveDraft(context.Background(), SaveDraftInput{
		CitySlug:  "boston",
		TopicSlug: "resource-directory",
		Title:     "Where to get help in Boston",
		IntroMD:   "Local organisations that help renters.",
		PageKind:  "directory",
		Statements: []StatementInput{
			orgStmt("Example Legal Aid gives free legal help to renters.",
				"We provide free legal help to tenants in Allegheny County"),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fs.ingested == nil {
		t.Fatal("IngestPlaybook was not called")
	}
	if fs.ingested.PageKind != "directory" {
		t.Errorf("ingested page_kind = %q, want directory", fs.ingested.PageKind)
	}
	if out.PageKind != "directory" {
		t.Errorf("output page_kind = %q, want directory — the agent needs to see what it saved", out.PageKind)
	}
	if fs.ingested.Status != "draft" {
		t.Errorf("status = %q, want draft: a directory is still reviewed before publication", fs.ingested.Status)
	}
}

func TestSaveDraft_DefaultsToPlaybookWhenPageKindOmitted(t *testing.T) {
	fs := &fakeStore{}
	tb := directoryToolbelt(t, fs)

	out, err := tb.SaveDraft(context.Background(), SaveDraftInput{
		CitySlug:  "boston",
		TopicSlug: "security-deposits",
		Title:     "Boston Security Deposits",
		IntroMD:   "What Boston renters should know.",
		Statements: []StatementInput{
			orgStmt("Example Legal Aid gives free legal help to renters.",
				"We provide free legal help to tenants in Allegheny County"),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fs.ingested.PageKind != "playbook" || out.PageKind != "playbook" {
		t.Errorf("ingested=%q output=%q, want playbook for an omitted page_kind",
			fs.ingested.PageKind, out.PageKind)
	}
}

// A mistyped page_kind must not quietly fall back to playbook. The wrong layout
// is invisible to the agent and to the draft queue, and only shows up in front
// of a reader.
func TestSaveDraft_RejectsUnknownPageKind(t *testing.T) {
	fs := &fakeStore{}
	tb := directoryToolbelt(t, fs)

	_, err := tb.SaveDraft(context.Background(), SaveDraftInput{
		CitySlug:  "boston",
		TopicSlug: "resource-directory",
		Title:     "Where to get help in Boston",
		IntroMD:   "Local organisations that help renters.",
		PageKind:  "Directory", // right word, wrong case
		Statements: []StatementInput{
			orgStmt("Example Legal Aid gives free legal help to renters.",
				"We provide free legal help to tenants in Allegheny County"),
		},
	})
	if !isRejection(err) {
		t.Fatalf("expected a rejection for an unknown page_kind, got err=%v", err)
	}
	if fs.ingested != nil {
		t.Error("a rejected page_kind must not write anything")
	}
}

// The verbatim-quote guardrail is not relaxed for directory pages. It is the
// reason a directory drafted here can be published at all: the publish path
// refuses any page carrying a citation with no quote.
func TestSaveDraft_DirectoryStillRequiresVerbatimQuotes(t *testing.T) {
	fs := &fakeStore{}
	tb := directoryToolbelt(t, fs)

	_, err := tb.SaveDraft(context.Background(), SaveDraftInput{
		CitySlug:  "boston",
		TopicSlug: "resource-directory",
		Title:     "Where to get help in Boston",
		IntroMD:   "Local organisations that help renters.",
		PageKind:  "directory",
		Statements: []StatementInput{
			orgStmt("Example Legal Aid runs a 24-hour hotline.",
				"We run a 24-hour hotline for renters"), // never appears in the source
		},
	})
	if !isRejection(err) {
		t.Fatalf("expected a rejection for a fabricated quote on a directory page, got err=%v", err)
	}
	if fs.ingested != nil {
		t.Error("a rejected directory draft must not write anything")
	}
}

// Topic and page_kind are independent axes and most combinations are fine. The
// one that never is: Local Help laid out as a playbook renders a list of
// organisations as a numbered legal argument, and nothing downstream catches
// it — the save succeeds and the draft queue shows an ordinary row.
func TestSaveDraft_RejectsLocalHelpLaidOutAsAPlaybook(t *testing.T) {
	for _, kind := range []string{"", "playbook", "faq", "checklist"} {
		fs := &fakeStore{}
		tb := directoryToolbelt(t, fs)
		_, err := tb.SaveDraft(context.Background(), SaveDraftInput{
			CitySlug:  "boston",
			TopicSlug: "resource-directory",
			Title:     "Where to get help in Boston",
			IntroMD:   "Local organisations that help renters.",
			PageKind:  kind,
			Statements: []StatementInput{
				orgStmt("Example Legal Aid gives free legal help to renters.",
					"We provide free legal help to tenants in Allegheny County"),
			},
		})
		if !isRejection(err) {
			t.Errorf("page_kind=%q on resource-directory: expected a rejection, got err=%v", kind, err)
		}
		if fs.ingested != nil {
			t.Errorf("page_kind=%q on resource-directory: nothing should have been written", kind)
		}
	}
}

// The reverse pairing stays legal. Migration 000005 put the directory layout on
// a cant-pay-rent page, and a list of places to go is a legitimate answer to
// plenty of subjects.
func TestSaveDraft_AllowsTheDirectoryLayoutOnOtherTopics(t *testing.T) {
	fs := &fakeStore{}
	tb := directoryToolbelt(t, fs)
	_, err := tb.SaveDraft(context.Background(), SaveDraftInput{
		CitySlug:  "boston",
		TopicSlug: "cant-pay-rent",
		Title:     "Rent help in Boston",
		IntroMD:   "Where to go if you cannot pay.",
		PageKind:  "directory",
		Statements: []StatementInput{
			orgStmt("Example Legal Aid gives free legal help to renters.",
				"We provide free legal help to tenants in Allegheny County"),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fs.ingested.PageKind != "directory" {
		t.Errorf("page_kind = %q, want directory", fs.ingested.PageKind)
	}
}
