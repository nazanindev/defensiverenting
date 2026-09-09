package store_test

import (
	"context"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// A statement's key is its identity across saves (ADR-014 D1). Every save
// rewrites the statements rows, so the key is the only thing a proposal, a
// history entry, or a translation link can hold onto. These tests pin down
// when a save keeps a key and when it mints one.

func statementKeys(t *testing.T, pg *store.PG, id int64) []string {
	t.Helper()
	pw, err := pg.AuthorGetPlaybook(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(pw.Statements))
	for _, st := range pw.Statements {
		if st.Key == "" {
			t.Fatalf("statement %q read back with no key", st.BodyMD)
		}
		out = append(out, st.Key)
	}
	return out
}

// resave writes the page back through the portal's path with the given
// statements, each citing the page's existing source with a confirmed quote.
func resave(t *testing.T, pg *store.PG, id, jID, tID int64, stmts ...store.IngestStatementParams) {
	t.Helper()
	ctx := context.Background()
	pw, err := pg.AuthorGetPlaybook(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	srcID := pw.Statements[0].Citations[0].SourceID
	for i := range stmts {
		stmts[i].Language = "en"
		stmts[i].Sources = []store.IngestCitationParams{{SourceID: srcID, Locator: "§ 1", Quote: "verbatim", CheckedNow: true, CheckedBy: "test"}}
	}
	if err := pg.AuthorUpdatePlaybook(ctx, store.AuthorUpdatePlaybookParams{
		ID: id, JurisdictionID: jID, TopicID: tID, Language: "en",
		Slug: pw.Slug, Title: pw.Title, IntroMD: pw.IntroMD, UpdatedBy: "test",
		Statements: stmts,
	}); err != nil {
		t.Fatalf("resave: %v", err)
	}
}

func TestStatementKey_unchangedBodyKeepsKeyWithoutBeingTold(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	id := seedPlaybook(t, pg, jID, tID, "draft", "Keys")
	before := statementKeys(t, pg, id)

	// Autosave posts the body back untouched; the form carries the key, but
	// even a caller that does not must not turn every save into a new claim.
	resave(t, pg, id, jID, tID, store.IngestStatementParams{BodyMD: "A claim. Keys"})

	after := statementKeys(t, pg, id)
	if after[0] != before[0] {
		t.Errorf("unchanged body got key %s, want the original %s", after[0], before[0])
	}
}

func TestStatementKey_editedBodyKeepsExplicitKey(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	id := seedPlaybook(t, pg, jID, tID, "draft", "Keys")
	before := statementKeys(t, pg, id)

	resave(t, pg, id, jID, tID, store.IngestStatementParams{BodyMD: "A reworded claim.", Key: before[0]})

	after := statementKeys(t, pg, id)
	if after[0] != before[0] {
		t.Errorf("edited body with explicit key got %s, want %s", after[0], before[0])
	}
}

func TestStatementKey_editedBodyWithoutKeyIsANewClaim(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	id := seedPlaybook(t, pg, jID, tID, "draft", "Keys")
	before := statementKeys(t, pg, id)

	resave(t, pg, id, jID, tID, store.IngestStatementParams{BodyMD: "A reworded claim."})

	after := statementKeys(t, pg, id)
	if after[0] == before[0] {
		t.Errorf("edited body with no key kept %s; a claim nobody linked to the old one must get a fresh key", after[0])
	}
}

func TestStatementKey_revisionInheritsFromLivePage(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	live := seedPlaybook(t, pg, jID, tID, "published", "Keys")
	liveKeys := statementKeys(t, pg, live)

	// The drafting agent re-researches the page and saves a revision beside
	// it, passing no keys. The statement it kept word for word is the same
	// claim, and the queue must be able to say so.
	draft := seedPlaybook(t, pg, jID, tID, "draft", "Keys")
	draftKeys := statementKeys(t, pg, draft)

	if draftKeys[0] != liveKeys[0] {
		t.Errorf("revision statement got key %s, want the live page's %s", draftKeys[0], liveKeys[0])
	}
	if got := statementKeys(t, pg, live); got[0] != liveKeys[0] {
		t.Errorf("saving the revision changed the live page's key to %s", got[0])
	}
}

func TestStatementKey_twoIdenticalBodiesGetDistinctKeys(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	id := seedPlaybook(t, pg, jID, tID, "draft", "Keys")
	before := statementKeys(t, pg, id)

	resave(t, pg, id, jID, tID,
		store.IngestStatementParams{BodyMD: "A claim. Keys"},
		store.IngestStatementParams{BodyMD: "A claim. Keys"},
	)

	after := statementKeys(t, pg, id)
	if len(after) != 2 {
		t.Fatalf("got %d statements, want 2", len(after))
	}
	if after[0] != before[0] {
		t.Errorf("first copy got %s, want the original %s", after[0], before[0])
	}
	if after[1] == after[0] {
		t.Errorf("both copies share key %s; a key names one statement per page", after[0])
	}
}

func TestStatementKey_malformedKeyIsRefused(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	id := seedPlaybook(t, pg, jID, tID, "draft", "Keys")
	ctx := context.Background()
	pw, err := pg.AuthorGetPlaybook(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	err = pg.AuthorUpdatePlaybook(ctx, store.AuthorUpdatePlaybookParams{
		ID: id, JurisdictionID: jID, TopicID: tID, Language: "en",
		Slug: pw.Slug, Title: pw.Title, UpdatedBy: "test",
		Statements: []store.IngestStatementParams{{BodyMD: "x", Language: "en", Key: "not-a-key"}},
	})
	if err == nil {
		t.Fatal("save with a malformed key succeeded, want a refusal naming the key")
	}
	if got := statementKeys(t, pg, id); got[0] != pw.Statements[0].Key {
		t.Errorf("refused save still changed the page")
	}
}
