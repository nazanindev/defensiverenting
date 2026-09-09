package sourcecheck_test

import (
	"context"
	"os"
	"strings"
	"testing"

	dbpkg "github.com/nazanindev/defensiverenting/db"
	"github.com/nazanindev/defensiverenting/internal/sourcecheck"
	"github.com/nazanindev/defensiverenting/internal/store"
)

// End to end against a real database: a page cites a quote, the source is
// re-fetched with the quote reworded, and the review queue ends up holding a
// source-drift proposal with the reworded passage as the replacement quote.
func TestRun_filesIntoTheRealQueue(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}
	ctx := context.Background()
	pg, err := store.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pg.Close)
	if err := dbpkg.Migrate(ctx, pg.Pool()); err != nil {
		t.Fatal(err)
	}
	slug := "drift-" + strings.ToLower(t.Name())
	_, _ = pg.Pool().Exec(ctx, `DELETE FROM playbooks WHERE slug = $1`, slug)
	_, _ = pg.Pool().Exec(ctx, `DELETE FROM sources WHERE url = $1`, "https://example.gov/"+slug)
	j, err := pg.UpsertJurisdiction(ctx, store.UpsertJurisdictionParams{Kind: "city", Name: "Drift City", Slug: slug})
	if err != nil {
		t.Fatal(err)
	}
	tp, err := pg.UpsertTopic(ctx, store.UpsertTopicParams{Slug: slug, Name: "Drift"})
	if err != nil {
		t.Fatal(err)
	}
	src, err := pg.UpsertSource(ctx, store.UpsertSourceParams{URL: "https://example.gov/" + slug, Publisher: "Example", Kind: "statute"})
	if err != nil {
		t.Fatal(err)
	}
	const old = "the landlord shall return the deposit within thirty days"
	if err := pg.IngestPlaybook(ctx, store.IngestPlaybookParams{
		JurisdictionID: j.ID, TopicID: tp.ID, Language: "en", Slug: slug, Title: "Drift", IntroMD: "i", Status: "published",
		Statements: []store.IngestStatementParams{{BodyMD: "Deposits come back in 30 days.", Language: "en",
			Sources: []store.IngestCitationParams{{SourceID: src.ID, Locator: "§ 9", Quote: old, CheckedNow: true, CheckedBy: "test"}}}},
	}); err != nil {
		t.Fatal(err)
	}

	fetch := func(u string) (string, error) {
		if u == src.URL {
			return "(a) The landlord shall return the deposit within twenty-one days. (b) Interest.", nil
		}
		return "unrelated page that still contains whatever it contained", nil
	}
	res, err := sourcecheck.Run(ctx, pg, fetch, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Drifted < 1 || res.Proposed < 1 {
		t.Fatalf("result = %+v, want the reworded source counted as drifted and proposed", res)
	}
	pending, err := pg.ListProposals(ctx, "pending")
	if err != nil {
		t.Fatal(err)
	}
	var found *store.ProposalRow
	for i := range pending {
		if pending[i].Title == "Drift" && pending[i].Reason == "source-drift" {
			found = &pending[i]
		}
	}
	if found == nil {
		t.Fatal("no source-drift proposal for the page in the pending queue")
	}
	if found.Proposed == nil || !strings.Contains(found.Proposed.Citations[0].Quote, "twenty-one days") || !found.Proposed.Citations[0].Checked {
		t.Errorf("proposal = %+v, want the reworded passage as a checked replacement quote", found.Proposed)
	}
	if found.CurrentBody != "Deposits come back in 30 days." || found.Position != 1 {
		t.Errorf("row shows current %q at %d", found.CurrentBody, found.Position)
	}

	// Running again must not stack a second proposal on the same drift.
	res2, err := sourcecheck.Run(ctx, pg, fetch, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res2.Proposed != 0 {
		t.Errorf("second run filed %d proposal(s); the first is still waiting", res2.Proposed)
	}
}
