package store_test

import (
	"context"
	"errors"
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

	// Scoped to this test's place: the test database is shared, and
	// publishing every draft in it would pull the ground from under the
	// other tests.
	out, err := pg.PublishReadyDrafts(ctx, "Nazanin", jID)
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
	if _, err := pg.PublishReadyDrafts(ctx, store.ActorDraftingAgent, jID); err == nil {
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

func TestMarkStatementDone_isOneActionForReadNoteAndQuote(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	id := seedPlaybook(t, pg, jID, tID, "draft", "Done")
	pw, _ := pg.AuthorGetPlaybook(ctx, id)
	srcID := pw.Statements[0].Citations[0].SourceID
	keys := statementKeys(t, pg, id)
	saveAs(t, pg, id, jID, tID, store.ActorDraftingAgent, store.IngestStatementParams{
		Key: keys[0], BodyMD: "Claim.", ReviewerNote: "A doubt.",
		Sources: []store.IngestCitationParams{{SourceID: srcID, Locator: "§ 1", Quote: "never confirmed"}},
	})
	keys = statementKeys(t, pg, id)
	if err := pg.MarkStatementDone(ctx, id, keys[0], "Nazanin"); err != nil {
		t.Fatalf("done: %v", err)
	}
	pw, _ = pg.AuthorGetPlaybook(ctx, id)
	st := pw.Statements[0]
	if st.ReviewedAt == nil || st.ReviewedBy != "Nazanin" || len(st.Notes) != 0 || st.Citations[0].CheckedAt == nil || !st.Citations[0].ManuallyVerified {
		t.Errorf("after Done: reviewed=%v by=%q notes=%d checked=%v attested=%v", st.ReviewedAt, st.ReviewedBy, len(st.Notes), st.Citations[0].CheckedAt, st.Citations[0].ManuallyVerified)
	}
	if s := st.Standing(); s.Status != store.Ready {
		t.Errorf("standing after Done = %+v", s)
	}
	if err := pg.AuthorPublishPlaybook(ctx, id, "Nazanin"); err != nil {
		t.Fatalf("publish after Done: %v", err)
	}

	// A drift finding is a question Done cannot answer.
	if _, err := pg.FileProposal(ctx, store.FileProposalParams{StatementKey: keys[0], Reason: "source-drift", ProposedBy: store.ActorSourceCheck}); err != nil {
		t.Fatal(err)
	}
	if err := pg.MarkStatementDone(ctx, id, keys[0], "Nazanin"); !errors.Is(err, store.ErrChangeProposed) {
		t.Errorf("Done over a drift finding: %v", err)
	}
	if err := pg.MarkStatementDone(ctx, id, keys[0], store.ActorDraftingAgent); err == nil {
		t.Error("the agent marked a statement done")
	}
}

func TestReplaceStatement_savesOneStatementInPlace(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	id := seedPlaybook(t, pg, jID, tID, "draft", "In place")
	pw, _ := pg.AuthorGetPlaybook(ctx, id)
	key := pw.Statements[0].Key
	srcID := pw.Statements[0].Citations[0].SourceID
	if err := pg.ReplaceStatement(ctx, id, key, store.IngestStatementParams{
		BodyMD: "Reworded.", Sources: []store.IngestCitationParams{{SourceID: srcID, Locator: "§ 2", Quote: "new words"}},
	}, "Nazanin"); err != nil {
		t.Fatal(err)
	}
	pw, _ = pg.AuthorGetPlaybook(ctx, id)
	st := pw.Statements[0]
	if st.Key != key || st.BodyMD != "Reworded." || st.Citations[0].Locator != "§ 2" || st.Citations[0].Quote != "new words" {
		t.Errorf("after replace: %+v", st)
	}
	if st.Citations[0].CheckedAt != nil {
		t.Error("an unchecked new quote must not inherit the old confirmation")
	}
	if err := pg.ReplaceStatement(ctx, id, "00000000-0000-0000-0000-000000000000", store.IngestStatementParams{BodyMD: "x"}, "Nazanin"); !errors.Is(err, store.ErrProposalTargetGone) {
		t.Errorf("replacing a missing key: %v", err)
	}
}
