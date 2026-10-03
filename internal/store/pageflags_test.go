package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

func pageIssueCodes(t *testing.T, pg *store.PG, id int64) []string {
	t.Helper()
	issues, err := pg.AuthorPlaybookIssues(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, is := range issues {
		out = append(out, is.Code)
	}
	return out
}

func TestPageFlag_holdsPublishUntilClosed(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	id := seedPlaybook(t, pg, jID, tID, "draft", "Flags")
	if codes := pageIssueCodes(t, pg, id); len(codes) != 0 {
		t.Fatalf("fixture not clean: %v", codes)
	}

	fid, filed, err := pg.FilePageFlag(ctx, id, "thin", "The page has one statement.", nil, store.ActorReviewAgent)
	if err != nil || !filed {
		t.Fatalf("file: filed=%v err=%v", filed, err)
	}
	if _, again, err := pg.FilePageFlag(ctx, id, "thin", "The page has one statement.", nil, store.ActorReviewAgent); err != nil || again {
		t.Fatalf("the same note filed twice: again=%v err=%v", again, err)
	}
	if codes := pageIssueCodes(t, pg, id); len(codes) != 1 || codes[0] != "page-flag" {
		t.Fatalf("issues = %v, want [page-flag]", codes)
	}
	var npe *store.NotPublishableError
	if err := pg.AuthorPublishPlaybook(ctx, id, "test"); !errors.As(err, &npe) {
		t.Fatalf("publish over an open page flag: %v", err)
	}
	// The flag sits on the page, not on a statement.
	pw, _ := pg.AuthorGetPlaybook(ctx, id)
	if pw.Statements[0].Undecided {
		t.Fatal("a page flag put an open item on the first statement")
	}

	if err := pg.ClosePageFlag(ctx, fid, store.ActorReviewAgent, ""); err == nil {
		t.Fatal("an agent closed a flag without saying what it did")
	}
	if err := pg.ClosePageFlag(ctx, fid, store.ActorReviewAgent, "Added two statements."); err != nil {
		t.Fatal(err)
	}
	if err := pg.ClosePageFlag(ctx, fid, "Nazanin", ""); !errors.Is(err, store.ErrPageFlagNotOpen) {
		t.Fatalf("second close: %v", err)
	}
	if codes := pageIssueCodes(t, pg, id); len(codes) != 0 {
		t.Fatalf("issues after close = %v", codes)
	}
	// Closed stays closed: the same note is not filed again.
	if _, again, _ := pg.FilePageFlag(ctx, id, "thin", "The page has one statement.", nil, store.ActorReviewAgent); again {
		t.Fatal("re-filing reopened a closed flag")
	}
	if err := pg.AuthorPublishPlaybook(ctx, id, "test"); err != nil {
		t.Fatalf("publish after close: %v", err)
	}
}

func TestPageFlag_personsFlagClosedByPerson(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	id := seedPlaybook(t, pg, jID, tID, "draft", "Persons")
	keys := statementKeys(t, pg, id)
	fid, _, err := pg.FilePageFlag(ctx, id, "intro", "The intro promises a remedy the page never gives.", keys, "Nazanin")
	if err != nil {
		t.Fatal(err)
	}
	f, err := pg.PageFlagByID(ctx, fid)
	if err != nil || !f.FiledByPerson() || len(f.Keys) != 1 || f.Keys[0] != keys[0] {
		t.Fatalf("flag = %+v, err %v", f, err)
	}
	if err := pg.ClosePageFlag(ctx, fid, store.ActorReviewAgent, "Rewrote the intro."); !errors.Is(err, store.ErrPageFlagPersons) {
		t.Fatalf("agent closed a person's flag: %v", err)
	}
	if err := pg.ClosePageFlag(ctx, fid, "Nazanin", ""); err != nil {
		t.Fatalf("person close: %v", err)
	}
}

func TestPageFlag_drafterPageNote(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	id := seedPlaybook(t, pg, jID, tID, "draft", "Drafter")
	pw, _ := pg.AuthorGetPlaybook(ctx, id)
	save := func() {
		t.Helper()
		if err := pg.IngestPlaybook(ctx, store.IngestPlaybookParams{
			JurisdictionID: jID, TopicID: tID, Language: "en", Slug: pw.Slug,
			Title: "Drafter", IntroMD: "intro", Status: "draft", UpdatedBy: store.ActorDraftingAgent,
			PageNote: "I found no source for repairs over $500.",
			Statements: []store.IngestStatementParams{{
				BodyMD: "A claim. Drafter", Language: "en",
				Sources: []store.IngestCitationParams{{SourceID: pw.Statements[0].Citations[0].SourceID, Locator: "§ 1", Quote: "verbatim", CheckedNow: true, CheckedBy: "test"}},
			}},
		}); err != nil {
			t.Fatal(err)
		}
	}
	save()
	save() // a re-save carrying the same note files it once
	flags, err := pg.OpenPageFlags(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(flags) != 1 || flags[0].Kind != store.PageFlagDrafter || flags[0].FiledBy != store.ActorDraftingAgent {
		t.Fatalf("flags = %+v", flags)
	}
}
