package store_test

import (
	"context"
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
