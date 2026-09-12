package store_test

import (
	"context"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// Group review (ADR-018 D4, D5): the by-source list, attestation over a
// source, and publishing every draft the gate passes.

func TestReviewBySource_listsAttestsAndStamps(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	id := seedPlaybook(t, pg, jID, tID, "draft", "By source")
	pw, _ := pg.AuthorGetPlaybook(ctx, id)
	srcID := pw.Statements[0].Citations[0].SourceID
	keys := statementKeys(t, pg, id)

	// The agent leaves two statements citing the source, one with a quote
	// the checker never confirmed.
	saveAs(t, pg, id, jID, tID, store.ActorDraftingAgent,
		store.IngestStatementParams{Key: keys[0], BodyMD: "Claim one."},
		store.IngestStatementParams{BodyMD: "Claim two.",
			Sources: []store.IngestCitationParams{{SourceID: srcID, Locator: "§ 2", Quote: "other words"}}})

	var mine store.SourceReviewSummary
	all, err := pg.ReviewSourcesOverview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range all {
		if s.SourceID == srcID {
			mine = s
		}
	}
	if mine.Statements != 2 || mine.Unreviewed != 2 || mine.Unconfirmed != 1 {
		t.Fatalf("overview = %+v", mine)
	}

	rows, err := pg.ReviewStatementsBySource(ctx, srcID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].PlaybookID != id || rows[1].Position != 2 {
		t.Fatalf("rows = %+v", rows)
	}
	if len(rows[1].Stmt.Citations) != 1 || rows[1].Stmt.Citations[0].Quote != "other words" {
		t.Errorf("row 2 citations = %+v", rows[1].Stmt.Citations)
	}

	n, err := pg.AttestSourceQuotes(ctx, srcID, "Nazanin")
	if err != nil || n != 1 {
		t.Fatalf("attest: n=%d err=%v", n, err)
	}
	if _, err := pg.AttestSourceQuotes(ctx, srcID, store.ActorDraftingAgent); err == nil {
		t.Error("the agent attested quotes")
	}
	all, _ = pg.ReviewSourcesOverview(ctx)
	for _, s := range all {
		if s.SourceID == srcID && s.Unconfirmed != 0 {
			t.Errorf("after attesting, unconfirmed = %d", s.Unconfirmed)
		}
	}

	// Stamping through the group path is the same stamp as the page view.
	keys = statementKeys(t, pg, id)
	if res, err := pg.MarkStatementsReviewed(ctx, id, keys, "Nazanin"); err != nil || res.Stamped != 2 {
		t.Fatalf("mark: %+v %v", res, err)
	}
	if err := pg.AuthorPublishPlaybook(ctx, id, "Nazanin"); err != nil {
		t.Fatalf("publish after group review: %v", err)
	}
}

func TestPublishReadyDrafts_publishesOnlyWhatTheGatePasses(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	ready := seedPlaybook(t, pg, jID, tID, "draft", "Ready")

	// A second draft in another slot, left unreviewed by an agent save.
	tp, err := pg.UpsertTopic(ctx, store.UpsertTopicParams{Slug: "held-topic-" + t.Name(), Name: "Held"})
	if err != nil {
		t.Fatal(err)
	}
	held := seedPlaybook(t, pg, jID, tp.ID, "draft", "Held")
	keys := statementKeys(t, pg, held)
	saveAs(t, pg, held, jID, tp.ID, store.ActorDraftingAgent, store.IngestStatementParams{Key: keys[0], BodyMD: "Changed by the agent."})

	out, err := pg.PublishReadyDrafts(ctx, "Nazanin")
	if err != nil {
		t.Fatal(err)
	}
	got := map[int64]store.PublishOutcome{}
	for _, o := range out {
		got[o.PlaybookID] = o
	}
	if !got[ready].Published {
		t.Errorf("ready draft not published: %+v", got[ready])
	}
	if got[held].Published || len(got[held].Issues) == 0 {
		t.Errorf("held draft should be refused with issues: %+v", got[held])
	}
	if _, err := pg.PublishReadyDrafts(ctx, store.ActorDraftingAgent); err == nil {
		t.Error("the agent published")
	}
}

func TestListProposalsByReason_separatesNotesFromDrift(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	id := seedPlaybook(t, pg, jID, tID, "draft", "Kinds")
	key := statementKeys(t, pg, id)[0]
	resave(t, pg, id, jID, tID, store.IngestStatementParams{Key: key, BodyMD: "Claim.", ReviewerNote: "A doubt."})
	if _, err := pg.FileProposal(ctx, store.FileProposalParams{StatementKey: key, Reason: "source-drift", ProposedBy: store.ActorSourceCheck}); err != nil {
		t.Fatal(err)
	}
	count := func(kind string) int {
		rows, err := pg.ListProposalsByReason(ctx, "pending", kind)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, r := range rows {
			if r.StatementKey == key {
				n++
			}
		}
		return n
	}
	if count("note") != 1 || count("drift") != 1 || count("other") != 0 || count("") != 2 {
		t.Errorf("note=%d drift=%d other=%d all=%d", count("note"), count("drift"), count("other"), count(""))
	}
}
