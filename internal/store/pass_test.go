package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

func TestPass_stampsUnderTheAgentsNameAndAnEditClearsIt(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	draft := seedPlaybook(t, pg, jID, tID, "draft", "Pass")
	key := firstKey(t, pg, draft)
	src := sourceOf(t, pg, draft)
	// The fixture seeds the statement reviewed by "test"; clear that so the
	// PASS is what stamps it.
	if _, err := pg.Pool().Exec(ctx, `UPDATE statements SET last_reviewed_at = NULL, reviewed_by = '', reviewed_hash = ''`); err != nil {
		t.Fatal(err)
	}
	ev := store.PassEvidence{SourceURL: src.SourceURL, Passage: "verbatim", Reason: "the section says so"}
	if err := pg.PassStatement(ctx, draft, key, ev); err != nil {
		t.Fatalf("pass: %v", err)
	}
	pw, _ := pg.AuthorGetPlaybook(ctx, draft)
	st := pw.Statements[0]
	if st.ReviewedAt == nil || st.ReviewedBy != store.ActorReviewAgent {
		t.Fatalf("after PASS: reviewed %v by %q, want a stamp by the review agent", st.ReviewedAt, st.ReviewedBy)
	}
	issues, _ := pg.AuthorPlaybookIssues(ctx, draft)
	for _, is := range issues {
		if is.Code == "unreviewed-statement" || is.Code == "undecided-item" {
			t.Errorf("gate still reports %s after PASS", is.Code)
		}
	}
	// The PASS is in the audit as an approved decision under the agent's name.
	rows, _ := pg.ListProposals(ctx, "approved")
	found := false
	for _, r := range rows {
		if r.StatementKey == key && r.Reason == store.ReasonAgentPass && r.DecidedBy == store.ActorReviewAgent {
			found = true
		}
	}
	if !found {
		t.Error("PASS not recorded as an approved agent decision")
	}
	// An edit to the words clears the stamp, like a person's.
	if err := pg.ReplaceStatement(ctx, draft, key, replacement(src, "Different words.", true), "Nazanin"); err != nil {
		t.Fatal(err)
	}
	pw, _ = pg.AuthorGetPlaybook(ctx, draft)
	if pw.Statements[0].ReviewedBy == store.ActorReviewAgent {
		t.Error("the agent's stamp survived an edit to the words")
	}
}

func TestPass_refusesAnUndecidedStatement(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	draft := seedPlaybook(t, pg, jID, tID, "draft", "Pass2")
	key := firstKey(t, pg, draft)
	src := sourceOf(t, pg, draft)
	file(t, pg, key, draft, "A competing edit.")
	err := pg.PassStatement(ctx, draft, key, store.PassEvidence{SourceURL: src.SourceURL, Passage: "verbatim", Reason: "r"})
	if !errors.Is(err, store.ErrUndecidedItem) {
		t.Errorf("PASS over an open queue item: err = %v, want ErrUndecidedItem", err)
	}
	if err := pg.PassStatement(ctx, draft, key, store.PassEvidence{}); err == nil {
		t.Error("PASS with no evidence accepted")
	}
}

func TestReaderNote_filesOnceUnderTheAgentsName(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	draft := seedPlaybook(t, pg, jID, tID, "draft", "Note")
	key := firstKey(t, pg, draft)
	for i := 0; i < 2; i++ {
		if err := pg.FileReaderNote(ctx, draft, key, "the statute narrows this to 5 days"); err != nil {
			t.Fatal(err)
		}
	}
	rows, _ := pg.ListProposalsByReason(ctx, "pending", "note")
	n := 0
	for _, r := range rows {
		if r.StatementKey == key && r.ProposedBy == store.ActorReviewAgent {
			n++
		}
	}
	if n != 1 {
		t.Errorf("reader notes on the key = %d, want exactly 1", n)
	}
}

func TestSplit_followersLandAfterTheReplacedStatement(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	draft := seedPlaybook(t, pg, jID, tID, "draft", "Split")
	key := firstKey(t, pg, draft)
	src := sourceOf(t, pg, draft)
	id := file(t, pg, key, draft, "The rule.")
	err := pg.ApproveProposal(ctx, store.ApproveProposalParams{
		ID: id, By: store.ActorReviewAgent, Note: "split",
		Statement: replacement(src, "The rule.", true),
		Followers: []store.IngestStatementParams{replacement(src, "The exception.", true), replacement(src, "The remedy.", true)},
	})
	if err != nil {
		t.Fatalf("split approval: %v", err)
	}
	pw, _ := pg.AuthorGetPlaybook(ctx, draft)
	var bodies []string
	for _, st := range pw.Statements {
		bodies = append(bodies, st.BodyMD)
	}
	if len(bodies) != 3 || bodies[0] != "The rule." || bodies[1] != "The exception." || bodies[2] != "The remedy." {
		t.Fatalf("page after split = %v", bodies)
	}
	if pw.Statements[0].Key != key {
		t.Error("the replaced statement lost its key")
	}
	if pw.Statements[1].Key == key || pw.Statements[2].Key == key || pw.Statements[1].Key == pw.Statements[2].Key {
		t.Error("followers must be new statements with their own keys")
	}
	if pw.Statements[1].ReviewedAt != nil {
		t.Error("a follower starts unreviewed")
	}
}

