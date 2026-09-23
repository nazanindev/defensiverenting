package store_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// threeStatements seeds a page with statements A, B, C on one source and
// returns the page, its keys, and the source.
func threeStatements(t *testing.T, status string) (*store.PG, int64, []string, int64) {
	t.Helper()
	pg, jID, tID := revisionFixture(t)
	id := seedPlaybook(t, pg, jID, tID, status, "Actions")
	if status == "draft" {
		resave(t, pg, id, jID, tID,
			store.IngestStatementParams{BodyMD: "A."},
			store.IngestStatementParams{BodyMD: "B."},
			store.IngestStatementParams{BodyMD: "C."})
	}
	return pg, id, statementKeys(t, pg, id), sourceOf(t, pg, id).SourceID
}

func fileAction(t *testing.T, pg *store.PG, key string, page int64, ps store.ProposedStatement) int64 {
	t.Helper()
	id, err := pg.FileProposal(context.Background(), store.FileProposalParams{
		StatementKey: key, PlaybookID: page, Reason: "agent-pass:page", Proposed: &ps, ProposedBy: "triage agent",
	})
	if err != nil {
		t.Fatalf("file %s: %v", ps.Action, err)
	}
	return id
}

func bodies(t *testing.T, pg *store.PG, id int64) []string {
	t.Helper()
	pw, err := pg.AuthorGetPlaybook(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, st := range pw.Statements {
		out = append(out, st.BodyMD)
	}
	return out
}

func TestPageAction_remove(t *testing.T) {
	pg, id, keys, _ := threeStatements(t, "draft")
	ctx := context.Background()
	note, _ := pg.FileReviewerNote(ctx, id, keys[1], "Page review (duplicate): B repeats A.", store.ActorReviewAgent)
	if !note {
		t.Fatal("note not filed")
	}
	p := fileAction(t, pg, keys[1], id, store.ProposedStatement{Action: store.ActionRemove})
	if err := pg.ApproveProposal(ctx, store.ApproveProposalParams{ID: p, By: store.ActorReviewAgent, Note: "duplicate", Action: store.ActionRemove}); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if got := bodies(t, pg, id); !slices.Equal(got, []string{"A.", "C."}) {
		t.Errorf("after remove: %v", got)
	}
	if left := flagsFor(t, pg, keys[1], "pending"); len(left) != 0 {
		t.Errorf("the removed statement's note should close with it, %d still pending", len(left))
	}
}

func TestPageAction_mergeKeepsEveryCitation(t *testing.T) {
	pg, id, keys, src := threeStatements(t, "draft")
	ctx := context.Background()
	merged := store.IngestStatementParams{BodyMD: "A and B.", Sources: []store.IngestCitationParams{{SourceID: src, Locator: "§ 1", Quote: "verbatim"}}}
	p := fileAction(t, pg, keys[0], id, store.ProposedStatement{Action: store.ActionMerge, MergeKey: keys[1], BodyMD: "A and B.", Citations: []store.ProposedCitation{{URL: "https://example.gov/x", Quote: "verbatim"}}})
	if err := pg.ApproveProposal(ctx, store.ApproveProposalParams{ID: p, By: store.ActorReviewAgent, Statement: merged, Action: store.ActionMerge, MergeKey: keys[1]}); err != nil {
		t.Fatalf("merge: %v", err)
	}
	if got := bodies(t, pg, id); !slices.Equal(got, []string{"A and B.", "C."}) {
		t.Errorf("after merge: %v", got)
	}
	if k := statementKeys(t, pg, id); k[0] != keys[0] {
		t.Errorf("the merged statement keeps the first key: %s, want %s", k[0], keys[0])
	}

	pg2, id2, keys2, src2 := threeStatements(t, "draft")
	dropped := store.IngestStatementParams{BodyMD: "A and B.", Sources: []store.IngestCitationParams{{SourceID: src2, Locator: "§ 1", Quote: "something else"}}}
	p2 := fileAction(t, pg2, keys2[0], id2, store.ProposedStatement{Action: store.ActionMerge, MergeKey: keys2[1], BodyMD: "A and B.", Citations: []store.ProposedCitation{{URL: "https://example.gov/x", Quote: "something else"}}})
	err := pg2.ApproveProposal(ctx, store.ApproveProposalParams{ID: p2, By: store.ActorReviewAgent, Statement: dropped, Action: store.ActionMerge, MergeKey: keys2[1]})
	if err == nil || !strings.Contains(err.Error(), "keeps every citation") {
		t.Errorf("a merge that drops a citation must be refused, got %v", err)
	}
	if got := bodies(t, pg2, id2); !slices.Equal(got, []string{"A.", "B.", "C."}) {
		t.Errorf("a refused merge must write nothing: %v", got)
	}
}

func TestPageAction_reorder(t *testing.T) {
	pg, id, keys, _ := threeStatements(t, "draft")
	ctx := context.Background()
	order := []string{keys[2], keys[0], keys[1]}
	p := fileAction(t, pg, keys[2], id, store.ProposedStatement{Action: store.ActionReorder, Order: order})
	if err := pg.ApproveProposal(ctx, store.ApproveProposalParams{ID: p, By: store.ActorReviewAgent, Action: store.ActionReorder, Order: order}); err != nil {
		t.Fatalf("reorder: %v", err)
	}
	if got := bodies(t, pg, id); !slices.Equal(got, []string{"C.", "A.", "B."}) {
		t.Errorf("after reorder: %v", got)
	}
	bad := []string{keys[0], keys[1]}
	p2 := fileAction(t, pg, keys[0], id, store.ProposedStatement{Action: store.ActionReorder, Order: bad})
	if err := pg.ApproveProposal(ctx, store.ApproveProposalParams{ID: p2, By: store.ActorReviewAgent, Action: store.ActionReorder, Order: bad}); err == nil {
		t.Error("a reorder that leaves a statement out must be refused")
	}
}

func TestPageAction_livePageIsAPersonsDecision(t *testing.T) {
	pg, id, keys, _ := threeStatements(t, "published")
	ctx := context.Background()
	p := fileAction(t, pg, keys[0], id, store.ProposedStatement{Action: store.ActionRemove})
	err := pg.ApproveProposal(ctx, store.ApproveProposalParams{ID: p, By: store.ActorReviewAgent, Action: store.ActionRemove})
	if err == nil || !strings.Contains(err.Error(), "person's decision") {
		t.Errorf("an agent must not remove from a live page, got %v", err)
	}
}

func TestFileProposal_refusesMalformedActions(t *testing.T) {
	pg, id, keys, _ := threeStatements(t, "draft")
	for name, ps := range map[string]store.ProposedStatement{
		"unknown action":        {Action: "shuffle"},
		"merge without key":     {Action: store.ActionMerge, BodyMD: "x", Citations: []store.ProposedCitation{{URL: "https://example.gov/x", Quote: "q"}}},
		"merge without body":    {Action: store.ActionMerge, MergeKey: keys[1]},
		"reorder without order": {Action: store.ActionReorder},
	} {
		if _, err := pg.FileProposal(context.Background(), store.FileProposalParams{StatementKey: keys[0], PlaybookID: id, Reason: "agent-pass:page", Proposed: &ps, ProposedBy: "triage agent"}); err == nil {
			t.Errorf("%s: want refused", name)
		}
	}
}

func TestPageAction_mergeIntoLeadAndFollower(t *testing.T) {
	pg, id, keys, src := threeStatements(t, "draft")
	ctx := context.Background()
	lead := store.IngestStatementParams{BodyMD: "A and part of B.", Sources: []store.IngestCitationParams{{SourceID: src, Locator: "§ 1", Quote: "other"}}}
	follower := store.IngestStatementParams{BodyMD: "The rest of B.", Sources: []store.IngestCitationParams{{SourceID: src, Locator: "§ 1", Quote: "verbatim"}}}
	p := fileAction(t, pg, keys[0], id, store.ProposedStatement{Action: store.ActionMerge, MergeKey: keys[1], BodyMD: "A and part of B.", Citations: []store.ProposedCitation{{URL: "https://example.gov/x", Quote: "other"}}})
	if err := pg.ApproveProposal(ctx, store.ApproveProposalParams{ID: p, By: store.ActorReviewAgent, Statement: lead, Followers: []store.IngestStatementParams{follower}, Action: store.ActionMerge, MergeKey: keys[1]}); err != nil {
		t.Fatalf("a citation carried by the follower still counts: %v", err)
	}
	if got := bodies(t, pg, id); !slices.Equal(got, []string{"A and part of B.", "The rest of B.", "C."}) {
		t.Errorf("after merge with follower: %v", got)
	}
}
