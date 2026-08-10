package store_test

import (
	"context"
	"strings"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// The verbatim quote is what makes a citation checkable. The drafting tools
// enforce it on the way in, but the authoring form can only preserve a quote it
// was given, never create one, so a citation added by hand arrives empty. These
// tests cover the boundary that decides whether such a page reaches renters.

// seedUnquoted writes a draft whose single citation carries no verbatim quote,
// the way a hand-added citation arrives from the authoring form.
func seedUnquoted(t *testing.T, pg *store.PG, jID, tID int64, kind string) int64 {
	t.Helper()
	ctx := context.Background()
	src, err := pg.UpsertSource(ctx, store.UpsertSourceParams{
		URL: "https://example.gov/unquoted-" + t.Name(), Publisher: "Example", Kind: kind,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := pg.IngestPlaybook(ctx, store.IngestPlaybookParams{
		JurisdictionID: jID, TopicID: tID, Language: "en", Slug: "slug-" + t.Name(),
		Title: "Unquoted", IntroMD: "intro", Status: "draft",
		Statements: []store.IngestStatementParams{{
			BodyMD: "A claim nobody can verify.", Language: "en",
			Sources: []store.IngestCitationParams{{SourceID: src.ID, Locator: "§ 1"}},
		}},
	}); err != nil {
		t.Fatalf("ingest draft: %v", err)
	}
	var id int64
	if err := pg.Pool().QueryRow(ctx,
		`SELECT id FROM playbooks WHERE jurisdiction_id=$1 AND topic_id=$2 AND language='en' AND status='draft'`,
		jID, tID).Scan(&id); err != nil {
		t.Fatalf("find draft: %v", err)
	}
	return id
}

func TestPublish_refusesACitationWithNoVerbatimQuote(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	id := seedUnquoted(t, pg, jID, tID, "statute")

	err := pg.AuthorPublishPlaybook(context.Background(), id)
	if err == nil {
		t.Fatal("published a page whose citation carries no verbatim quote; the source checker could never verify it")
	}
	if !strings.Contains(err.Error(), "no verbatim quote") {
		t.Fatalf("error should say what is wrong, got: %v", err)
	}

	var status string
	if err := pg.Pool().QueryRow(context.Background(),
		`SELECT status FROM playbooks WHERE id = $1`, id).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "draft" {
		t.Fatalf("rejected publish must leave the page a draft, got %q", status)
	}
}

func TestPublish_allowsAnEditorialCitationWithNoQuote(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	id := seedUnquoted(t, pg, jID, tID, "editorial")

	// Editorial sources cite no external text by design (ADR-003), so there is
	// nothing to quote and nothing for the checker to verify. They must not be
	// caught by a rule aimed at unverifiable statutory claims.
	if err := pg.AuthorPublishPlaybook(context.Background(), id); err != nil {
		t.Fatalf("editorial citation should not block publishing: %v", err)
	}
}

func TestPublish_allowsAQuotedCitation(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	id := seedPlaybook(t, pg, jID, tID, "draft", "Quoted")

	if err := pg.AuthorPublishPlaybook(context.Background(), id); err != nil {
		t.Fatalf("a properly quoted page must still publish: %v", err)
	}
}
