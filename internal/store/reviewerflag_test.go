package store_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// A statement's reviewer note is filed as a work item on its key at save
// time (ADR-018 D1), so the drafting agent's doubt reaches the queue with the
// claim it is about. These tests pin down that it files once, does not stack
// on re-saves, and does not reopen a note a reviewer already decided.

func flagsFor(t *testing.T, pg *store.PG, key, status string) []store.ProposalRow {
	t.Helper()
	rows, err := pg.ListProposals(context.Background(), status)
	if err != nil {
		t.Fatal(err)
	}
	var out []store.ProposalRow
	for _, r := range rows {
		if r.StatementKey == key && r.Reason == store.ReasonReviewerFlag {
			out = append(out, r)
		}
	}
	return out
}

func noteOf(t *testing.T, r store.ProposalRow) string {
	t.Helper()
	var ev store.ReviewerFlagEvidence
	if err := json.Unmarshal(r.Evidence, &ev); err != nil {
		t.Fatalf("evidence %s: %v", r.Evidence, err)
	}
	return ev.Note
}

func TestReviewerNote_filesOneWorkItemOnTheStatementKey(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	id := seedPlaybook(t, pg, jID, tID, "draft", "Notes")
	key := statementKeys(t, pg, id)[0]

	resave(t, pg, id, jID, tID, store.IngestStatementParams{
		Key: key, BodyMD: "Claim one.", ReviewerNote: "Inferred from the statute's silence.",
	})
	got := flagsFor(t, pg, key, "pending")
	if len(got) != 1 {
		t.Fatalf("pending flags = %d, want 1", len(got))
	}
	if got[0].Proposed != nil {
		t.Error("a reviewer note is a work item; it must carry no proposed text")
	}
	if noteOf(t, got[0]) != "Inferred from the statute's silence." {
		t.Errorf("note = %q", noteOf(t, got[0]))
	}
	if got[0].PlaybookID != id {
		t.Errorf("filed against page %d, want %d", got[0].PlaybookID, id)
	}

	// Autosave carries the same note forward: nothing stacks.
	resave(t, pg, id, jID, tID, store.IngestStatementParams{
		Key: key, BodyMD: "Claim one.", ReviewerNote: "Inferred from the statute's silence.",
	})
	if n := len(flagsFor(t, pg, key, "pending")); n != 1 {
		t.Errorf("after an identical re-save, pending flags = %d, want 1", n)
	}

	// A different note on the same claim replaces the pending one.
	resave(t, pg, id, jID, tID, store.IngestStatementParams{
		Key: key, BodyMD: "Claim one.", ReviewerNote: "Guidance only, no statute.",
	})
	if n := len(flagsFor(t, pg, key, "pending")); n != 1 {
		t.Errorf("after a new note, pending flags = %d, want 1", n)
	}
	if n := len(flagsFor(t, pg, key, "superseded")); n != 1 {
		t.Errorf("superseded flags = %d, want 1", n)
	}
}

func TestReviewerNote_decidedNoteIsNotReopenedByResave(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	id := seedPlaybook(t, pg, jID, tID, "draft", "Notes")
	key := statementKeys(t, pg, id)[0]
	ctx := context.Background()

	resave(t, pg, id, jID, tID, store.IngestStatementParams{
		Key: key, BodyMD: "Claim one.", ReviewerNote: "Check the deadline.",
	})
	flag := flagsFor(t, pg, key, "pending")[0]
	if err := pg.DecideProposal(ctx, flag.ID, "rejected", "Nazanin", "checked, it stands", nil); err != nil {
		t.Fatal(err)
	}

	resave(t, pg, id, jID, tID, store.IngestStatementParams{
		Key: key, BodyMD: "Claim one.", ReviewerNote: "Check the deadline.",
	})
	if n := len(flagsFor(t, pg, key, "pending")); n != 0 {
		t.Errorf("a rejected note came back on re-save: pending flags = %d", n)
	}
}

func TestReviewerNote_emptyFilesNothing(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	id := seedPlaybook(t, pg, jID, tID, "draft", "Notes")
	key := statementKeys(t, pg, id)[0]
	resave(t, pg, id, jID, tID, store.IngestStatementParams{Key: key, BodyMD: "Claim one.", ReviewerNote: "   "})
	if n := len(flagsFor(t, pg, key, "pending")); n != 0 {
		t.Errorf("blank note filed %d flags", n)
	}
}
