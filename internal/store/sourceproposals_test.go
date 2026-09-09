package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// An unused source becomes a proposal to delete it (ADR-014 D7). These tests
// pin down filing, the refusal when a page cites it again, and the deletion.

// resetSourceProposals forgets earlier runs' decisions about this test's
// sources. The shared test database keeps rows between runs, and a rejected
// proposal is by design never refiled, so without this a second run finds
// nothing filed. Sources from other tests are ignored throughout: each
// test's cleanup orphans its own source, which a later filing picks up.
func resetSourceProposals(t *testing.T, pg *store.PG) {
	t.Helper()
	if _, err := pg.Pool().Exec(context.Background(),
		`DELETE FROM source_proposals WHERE url LIKE $1`, "%"+t.Name()+"%"); err != nil {
		t.Fatal(err)
	}
}

func unusedSource(t *testing.T, pg *store.PG, suffix string) store.Source {
	t.Helper()
	resetSourceProposals(t, pg)
	src, err := pg.UpsertSource(context.Background(), store.UpsertSourceParams{
		URL: "https://example.gov/unused-" + t.Name() + suffix, Publisher: "Unused Pub", Kind: "statute",
	})
	if err != nil {
		t.Fatal(err)
	}
	return src
}

func openSourceProposalFor(t *testing.T, pg *store.PG, sourceID int64) *store.SourceProposal {
	t.Helper()
	rows, err := pg.ListSourceProposals(context.Background(), "pending")
	if err != nil {
		t.Fatal(err)
	}
	for i := range rows {
		if rows[i].SourceID != nil && *rows[i].SourceID == sourceID {
			return &rows[i]
		}
	}
	return nil
}

func TestSourceProposal_filesOnceAndSkipsCitedAndRejected(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	page := seedPlaybook(t, pg, jID, tID, "published", "Cites its source")
	cited := sourceOf(t, pg, page)
	unused := unusedSource(t, pg, "")

	if _, err := pg.FileUnusedSourceProposals(ctx, store.ActorSourceCheck); err != nil {
		t.Fatal(err)
	}
	if openSourceProposalFor(t, pg, cited.SourceID) != nil {
		t.Error("a cited source was filed for deletion")
	}
	p := openSourceProposalFor(t, pg, unused.ID)
	if p == nil {
		t.Fatal("the unused source was not filed")
	}
	if p.Reason != "unused-source" || p.ProposedBy != store.ActorSourceCheck || p.URL != unused.URL {
		t.Errorf("proposal = %+v", *p)
	}

	// A second run refiles nothing while the first is waiting.
	if _, err := pg.FileUnusedSourceProposals(ctx, store.ActorSourceCheck); err != nil {
		t.Fatal(err)
	}
	if got := countOpenSourceProposals(t, pg, unused.ID); got != 1 {
		t.Errorf("open proposals for the source after a second run = %d, want 1", got)
	}

	// Rejecting it means "keep this row": later runs leave it alone.
	if err := pg.DecideSourceProposal(ctx, p.ID, "rejected", "nazanin", "kept for a page in progress", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := pg.FileUnusedSourceProposals(ctx, store.ActorSourceCheck); err != nil {
		t.Fatal(err)
	}
	if openSourceProposalFor(t, pg, unused.ID) != nil {
		t.Error("a rejected source was filed again")
	}
	if err := pg.DecideSourceProposal(ctx, p.ID, "rejected", "nazanin", "", nil); !errors.Is(err, store.ErrProposalNotPending) {
		t.Errorf("deciding a decided proposal: %v, want ErrProposalNotPending", err)
	}
}

func countOpenSourceProposals(t *testing.T, pg *store.PG, sourceID int64) int {
	t.Helper()
	var n int
	if err := pg.Pool().QueryRow(context.Background(),
		`SELECT count(*) FROM source_proposals WHERE source_id = $1 AND status IN ('pending','snoozed')`, sourceID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestSourceProposal_approveDeletesTheSourceAndKeepsTheRecord(t *testing.T) {
	pg, _, _ := revisionFixture(t)
	ctx := context.Background()
	unused := unusedSource(t, pg, "")
	if _, err := pg.FileUnusedSourceProposals(ctx, store.ActorSourceCheck); err != nil {
		t.Fatal(err)
	}
	p := openSourceProposalFor(t, pg, unused.ID)
	if p == nil {
		t.Fatal("not filed")
	}
	if err := pg.ApproveSourceProposal(ctx, p.ID, "nazanin"); err != nil {
		t.Fatalf("approve: %v", err)
	}
	var exists bool
	if err := pg.Pool().QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM sources WHERE id = $1)`, unused.ID).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("the source row survived its approved deletion")
	}
	got, err := pg.GetSourceProposal(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "approved" || got.DecidedBy != "nazanin" || got.SourceID != nil || got.URL != unused.URL {
		t.Errorf("record after approval = %+v", got)
	}
	if err := pg.ApproveSourceProposal(ctx, p.ID, "nazanin"); !errors.Is(err, store.ErrProposalNotPending) {
		t.Errorf("approving twice: %v, want ErrProposalNotPending", err)
	}
}

func TestSourceProposal_approveRefusesASourceCitedAgain(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	unused := unusedSource(t, pg, "")
	if _, err := pg.FileUnusedSourceProposals(ctx, store.ActorSourceCheck); err != nil {
		t.Fatal(err)
	}
	p := openSourceProposalFor(t, pg, unused.ID)
	if p == nil {
		t.Fatal("not filed")
	}
	// A page comes to cite the source while the proposal waits.
	if err := pg.IngestPlaybook(ctx, store.IngestPlaybookParams{
		JurisdictionID: jID, TopicID: tID, Language: "en", Slug: "slug-" + t.Name(),
		Title: "Cites it now", IntroMD: "intro", Status: "draft",
		Statements: []store.IngestStatementParams{{
			BodyMD: "A claim.", Language: "en",
			Sources: []store.IngestCitationParams{{SourceID: unused.ID, Locator: "§ 1", Quote: "verbatim", CheckedNow: true, CheckedBy: "test"}},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := pg.ApproveSourceProposal(ctx, p.ID, "nazanin"); !errors.Is(err, store.ErrSourceInUse) {
		t.Fatalf("approve: %v, want ErrSourceInUse", err)
	}
	listed := openSourceProposalFor(t, pg, unused.ID)
	if listed == nil || !listed.InUse {
		t.Error("the queue does not say the source is in use again")
	}
	until := time.Now().Add(24 * time.Hour)
	if err := pg.DecideSourceProposal(ctx, p.ID, "snoozed", "nazanin", "", &until); err != nil {
		t.Fatal(err)
	}
	if openSourceProposalFor(t, pg, unused.ID) != nil {
		t.Error("a snoozed proposal still lists as pending")
	}
}
