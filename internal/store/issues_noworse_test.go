package store_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// A live page published before the gate existed can carry an old problem.
// A save may fix or add around it, but may not add a problem of its own.
func TestAuthorUpdate_aLivePageSaveMayNotAddAProblem(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	src, err := pg.UpsertSource(ctx, store.UpsertSourceParams{URL: "https://example.gov/old-" + t.Name(), Publisher: "Example", Kind: "gov_guidance"})
	if err != nil {
		t.Fatal(err)
	}
	good := store.IngestStatementParams{BodyMD: "A checked claim.", Language: "en",
		Sources: []store.IngestCitationParams{{SourceID: src.ID, Locator: "Part 1", Quote: "verbatim", CheckedNow: true, CheckedBy: "test"}}}
	legacy := store.IngestStatementParams{BodyMD: "An old claim with no quote.", Language: "en",
		Sources: []store.IngestCitationParams{{SourceID: src.ID, Locator: "Part 2"}}}
	slug := "slug-" + t.Name()
	if err := pg.IngestPlaybook(ctx, store.IngestPlaybookParams{
		JurisdictionID: jID, TopicID: tID, Language: "en", Slug: slug,
		Title: "Legacy", IntroMD: "intro", Status: "published", UpdatedBy: "test",
		Statements: []store.IngestStatementParams{legacy},
	}); err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := pg.Pool().QueryRow(ctx, `SELECT id FROM playbooks WHERE jurisdiction_id=$1 AND topic_id=$2 AND status='published'`, jID, tID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if is, _ := pg.AuthorPlaybookIssues(ctx, id); len(is) == 0 {
		t.Fatal("fixture: the legacy page should carry an old problem")
	}
	save := func(stmts ...store.IngestStatementParams) error {
		return pg.AuthorUpdatePlaybook(ctx, store.AuthorUpdatePlaybookParams{
			ID: id, JurisdictionID: jID, TopicID: tID, Language: "en", Slug: slug,
			Title: "Legacy", IntroMD: "intro", UpdatedBy: "test", Statements: stmts,
		})
	}

	// A good statement above the old one moves it to statement 2; that is
	// not a new problem.
	if err := save(good, legacy); err != nil {
		t.Fatalf("a save that adds no problem was refused: %v", err)
	}

	// A second statement with the same kind of problem is new.
	another := legacy
	another.BodyMD = "Another claim with no quote."
	err = save(good, legacy, another)
	var npe *store.NotPublishableError
	if !errors.As(err, &npe) {
		t.Fatalf("a save that adds a problem was allowed: %v", err)
	}
	if len(npe.Issues) == 0 || !strings.Contains(npe.Issues[0].Detail, "statement 3") {
		t.Errorf("the refusal should name only the new problem, got %+v", npe.Issues)
	}
}