func TestFlagStatement_recordsAnOverturnOfTheAgentsStamp(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	draft := seedPlaybook(t, pg, jID, tID, "draft", "Flag")
	key := firstKey(t, pg, draft)
	src := sourceOf(t, pg, draft)
	if _, err := pg.Pool().Exec(ctx, `UPDATE statements SET last_reviewed_at = NULL, reviewed_by = '', reviewed_hash = ''`); err != nil {
		t.Fatal(err)
	}
	if err := pg.PassStatement(ctx, draft, key, store.PassEvidence{SourceURL: src.SourceURL, Passage: "verbatim", Reason: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := pg.FlagStatement(ctx, draft, key, "the statute has an exception this leaves out", "Nazanin"); err != nil {
		t.Fatalf("flag: %v", err)
	}
	rows, _ := pg.ListProposalsByReason(ctx, "pending", "note")
	var found bool
	for _, r := range rows {
		if r.StatementKey != key {
			continue
		}
		found = true
		var ev store.ReviewerFlagEvidence
		if err := json.Unmarshal(r.Evidence, &ev); err != nil {
			t.Fatal(err)
		}
		if ev.Overturned != store.ActorReviewAgent {
			t.Errorf("overturned = %q, want the review agent whose stamp it contradicts", ev.Overturned)
		}
		if r.ProposedBy != "Nazanin" {
			t.Errorf("flag filed by %q, want the person", r.ProposedBy)
		}
	}
	if !found {
		t.Fatal("no note filed")
	}
	// The open item stops the page publishing.
	issues, _ := pg.AuthorPlaybookIssues(ctx, draft)
	var blocked bool
	for _, is := range issues {
		if is.Code == "undecided-item" {
			blocked = true
		}
	}
	if !blocked {
		t.Error("a flagged statement does not block the page")
	}
	// An agent cannot flag, and a flag needs a reason.
	if err := pg.FlagStatement(ctx, draft, key, "x", store.ActorReviewAgent); err == nil {
		t.Error("the review agent was allowed to flag a statement")
	}
	if err := pg.FlagStatement(ctx, draft, key, "  ", "Nazanin"); err == nil {
		t.Error("a flag with no reason was accepted")
	}
}

func TestRankProposals_ordersByWhatItKnows(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	// Two slots, so a live page and a draft page do not resolve to the
	// same target (a draft beside a live page IS that page's next version).
	t2, err := pg.UpsertTopic(ctx, store.UpsertTopicParams{Slug: "rank-topic-" + t.Name(), Name: "Rank Topic"})
	if err != nil {
		t.Fatal(err)
	}
	live := seedPlaybook(t, pg, jID, tID, "published", "Ranked")
	draft := seedPlaybook(t, pg, jID, t2.ID, "draft", "Ranked draft")
	liveKey, draftKey := firstKey(t, pg, live), firstKey(t, pg, draft)
	file(t, pg, draftKey, draft, "A draft edit.")
	file(t, pg, liveKey, live, "A live edit.")
	if err := pg.FlagStatement(ctx, live, liveKey, "this reads wrong to me", "Nazanin"); err != nil {
		t.Fatal(err)
	}
	rows, err := pg.ListProposals(ctx, "pending")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 3 {
		t.Fatalf("want at least 3 pending, got %d", len(rows))
	}
	if rows[0].Rank != 1 || rows[0].RankWhy != "live page, claim disputed" {
		t.Errorf("first item is rank %d (%q); a disputed claim on a live page comes first", rows[0].Rank, rows[0].RankWhy)
	}
	var sawLive, sawLast bool
	for _, r := range rows {
		if r.Rank == 2 && r.TargetStatus == "published" {
			sawLive = true
		}
		if r.Rank == 3 && r.RankWhy == "last item on this page" {
			sawLast = true
		}
	}
	if !sawLive || !sawLast {
		t.Errorf("ranks 2 and 3 not both present: live=%v last=%v", sawLive, sawLast)
	}
	for i := 1; i < len(rows); i++ {
		if rows[i-1].Rank > rows[i].Rank {
			t.Fatalf("the list is not ordered by rank: %d before %d", rows[i-1].Rank, rows[i].Rank)
		}
	}
}
