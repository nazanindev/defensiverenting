package store_test

import (
	"context"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// Two subsections of one statute page are two citations. Until migration
// 000044 the key was (statement, source), so the save kept the last quote and
// dropped the first; 154 approved proposals lost quotes that way.
func TestApproveProposal_keepsEveryQuoteFromOneSource(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	draft := seedPlaybook(t, pg, jID, tID, "draft", "Two quotes")
	key := firstKey(t, pg, draft)
	src := sourceOf(t, pg, draft)

	id := file(t, pg, key, draft, "Two claims. One page.")
	err := pg.ApproveProposal(ctx, store.ApproveProposalParams{
		ID: id, By: store.ActorReviewAgent, Note: "edit rule",
		Statement: store.IngestStatementParams{BodyMD: "Two claims. One page.", Sources: []store.IngestCitationParams{
			{SourceID: src.SourceID, Locator: "§ 1(a)", Quote: "first subsection"},
			{SourceID: src.SourceID, Locator: "§ 1(b)", Quote: "second subsection"},
			{SourceID: src.SourceID, Locator: "§ 1(b)", Quote: "second subsection"}, // same quote twice is one citation
		}},
	})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	after, _ := pg.AuthorGetPlaybook(ctx, draft)
	cs := after.Statements[0].Citations
	if len(cs) != 2 {
		t.Fatalf("got %d citations, want 2 (both quotes from one source, the repeat folded): %+v", len(cs), cs)
	}
	if cs[0].Quote != "first subsection" || cs[1].Quote != "second subsection" {
		t.Errorf("quotes = %q, %q; want both, in the order given", cs[0].Quote, cs[1].Quote)
	}
}
