package store_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// widened returns the seeded statement with its one quote grown to hold the
// original, the shape of a widen-quote proposal (ADR-021 D2).
func widened(src store.CitationWithSource, body, quote string) store.IngestStatementParams {
	return store.IngestStatementParams{BodyMD: body, Sources: []store.IngestCitationParams{{
		SourceID: src.SourceID, Locator: "§ 1", Quote: quote, CheckedNow: true, CheckedBy: store.ActorReviewAgent,
	}}}
}

func TestReviewAgent_widenedQuoteKeepsThePersonsStamp(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	live := seedPlaybook(t, pg, jID, tID, "published", "Widen")
	key := firstKey(t, pg, live)
	src := sourceOf(t, pg, live)
	before, _ := pg.AuthorGetPlaybook(ctx, live)
	if before.Statements[0].ReviewedAt == nil || before.Statements[0].ReviewedBy != "test" {
		t.Fatalf("fixture statement should start reviewed by test, got %v/%q", before.Statements[0].ReviewedAt, before.Statements[0].ReviewedBy)
	}

	id := file(t, pg, key, live, "A claim. Widen")
	err := pg.ApproveProposal(ctx, store.ApproveProposalParams{
		ID: id, By: store.ActorReviewAgent, Note: "widen-quote rule",
		Statement: widened(src, "A claim. Widen", "the whole subsection, verbatim, and more"),
	})
	if err != nil {
		t.Fatalf("agent approval of a widened quote on a live page: %v", err)
	}
	after, _ := pg.AuthorGetPlaybook(ctx, live)
	st := after.Statements[0]
	if st.Citations[0].Quote != "the whole subsection, verbatim, and more" {
		t.Errorf("quote after approval = %q", st.Citations[0].Quote)
	}
	if st.ReviewedAt == nil || st.ReviewedBy != "test" {
		t.Errorf("stamp after widening = %v by %q; want the person's stamp carried (body and evidence unchanged, more of it shown)", st.ReviewedAt, st.ReviewedBy)
	}
	if st.ReviewedAt != nil && !st.ReviewedAt.Equal(*before.Statements[0].ReviewedAt) {
		t.Errorf("stamp time moved from %v to %v; a carried stamp keeps its time", before.Statements[0].ReviewedAt, st.ReviewedAt)
	}
	if after.UpdatedBy != store.ActorReviewAgent {
		t.Errorf("updated_by = %q, want the review agent, which made the save", after.UpdatedBy)
	}
	p, _ := pg.GetProposal(ctx, id)
	if p.Status != "approved" || p.DecidedBy != store.ActorReviewAgent || p.DecisionNote != "widen-quote rule" {
		t.Errorf("proposal after agent approval: %q by %q note %q", p.Status, p.DecidedBy, p.DecisionNote)
	}
}

func TestReviewAgent_rewordingLeavesTheStatementUnreviewed(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	draft := seedPlaybook(t, pg, jID, tID, "draft", "Reword")
	key := firstKey(t, pg, draft)
	src := sourceOf(t, pg, draft)

	id := file(t, pg, key, draft, "A different claim.")
	if err := pg.ApproveProposal(ctx, store.ApproveProposalParams{
		ID: id, By: store.ActorReviewAgent, Statement: widened(src, "A different claim.", "verbatim"),
	}); err != nil {
		t.Fatalf("agent approval on a draft: %v", err)
	}
	after, _ := pg.AuthorGetPlaybook(ctx, draft)
	if st := after.Statements[0]; st.ReviewedAt != nil {
		t.Errorf("a reworded statement approved by the agent reads as reviewed by %q; review is a person's sign-off", st.ReviewedBy)
	}
	// And the agent cannot stamp one by hand either.
	if _, err := pg.MarkStatementsReviewed(ctx, draft, nil, store.ActorReviewAgent); err == nil {
		t.Error("MarkStatementsReviewed accepted the review agent as a reviewer")
	}
}

func TestReviewAgent_shrunkOrMovedQuoteDropsTheStamp(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	draft := seedPlaybook(t, pg, jID, tID, "draft", "Shrink")
	key := firstKey(t, pg, draft)
	src := sourceOf(t, pg, draft)

	id := file(t, pg, key, draft, "A claim. Shrink")
	// "verbatim" is not contained in "verb": the new quote is not a widening.
	if err := pg.ApproveProposal(ctx, store.ApproveProposalParams{
		ID: id, By: store.ActorReviewAgent, Statement: widened(src, "A claim. Shrink", "verb"),
	}); err != nil {
		t.Fatal(err)
	}
	after, _ := pg.AuthorGetPlaybook(ctx, draft)
	if st := after.Statements[0]; st.ReviewedAt != nil {
		t.Errorf("a quote that no longer contains what the person read kept their stamp (by %q)", st.ReviewedBy)
	}
}

func TestApproval_onLivePageIgnoresOtherPendingItems(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	live := seedPlaybook(t, pg, jID, tID, "published", "Siblings")
	key := firstKey(t, pg, live)
	src := sourceOf(t, pg, live)

	// An open question on the statement (a work item) and, filed after it,
	// the widening. Approving the widening is a decision about the widening
	// alone; the question stays open.
	flag, err := pg.FileProposal(ctx, store.FileProposalParams{
		StatementKey: key, PlaybookID: live, Reason: store.ReasonReviewerFlag,
		Evidence: json.RawMessage(`{"note":"is 30 days right?"}`), ProposedBy: "test agent",
	})
	if err != nil {
		t.Fatal(err)
	}
	widen := file(t, pg, key, live, "A claim. Siblings")
	err = pg.ApproveProposal(ctx, store.ApproveProposalParams{
		ID: widen, By: store.ActorReviewAgent, Statement: widened(src, "A claim. Siblings", "verbatim and the rest of the subsection"),
	})
	if err != nil {
		t.Fatalf("approval on a live page with another pending item: %v (ADR-021 D6: the gate ignores the other items on an approval)", err)
	}
	after, _ := pg.AuthorGetPlaybook(ctx, live)
	if q := after.Statements[0].Citations[0].Quote; q != "verbatim and the rest of the subsection" {
		t.Errorf("live page quote = %q, want the widened one", q)
	}
	f, _ := pg.GetProposal(ctx, flag)
	if f.Status != "pending" {
		t.Errorf("the open question was %s by the approval; it must stay pending", f.Status)
	}
	// Publishing the page by hand still refuses over it.
	if _, err := pg.Pool().Exec(ctx, `UPDATE playbooks SET status='draft' WHERE id=$1`, live); err != nil {
		t.Fatal(err)
	}
	if err := pg.AuthorPublishPlaybook(ctx, live, "Nazanin"); err == nil {
		t.Error("publish went through over an undecided item; the gate must still refuse a publish")
	}
}
