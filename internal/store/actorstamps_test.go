package store_test

import (
	"context"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// The portal stamps who last touched a page and who confirmed each citation.
// The subtle path is inheritance: a re-save that skips the live fetch (the
// quote is already known-verified) must carry forward the original checker's
// name with the original stamp, not credit the person re-saving.

func actorFixture(t *testing.T) (*store.PG, int64, int64, store.Source) {
	t.Helper()
	pg := testDB(t)
	ctx := context.Background()
	freshSlugs(t, pg, "actor-city-"+t.Name(), "actor-topic-"+t.Name())
	j, err := pg.UpsertJurisdiction(ctx, store.UpsertJurisdictionParams{
		Kind: "city", Name: "Actor City", Slug: "actor-city-" + t.Name(),
	})
	if err != nil {
		t.Fatal(err)
	}
	tp, err := pg.UpsertTopic(ctx, store.UpsertTopicParams{
		Slug: "actor-topic-" + t.Name(), Name: "Actor Topic",
	})
	if err != nil {
		t.Fatal(err)
	}
	src, err := pg.UpsertSource(ctx, store.UpsertSourceParams{
		URL: "https://example.gov/actor-" + t.Name(), Publisher: "Example", Kind: "statute",
	})
	if err != nil {
		t.Fatal(err)
	}
	return pg, j.ID, tp.ID, src
}

func TestActorStamps_saveEditAndPublish(t *testing.T) {
	pg, jID, tID, src := actorFixture(t)
	ctx := context.Background()

	// First save: a live-verified quote, saved by Nazanin.
	if err := pg.IngestPlaybook(ctx, store.IngestPlaybookParams{
		JurisdictionID: jID, TopicID: tID, Language: "en", Slug: "actor-topic-" + t.Name(),
		Title: "First save", IntroMD: "intro", Status: "draft", UpdatedBy: "Nazanin",
		Statements: []store.IngestStatementParams{{
			BodyMD: "A claim.", Language: "en",
			Sources: []store.IngestCitationParams{{
				SourceID: src.ID, Locator: "§ 1", Quote: "verbatim words",
				CheckedNow: true, CheckedBy: "Nazanin",
			}},
		}},
	}); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	var id int64
	if err := pg.Pool().QueryRow(ctx,
		`SELECT id FROM playbooks WHERE jurisdiction_id=$1 AND topic_id=$2 AND status='draft'`,
		jID, tID).Scan(&id); err != nil {
		t.Fatal(err)
	}

	pw, err := pg.AuthorGetPlaybook(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if pw.Playbook.UpdatedBy != "Nazanin" {
		t.Errorf("after first save updated_by = %q, want Nazanin", pw.Playbook.UpdatedBy)
	}
	cite := pw.Statements[0].Citations[0]
	if cite.CheckedAt == nil || cite.CheckedBy != "Nazanin" {
		t.Errorf("first save citation checked_at=%v checked_by=%q, want a stamp by Nazanin", cite.CheckedAt, cite.CheckedBy)
	}
	firstChecked := *cite.CheckedAt

	// Cameron re-saves the page with the same quote. The save skips the live
	// fetch (CheckedNow false), so the confirmation still belongs to Nazanin;
	// the edit belongs to Cameron.
	if err := pg.AuthorUpdatePlaybook(ctx, store.AuthorUpdatePlaybookParams{
		ID: id, JurisdictionID: jID, TopicID: tID, Language: "en",
		Slug: "actor-topic-" + t.Name(), Title: "Second save", IntroMD: "intro",
		UpdatedBy: "Cameron",
		Statements: []store.IngestStatementParams{{
			BodyMD: "A reworded claim.", Language: "en",
			Sources: []store.IngestCitationParams{{
				SourceID: src.ID, Locator: "§ 1", Quote: "verbatim words",
				CheckedNow: false, CheckedBy: "Cameron",
			}},
		}},
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	pw, err = pg.AuthorGetPlaybook(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if pw.Playbook.UpdatedBy != "Cameron" {
		t.Errorf("after edit updated_by = %q, want Cameron", pw.Playbook.UpdatedBy)
	}
	cite = pw.Statements[0].Citations[0]
	if cite.CheckedBy != "Nazanin" {
		t.Errorf("re-save without a fetch credited %q with the confirmation, want the inherited Nazanin", cite.CheckedBy)
	}
	if cite.CheckedAt == nil || !cite.CheckedAt.Equal(firstChecked) {
		t.Errorf("re-save without a fetch moved checked_at to %v, want the inherited %v", cite.CheckedAt, firstChecked)
	}

	// Publishing is a touch too.
	if err := pg.AuthorPublishPlaybook(ctx, id, "Cameron"); err != nil {
		t.Fatalf("publish: %v", err)
	}
	pw, err = pg.AuthorGetPlaybook(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if pw.Playbook.Status != "published" || pw.Playbook.UpdatedBy != "Cameron" {
		t.Errorf("after publish status=%q updated_by=%q, want published by Cameron", pw.Playbook.Status, pw.Playbook.UpdatedBy)
	}
}

func TestActorStamps_emptyQuoteNamesNobody(t *testing.T) {
	pg, jID, tID, src := actorFixture(t)
	ctx := context.Background()

	// An empty quote was never confirmed, so there is nobody to name even
	// though the saver is known — checked_by must stay empty with checked_at
	// NULL, or the pair would claim a confirmation that never happened.
	if err := pg.IngestPlaybook(ctx, store.IngestPlaybookParams{
		JurisdictionID: jID, TopicID: tID, Language: "en", Slug: "actor-topic-" + t.Name(),
		Title: "Empty quote", IntroMD: "intro", Status: "draft", UpdatedBy: "Nazanin",
		Statements: []store.IngestStatementParams{{
			BodyMD: "A claim.", Language: "en",
			Sources: []store.IngestCitationParams{{
				SourceID: src.ID, Locator: "§ 1", Quote: "",
				CheckedNow: true, CheckedBy: "Nazanin",
			}},
		}},
	}); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	var id int64
	if err := pg.Pool().QueryRow(ctx,
		`SELECT id FROM playbooks WHERE jurisdiction_id=$1 AND topic_id=$2 AND status='draft'`,
		jID, tID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	pw, err := pg.AuthorGetPlaybook(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	cite := pw.Statements[0].Citations[0]
	if cite.CheckedAt != nil || cite.CheckedBy != "" {
		t.Errorf("empty quote got checked_at=%v checked_by=%q, want no stamp and no name", cite.CheckedAt, cite.CheckedBy)
	}
}
