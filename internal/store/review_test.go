package store_test

import (
	"context"
	"strings"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// Review is per statement, publishing per page (ADR-018). These tests pin
// the stamp's rules: who can set it, when a save sets it, when an edit voids
// it, what blocks it, and which pages are exempt.

func reviewedAt(t *testing.T, pg *store.PG, id int64) []bool {
	t.Helper()
	pw, err := pg.AuthorGetPlaybook(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]bool, len(pw.Statements))
	for i, st := range pw.Statements {
		out[i] = st.ReviewedAt != nil
		if (st.ReviewedAt != nil) != (st.ReviewedBy != "") {
			t.Errorf("statement %d: ReviewedAt %v but ReviewedBy %q", i+1, st.ReviewedAt, st.ReviewedBy)
		}
	}
	return out
}

// saveAs is resave with a chosen actor.
func saveAs(t *testing.T, pg *store.PG, id, jID, tID int64, by string, stmts ...store.IngestStatementParams) {
	t.Helper()
	ctx := context.Background()
	pw, err := pg.AuthorGetPlaybook(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	srcID := pw.Statements[0].Citations[0].SourceID
	for i := range stmts {
		stmts[i].Language = "en"
		if stmts[i].Sources == nil {
			stmts[i].Sources = []store.IngestCitationParams{{SourceID: srcID, Locator: "§ 1", Quote: "verbatim", CheckedNow: true, CheckedBy: by}}
		}
	}
	if err := pg.AuthorUpdatePlaybook(ctx, store.AuthorUpdatePlaybookParams{
		ID: id, JurisdictionID: jID, TopicID: tID, Language: "en",
		Slug: pw.Slug, Title: pw.Title, IntroMD: pw.IntroMD, UpdatedBy: by,
		Statements: stmts,
	}); err != nil {
		t.Fatalf("save as %s: %v", by, err)
	}
}

func TestReview_agentDraftIsUnreviewedUntilAPersonStampsIt(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	id := seedPlaybook(t, pg, jID, tID, "draft", "Agent draft")
	keys := statementKeys(t, pg, id)
	saveAs(t, pg, id, jID, tID, store.ActorDraftingAgent,
		store.IngestStatementParams{Key: keys[0], BodyMD: "Claim one."},
		store.IngestStatementParams{BodyMD: "Claim two."})

	if got := reviewedAt(t, pg, id); got[0] || got[1] {
		t.Fatalf("an agent save must never stamp review, got %v", got)
	}
	err := pg.AuthorPublishPlaybook(ctx, id, "Nazanin")
	if err == nil || !strings.Contains(err.Error(), "have not been reviewed") {
		t.Fatalf("publish of an unreviewed draft: %v", err)
	}

	keys = statementKeys(t, pg, id)
	res, err := pg.MarkStatementsReviewed(ctx, id, []string{keys[1]}, "Nazanin")
	if err != nil || res.Stamped != 1 {
		t.Fatalf("mark one: %+v %v", res, err)
	}
	if got := reviewedAt(t, pg, id); got[0] || !got[1] {
		t.Fatalf("after marking statement 2 only: %v", got)
	}
	if res, err = pg.MarkStatementsReviewed(ctx, id, nil, "Nazanin"); err != nil || res.Stamped != 1 {
		t.Fatalf("mark the rest: %+v %v", res, err)
	}
	if err := pg.AuthorPublishPlaybook(ctx, id, "Nazanin"); err != nil {
		t.Fatalf("a fully reviewed page must publish: %v", err)
	}
	pw, _ := pg.AuthorGetPlaybook(ctx, id)
	if pw.Statements[1].ReviewedBy != "Nazanin" {
		t.Errorf("reviewed_by = %q", pw.Statements[1].ReviewedBy)
	}
}

func TestReview_aPersonsSaveStampsOnlyWhatTheyWrote(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	id := seedPlaybook(t, pg, jID, tID, "draft", "Edited")
	keys := statementKeys(t, pg, id)
	// The agent leaves two unreviewed statements.
	saveAs(t, pg, id, jID, tID, store.ActorDraftingAgent,
		store.IngestStatementParams{Key: keys[0], BodyMD: "Claim one."},
		store.IngestStatementParams{BodyMD: "Claim two."})
	keys = statementKeys(t, pg, id)

	// A person rewords the second and adds a third; the first is carried as is.
	saveAs(t, pg, id, jID, tID, "Nazanin",
		store.IngestStatementParams{Key: keys[0], BodyMD: "Claim one."},
		store.IngestStatementParams{Key: keys[1], BodyMD: "Claim two, reworded."},
		store.IngestStatementParams{BodyMD: "Claim three."})
	if got := reviewedAt(t, pg, id); got[0] || !got[1] || !got[2] {
		t.Fatalf("stamps after a person's edit = %v, want [false true true]", got)
	}
}

func TestReview_changingTheEvidenceVoidsTheStamp(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	id := seedPlaybook(t, pg, jID, tID, "draft", "Voided")
	if _, err := pg.MarkStatementsReviewed(ctx, id, nil, "Nazanin"); err != nil {
		t.Fatal(err)
	}
	if got := reviewedAt(t, pg, id); !got[0] {
		t.Fatal("stamp did not take")
	}
	keys := statementKeys(t, pg, id)
	pw, _ := pg.AuthorGetPlaybook(ctx, id)
	srcID := pw.Statements[0].Citations[0].SourceID

	// Same words, different quote: the reviewer never saw this evidence.
	saveAs(t, pg, id, jID, tID, store.ActorDraftingAgent, store.IngestStatementParams{
		Key: keys[0], BodyMD: pw.Statements[0].BodyMD,
		Sources: []store.IngestCitationParams{{SourceID: srcID, Locator: "§ 1", Quote: "different words", CheckedNow: true, CheckedBy: "x"}},
	})
	if got := reviewedAt(t, pg, id); got[0] {
		t.Fatal("a changed quote must void the review stamp")
	}
	// Restoring the reviewed content restores the stamp: it is the hash that
	// is honoured, not the row.
	saveAs(t, pg, id, jID, tID, store.ActorDraftingAgent, store.IngestStatementParams{Key: keys[0], BodyMD: pw.Statements[0].BodyMD})
	if got := reviewedAt(t, pg, id); !got[0] {
		t.Fatal("the original content is what was reviewed; its stamp should read again")
	}
}

func TestReview_undecidedNoteBlocksTheStampAndThePage(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	id := seedPlaybook(t, pg, jID, tID, "draft", "Doubt")
	keys := statementKeys(t, pg, id)
	saveAs(t, pg, id, jID, tID, store.ActorDraftingAgent,
		store.IngestStatementParams{Key: keys[0], BodyMD: "Claim one.", ReviewerNote: "Inferred from silence."})

	res, err := pg.MarkStatementsReviewed(ctx, id, nil, "Nazanin")
	if err != nil || res.Stamped != 0 || res.Undecided != 1 {
		t.Fatalf("mark with a pending note: %+v %v", res, err)
	}
	pw, _ := pg.AuthorGetPlaybook(ctx, id)
	if !pw.Statements[0].Undecided {
		t.Error("the view must show the statement as undecided")
	}
	err = pg.AuthorPublishPlaybook(ctx, id, "Nazanin")
	if err == nil || !strings.Contains(err.Error(), "undecided item") {
		t.Fatalf("publish over an undecided note: %v", err)
	}

	flag := flagsFor(t, pg, keys[0], "pending")[0]
	if err := pg.DecideProposal(ctx, flag.ID, "rejected", "Nazanin", "the statute is silent; the claim stands", nil); err != nil {
		t.Fatal(err)
	}
	if res, err = pg.MarkStatementsReviewed(ctx, id, nil, "Nazanin"); err != nil || res.Stamped != 1 {
		t.Fatalf("mark after deciding: %+v %v", res, err)
	}
	if err := pg.AuthorPublishPlaybook(ctx, id, "Nazanin"); err != nil {
		t.Fatalf("publish after deciding and reviewing: %v", err)
	}
}

func TestReview_theAgentCannotStamp(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	id := seedPlaybook(t, pg, jID, tID, "draft", "No agent stamps")
	if _, err := pg.MarkStatementsReviewed(context.Background(), id, nil, store.ActorDraftingAgent); err == nil {
		t.Fatal("the drafting agent marked a statement reviewed")
	}
}

func TestReview_directoryEntriesAreDoneLikeAnyStatement(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	src, err := pg.UpsertSource(ctx, store.UpsertSourceParams{URL: "https://help.example.org/" + t.Name(), Publisher: "Help", Kind: "nonprofit"})
	if err != nil {
		t.Fatal(err)
	}
	if err := pg.IngestPlaybook(ctx, store.IngestPlaybookParams{
		JurisdictionID: jID, TopicID: tID, Language: "en", Slug: "dir-" + t.Name(),
		Title: "Where to get help", IntroMD: "intro", Status: "draft", PageKind: "directory",
		UpdatedBy: store.ActorDraftingAgent,
		Statements: []store.IngestStatementParams{
			{BodyMD: "Help Org: call 555-0100.", Language: "en", Sources: []store.IngestCitationParams{{SourceID: src.ID, Quote: "verbatim", CheckedNow: true, CheckedBy: "x"}}},
			{BodyMD: "Open weekdays 9 to 5.", Language: "en", Sources: []store.IngestCitationParams{{SourceID: src.ID, Quote: "verbatim", CheckedNow: true, CheckedBy: "x"}}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := pg.Pool().QueryRow(ctx, `SELECT id FROM playbooks WHERE slug = $1 AND status = 'draft'`, "dir-"+t.Name()).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if got := reviewedAt(t, pg, id); got[0] || got[1] {
		t.Fatal("agent-saved directory entries must start unreviewed")
	}
	if err := pg.AuthorPublishPlaybook(ctx, id, "Nazanin"); err == nil {
		t.Fatal("a directory page must not publish with unread entries")
	}
	if _, err := pg.MarkStatementsReviewed(ctx, id, nil, "Nazanin"); err != nil {
		t.Fatal(err)
	}
	if err := pg.AuthorPublishPlaybook(ctx, id, "Nazanin"); err != nil {
		t.Fatalf("a directory page with every entry done publishes: %v", err)
	}
}

func TestReview_ingestingAsPublishedStampsEveryStatement(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	id := seedPlaybook(t, pg, jID, tID, "published", "Seeded live")
	if got := reviewedAt(t, pg, id); !got[0] {
		t.Fatal("a page ingested as published was reviewed as a page")
	}
}

func TestReview_draftCountsForTheDashboard(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	id := seedPlaybook(t, pg, jID, tID, "draft", "Counts")
	keys := statementKeys(t, pg, id)
	saveAs(t, pg, id, jID, tID, store.ActorDraftingAgent,
		store.IngestStatementParams{Key: keys[0], BodyMD: "One."},
		store.IngestStatementParams{BodyMD: "Two."},
		store.IngestStatementParams{BodyMD: "Three."})
	keys = statementKeys(t, pg, id)
	if _, err := pg.MarkStatementsReviewed(ctx, id, keys[:2], "Nazanin"); err != nil {
		t.Fatal(err)
	}
	counts, err := pg.AuthorDraftReviewCounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if counts[id] != (store.ReviewCount{Reviewed: 2, Total: 3}) {
		t.Errorf("counts = %+v", counts[id])
	}
}
