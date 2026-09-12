package store_test

import (
	"context"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// Standing is the per-statement answer the cards and the worklist show; the
// gate is the per-page answer publishing enforces. They must agree: a page
// publishes exactly when every statement on it is Ready.

func standings(t *testing.T, pg *store.PG, id int64) []store.Standing {
	t.Helper()
	pw, err := pg.AuthorGetPlaybook(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]store.Standing, len(pw.Statements))
	for i, st := range pw.Statements {
		out[i] = st.Standing(pw.Playbook.PageKind == "directory")
	}
	return out
}

func gateAgrees(t *testing.T, pg *store.PG, id int64) {
	t.Helper()
	issues, err := pg.AuthorPlaybookIssues(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	allReady := true
	for _, s := range standings(t, pg, id) {
		if s.Status != store.Ready {
			allReady = false
		}
	}
	if allReady != (len(issues) == 0) {
		t.Fatalf("gate and standing disagree: issues=%v standings=%+v", issues, standings(t, pg, id))
	}
}

func TestStanding_followsTheStatementThroughReview(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	id := seedPlaybook(t, pg, jID, tID, "draft", "Standing")
	pw, _ := pg.AuthorGetPlaybook(ctx, id)
	srcID := pw.Statements[0].Citations[0].SourceID
	keys := statementKeys(t, pg, id)
	// The source row outlives test runs; start from "readable".
	if err := pg.MarkSourceChecked(ctx, srcID, ""); err != nil {
		t.Fatal(err)
	}

	saveAs(t, pg, id, jID, tID, store.ActorDraftingAgent,
		store.IngestStatementParams{Key: keys[0], BodyMD: "Claim one.", ReviewerNote: "A doubt."},
		store.IngestStatementParams{BodyMD: "Claim two.",
			Sources: []store.IngestCitationParams{{SourceID: srcID, Locator: "§ 2", Quote: "never confirmed"}}})
	got := standings(t, pg, id)
	if got[0].Status != store.NeedsYou || !got[0].Has(store.ReasonNote) || !got[0].Has(store.ReasonUnread) {
		t.Errorf("statement 1 = %+v", got[0])
	}
	if got[1].Status != store.NeedsYou || !got[1].Has(store.ReasonQuoteUnconfirmed) || got[1].Unconfirmed != 1 {
		t.Errorf("statement 2 = %+v", got[1])
	}
	gateAgrees(t, pg, id)

	// The checker reports it cannot read the source: the unconfirmed quote
	// is now blocked, not merely unconfirmed.
	if err := pg.MarkSourceUnreadable(ctx, srcID, "bot check"); err != nil {
		t.Fatal(err)
	}
	got = standings(t, pg, id)
	if got[1].Status != store.Blocked || !got[1].Has(store.ReasonSourceBlocked) {
		t.Errorf("after an unreadable fetch, statement 2 = %+v", got[1])
	}

	// A person attests the quote, decides the note, and reads both.
	if n, err := pg.AttestSourceQuotes(ctx, srcID, "Nazanin"); err != nil || n != 1 {
		t.Fatalf("attest: %d %v", n, err)
	}
	keys = statementKeys(t, pg, id)
	flag := flagsFor(t, pg, keys[0], "pending")[0]
	if err := pg.DecideProposal(ctx, flag.ID, "rejected", "Nazanin", "stands", nil); err != nil {
		t.Fatal(err)
	}
	gateAgrees(t, pg, id)
	if _, err := pg.MarkStatementsReviewed(ctx, id, nil, "Nazanin"); err != nil {
		t.Fatal(err)
	}
	for i, s := range standings(t, pg, id) {
		if s.Status != store.Ready {
			t.Errorf("statement %d after full review = %+v", i+1, s)
		}
	}
	gateAgrees(t, pg, id)

	var page store.PageStanding
	for _, s := range standings(t, pg, id) {
		page.Aggregate(s)
	}
	if !page.Publishable() || page.Total != 2 {
		t.Errorf("page standing = %+v", page)
	}
}

func TestStanding_directoryStatementsAreNeverUnread(t *testing.T) {
	st := store.CitedStatement{BodyMD: "Org.", Citations: []store.CitationWithSource{{SourceKind: "nonprofit", Quote: "q", CheckedAt: nil, ManuallyVerified: true}}}
	if s := st.Standing(true); s.Status != store.Ready {
		t.Errorf("directory entry = %+v", s)
	}
	if s := st.Standing(false); !s.Has(store.ReasonUnread) {
		t.Errorf("playbook statement = %+v", s)
	}
}
