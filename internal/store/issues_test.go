package store_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// ADR-013: saving captures, publishing guarantees. These tests cover the two
// halves — an incomplete draft always saves and is reported as not
// publishable, and the publish gate (on publish, and on saves that rewrite a
// live page) refuses exactly what the issue list names.

// issueCodes flattens the issue list for set assertions.
func issueCodes(issues []store.PageIssue) map[string]bool {
	out := map[string]bool{}
	for _, is := range issues {
		out[is.Code] = true
	}
	return out
}

func TestAuthorUpdate_savesAnIncompleteDraftAndFlagsIt(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	id := seedPlaybook(t, pg, jID, tID, "draft", "Complete Draft")

	statute, err := pg.UpsertSource(ctx, store.UpsertSourceParams{
		URL: "https://example.gov/statute-" + t.Name(), Publisher: "Example Legislature", Kind: "statute",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Everything the old save path refused, in one draft: no title, an
	// uncited statement, an empty statement, an unconfirmed quote, and a
	// statute citation whose locator names no provision.
	err = pg.AuthorUpdatePlaybook(ctx, store.AuthorUpdatePlaybookParams{
		ID: id, JurisdictionID: jID, TopicID: tID, Language: "en",
		Slug: "slug-" + t.Name(), Title: "", IntroMD: "intro", UpdatedBy: "test",
		Statements: []store.IngestStatementParams{
			{BodyMD: "A claim with no citation yet.", Language: "en"},
			{BodyMD: "", Language: "en"},
			{BodyMD: "A claim whose quote nobody confirmed.", Language: "en",
				Sources: []store.IngestCitationParams{{
					SourceID: statute.ID, Locator: "March 2026", Quote: "words never checked",
				}}},
		},
	})
	if err != nil {
		t.Fatalf("an incomplete draft must save (ADR-013), got: %v", err)
	}

	issues, err := pg.AuthorPlaybookIssues(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	got := issueCodes(issues)
	// The unconfirmed quote reports as source-unreachable, not
	// unverified-quote: the fixture's source has never been examined by the
	// checker (last_checked_at NULL), and the gate now groups unconfirmed
	// quotes by source and says when the source itself is the problem.
	for _, want := range []string{"no-title", "uncited-statement", "empty-statement", "source-unreachable", "statute-locator"} {
		if !got[want] {
			t.Errorf("issue %q not reported; got %v", want, issues)
		}
	}

	// The dashboard sweep must agree with the single-page check.
	all, err := pg.AuthorDraftIssues(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all[id]) != len(issues) {
		t.Errorf("dashboard sweep reports %d issues, single-page check %d", len(all[id]), len(issues))
	}

	// And publishing is where those issues block.
	err = pg.AuthorPublishPlaybook(ctx, id, "test")
	var npe *store.NotPublishableError
	if !errors.As(err, &npe) {
		t.Fatalf("publish must refuse a draft with issues, got: %v", err)
	}
	if len(npe.Issues) != len(issues) {
		t.Errorf("publish refused on %d issues, the page reports %d", len(npe.Issues), len(issues))
	}
	var status string
	if err := pg.Pool().QueryRow(ctx, `SELECT status FROM playbooks WHERE id = $1`, id).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "draft" {
		t.Fatalf("refused publish must leave the page a draft, got %q", status)
	}
}

func TestAuthorUpdate_aLivePageSaveRunsThePublishGate(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	id := seedPlaybook(t, pg, jID, tID, "published", "Live Page")

	err := pg.AuthorUpdatePlaybook(ctx, store.AuthorUpdatePlaybookParams{
		ID: id, JurisdictionID: jID, TopicID: tID, Language: "en",
		Slug: "slug-" + t.Name(), Title: "Live Page", IntroMD: "intro", UpdatedBy: "test",
		Statements: []store.IngestStatementParams{
			{BodyMD: "An uncited claim renters would read immediately.", Language: "en"},
		},
	})
	var npe *store.NotPublishableError
	if !errors.As(err, &npe) {
		t.Fatalf("saving a live page into an unpublishable state must be refused, got: %v", err)
	}

	// The refusal must roll the whole save back: the page still reads as before.
	var n int
	if err := pg.Pool().QueryRow(ctx, `
		SELECT count(*) FROM playbook_statements ps
		JOIN statements s ON s.id = ps.statement_id
		WHERE ps.playbook_id = $1 AND s.body_md LIKE 'A claim.%'`, id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("refused live-page save must leave the stored statements untouched, found %d original statements", n)
	}
}

func TestAuthorUpdate_cleansUpDetachedStatements(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	id := seedPlaybook(t, pg, jID, tID, "draft", "Tidy")

	// Each save detaches the previous statement rows; autosave makes this a
	// once-a-minute event, so they must not accumulate.
	for range 3 {
		if err := pg.AuthorUpdatePlaybook(ctx, store.AuthorUpdatePlaybookParams{
			ID: id, JurisdictionID: jID, TopicID: tID, Language: "en",
			Slug: "slug-" + t.Name(), Title: "Tidy", UpdatedBy: "test",
			Statements: []store.IngestStatementParams{
				{BodyMD: "The only statement. " + t.Name(), Language: "en"},
			},
		}); err != nil {
			t.Fatal(err)
		}
	}
	var orphans int
	if err := pg.Pool().QueryRow(ctx, `
		SELECT count(*) FROM statements s
		WHERE NOT EXISTS (SELECT 1 FROM playbook_statements ps WHERE ps.statement_id = s.id)`).Scan(&orphans); err != nil {
		t.Fatal(err)
	}
	if orphans != 0 {
		t.Errorf("%d detached statements left behind after re-saves", orphans)
	}
}

func TestPublish_refusesAQuoteNobodyConfirmed(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	id := seedPlaybook(t, pg, jID, tID, "draft", "Unconfirmed")

	src, err := pg.UpsertSource(ctx, store.UpsertSourceParams{
		URL: "https://example.gov/unconfirmed-" + t.Name(), Publisher: "Example", Kind: "gov_guidance",
	})
	if err != nil {
		t.Fatal(err)
	}
	// A quote saved without CheckedNow and with no earlier stamp to inherit is
	// stored unverified — exactly what the old save path refused outright.
	if err := pg.AuthorUpdatePlaybook(ctx, store.AuthorUpdatePlaybookParams{
		ID: id, JurisdictionID: jID, TopicID: tID, Language: "en",
		Slug: "slug-" + t.Name(), Title: "Unconfirmed", UpdatedBy: "test",
		Statements: []store.IngestStatementParams{
			{BodyMD: "A claim.", Language: "en",
				Sources: []store.IngestCitationParams{{SourceID: src.ID, Quote: "never confirmed anywhere"}}},
		},
	}); err != nil {
		t.Fatal(err)
	}

	err = pg.AuthorPublishPlaybook(ctx, id, "test")
	if err == nil {
		t.Fatal("published a page whose quote was never confirmed at its source")
	}
	if !strings.Contains(err.Error(), "unconfirmed quotes") {
		t.Fatalf("error should say the quotes are unconfirmed, got: %v", err)
	}

	// A reviewer's attestation is a confirmation, so it unblocks.
	if err := pg.AuthorUpdatePlaybook(ctx, store.AuthorUpdatePlaybookParams{
		ID: id, JurisdictionID: jID, TopicID: tID, Language: "en",
		Slug: "slug-" + t.Name(), Title: "Unconfirmed", UpdatedBy: "test",
		Statements: []store.IngestStatementParams{
			{BodyMD: "A claim.", Language: "en",
				Sources: []store.IngestCitationParams{{
					SourceID: src.ID, Quote: "never confirmed anywhere",
					ManuallyVerified: true, CheckedNow: true, CheckedBy: "test",
				}}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := pg.AuthorPublishPlaybook(ctx, id, "test"); err != nil {
		t.Fatalf("an attested quote must publish: %v", err)
	}
}

func TestIngest_allowIncompleteIsForDraftsOnly(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()

	if err := pg.IngestPlaybook(ctx, store.IngestPlaybookParams{
		JurisdictionID: jID, TopicID: tID, Language: "en", Slug: "slug-" + t.Name(),
		Title: "Incomplete", Status: "published", AllowIncomplete: true,
		Statements: []store.IngestStatementParams{{BodyMD: "Uncited.", Language: "en"}},
	}); err == nil {
		t.Fatal("AllowIncomplete must not apply to a published ingest")
	}

	if err := pg.IngestPlaybook(ctx, store.IngestPlaybookParams{
		JurisdictionID: jID, TopicID: tID, Language: "en", Slug: "slug-" + t.Name(),
		Title: "Incomplete", Status: "draft", AllowIncomplete: true,
		Statements: []store.IngestStatementParams{{BodyMD: "Uncited.", Language: "en"}},
	}); err != nil {
		t.Fatalf("an incomplete human draft must save: %v", err)
	}

	id, err := pg.AuthorFindDraft(ctx, jID, tID, "en")
	if err != nil {
		t.Fatalf("the saved draft must be findable by slot: %v", err)
	}
	issues, err := pg.AuthorPlaybookIssues(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if !issueCodes(issues)["uncited-statement"] {
		t.Errorf("uncited statement not flagged, got %v", issues)
	}
}

func TestCitationQuoteExists_requiresAConfirmation(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	id := seedPlaybook(t, pg, jID, tID, "draft", "Known Quotes")

	url := "https://example.gov/known-" + t.Name()
	src, err := pg.UpsertSource(ctx, store.UpsertSourceParams{URL: url, Publisher: "Example", Kind: "gov_guidance"})
	if err != nil {
		t.Fatal(err)
	}
	save := func(checkedNow bool) {
		t.Helper()
		if err := pg.AuthorUpdatePlaybook(ctx, store.AuthorUpdatePlaybookParams{
			ID: id, JurisdictionID: jID, TopicID: tID, Language: "en",
			Slug: "slug-" + t.Name(), Title: "Known Quotes", UpdatedBy: "test",
			Statements: []store.IngestStatementParams{
				{BodyMD: "A claim.", Language: "en",
					Sources: []store.IngestCitationParams{{
						SourceID: src.ID, Quote: "the exact words", CheckedNow: checkedNow, CheckedBy: "test",
					}}},
			},
		}); err != nil {
			t.Fatal(err)
		}
	}

	// Stored but never confirmed: the known-quote skip must not launder it.
	save(false)
	known, err := pg.CitationQuoteExists(ctx, url, "the exact words")
	if err != nil {
		t.Fatal(err)
	}
	if known {
		t.Fatal("an unconfirmed stored quote must not count as known-verified")
	}

	// Confirmed once: from then on the pair is known and skips the re-fetch.
	save(true)
	known, err = pg.CitationQuoteExists(ctx, url, "the exact words")
	if err != nil {
		t.Fatal(err)
	}
	if !known {
		t.Fatal("a confirmed quote must count as known-verified")
	}
}
