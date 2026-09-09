package store_test

import (
	"context"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// Content in a deferred language leaves the working set (ADR-015): it can
// be edited but not published, its citations are not re-checked, and its
// proposals are not queued. These tests flip the registry to prove each
// gate keys off it and nothing else.

func withLanguages(t *testing.T, langs ...string) {
	t.Helper()
	prev := store.ContentLanguages
	store.ContentLanguages = langs
	t.Cleanup(func() { store.ContentLanguages = prev })
}

func TestDeferredLanguage_cannotPublishButCanSave(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	withLanguages(t, "en", "es")
	// Seeded while Spanish was active, as the pilot drafts were.
	src, err := pg.UpsertSource(ctx, store.UpsertSourceParams{URL: "https://example.gov/es-" + t.Name(), Publisher: "Example", Kind: "statute"})
	if err != nil {
		t.Fatal(err)
	}
	if err := pg.IngestPlaybook(ctx, store.IngestPlaybookParams{
		JurisdictionID: jID, TopicID: tID, Language: "es", Slug: "es-" + t.Name(), Title: "Página", IntroMD: "intro", Status: "draft",
		Statements: []store.IngestStatementParams{{BodyMD: "Una afirmación.", Language: "es",
			Sources: []store.IngestCitationParams{{SourceID: src.ID, Locator: "§ 1", Quote: "verbatim", CheckedNow: true, CheckedBy: "test"}}}},
	}); err != nil {
		t.Fatal(err)
	}
	id, err := pg.AuthorFindDraft(ctx, jID, tID, "es")
	if err != nil {
		t.Fatal(err)
	}
	if codes := issueCodes(pageIssues(t, pg, id)); codes["language-deferred"] {
		t.Fatal("Spanish active, yet the page reports language-deferred")
	}

	withLanguages(t, "en")
	if codes := issueCodes(pageIssues(t, pg, id)); !codes["language-deferred"] {
		t.Errorf("Spanish deferred, but the page's issues are %v, want language-deferred", codes)
	}
	if err := pg.AuthorPublishPlaybook(ctx, id, "test"); err == nil {
		t.Error("published a page in a deferred language")
	}
	// Editing still saves: refusing to save what exists loses work for nothing.
	pw, _ := pg.AuthorGetPlaybook(ctx, id)
	if err := pg.AuthorUpdatePlaybook(ctx, store.AuthorUpdatePlaybookParams{
		ID: id, JurisdictionID: jID, TopicID: tID, Language: "es", Slug: pw.Slug, Title: "Página editada", UpdatedBy: "test",
		Statements: []store.IngestStatementParams{{BodyMD: "Una afirmación editada.", Language: "es", Key: pw.Statements[0].Key,
			Sources: []store.IngestCitationParams{{SourceID: src.ID, Locator: "§ 1", Quote: "verbatim"}}}},
	}); err != nil {
		t.Errorf("editing a parked draft failed: %v", err)
	}
}

func TestDeferredLanguage_leavesCheckerAndQueue(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	withLanguages(t, "en", "es")
	en := seedPlaybook(t, pg, jID, tID, "published", "Keys")
	src := sourceOf(t, pg, en)
	if err := pg.IngestPlaybook(ctx, store.IngestPlaybookParams{
		JurisdictionID: jID, TopicID: tID, Language: "es", Slug: "es-" + t.Name(), Title: "Página", IntroMD: "intro", Status: "draft",
		Statements: []store.IngestStatementParams{{BodyMD: "Una afirmación.", Language: "es",
			Sources: []store.IngestCitationParams{{SourceID: src.SourceID, Locator: "§ 1", Quote: "sólo en español", CheckedNow: true, CheckedBy: "test"}}}},
	}); err != nil {
		t.Fatal(err)
	}
	esID, _ := pg.AuthorFindDraft(ctx, jID, tID, "es")
	esKey := statementKeys(t, pg, esID)[0]
	esProposal := file(t, pg, esKey, esID, "Otra redacción.")
	enProposal := file(t, pg, statementKeys(t, pg, en)[0], en, "Another wording.")

	count := func(quote string) int {
		rows, err := pg.ListCitationsForCheck(ctx)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, r := range rows {
			if r.Quote == quote {
				n++
			}
		}
		return n
	}
	if count("sólo en español") == 0 || !inList(t, pg, "pending", esProposal) {
		t.Fatal("Spanish active, yet its citation or proposal is missing from the working set")
	}

	withLanguages(t, "en")
	if n := count("sólo en español"); n != 0 {
		t.Errorf("checker still sees %d Spanish-only citation(s) with Spanish deferred", n)
	}
	if count("verbatim") == 0 {
		t.Error("the English page's citation dropped out of the checker")
	}
	if inList(t, pg, "pending", esProposal) {
		t.Error("a proposal against a parked Spanish statement is still queued")
	}
	if !inList(t, pg, "pending", enProposal) {
		t.Error("the English proposal dropped out of the queue")
	}
	n, err := pg.CountPendingProposals(ctx)
	if err != nil {
		t.Fatal(err)
	}
	pending, _ := pg.ListProposals(ctx, "pending")
	if n != len(pending) {
		t.Errorf("dashboard count %d disagrees with the queue's %d rows", n, len(pending))
	}
}

func pageIssues(t *testing.T, pg *store.PG, id int64) []store.PageIssue {
	t.Helper()
	issues, err := pg.AuthorPlaybookIssues(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return issues
}
