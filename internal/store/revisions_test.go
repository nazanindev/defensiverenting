package store_test

import (
	"context"
	"strings"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// A research pass over an already-published page writes a proposed revision
// beside it. The live page must stay live and unchanged throughout, because it
// is serving renters the whole time.

// seedPlaybook writes a playbook with one cited statement and returns its id.
func seedPlaybook(t *testing.T, pg *store.PG, jID, tID int64, status, title string) int64 {
	t.Helper()
	ctx := context.Background()
	src, err := pg.UpsertSource(ctx, store.UpsertSourceParams{
		URL: "https://example.gov/law-" + t.Name(), Publisher: "Example", Kind: "statute",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := pg.IngestPlaybook(ctx, store.IngestPlaybookParams{
		JurisdictionID: jID, TopicID: tID, Language: "en", Slug: "slug-" + t.Name(),
		Title: title, IntroMD: "intro", Status: status,
		Statements: []store.IngestStatementParams{{
			BodyMD: "A claim. " + title, Language: "en",
			Sources: []store.IngestCitationParams{{SourceID: src.ID, Locator: "§ 1", Quote: "verbatim"}},
		}},
	}); err != nil {
		t.Fatalf("ingest %s: %v", status, err)
	}
	var id int64
	if err := pg.Pool().QueryRow(ctx,
		`SELECT id FROM playbooks WHERE jurisdiction_id=$1 AND topic_id=$2 AND language='en' AND status=$3`,
		jID, tID, status).Scan(&id); err != nil {
		t.Fatalf("find %s playbook: %v", status, err)
	}
	return id
}

func revisionFixture(t *testing.T) (*store.PG, int64, int64) {
	t.Helper()
	pg := testDB(t)
	ctx := context.Background()
	freshSlugs(t, pg, "rev-city-"+t.Name(), "rev-topic-"+t.Name())
	j, err := pg.UpsertJurisdiction(ctx, store.UpsertJurisdictionParams{
		Kind: "city", Name: "Rev City", Slug: "rev-city-" + t.Name(),
	})
	if err != nil {
		t.Fatal(err)
	}
	tp, err := pg.UpsertTopic(ctx, store.UpsertTopicParams{
		Slug: "rev-topic-" + t.Name(), Name: "Rev Topic",
	})
	if err != nil {
		t.Fatal(err)
	}
	return pg, j.ID, tp.ID
}

func TestRevision_draftCoexistsWithLivePage(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()

	live := seedPlaybook(t, pg, jID, tID, "published", "Live version")
	draft := seedPlaybook(t, pg, jID, tID, "draft", "Proposed revision")

	if live == draft {
		t.Fatal("the revision overwrote the live page instead of sitting beside it")
	}
	// The live page must still be the one served.
	got, err := pg.GetPlaybook(ctx, "rev-city-"+t.Name(), "rev-topic-"+t.Name(), "en")
	if err != nil {
		t.Fatalf("live page no longer served: %v", err)
	}
	if got.Playbook.Title != "Live version" {
		t.Errorf("served title = %q, want the live version", got.Playbook.Title)
	}
}

func TestRevision_publishingSwapsAndRetainsTheOldPage(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()

	live := seedPlaybook(t, pg, jID, tID, "published", "Live version")
	draft := seedPlaybook(t, pg, jID, tID, "draft", "Proposed revision")

	if err := pg.AuthorPublishPlaybook(ctx, draft); err != nil {
		t.Fatalf("publish revision: %v", err)
	}

	got, err := pg.GetPlaybook(ctx, "rev-city-"+t.Name(), "rev-topic-"+t.Name(), "en")
	if err != nil {
		t.Fatalf("no page served after the swap: %v", err)
	}
	if got.Playbook.ID != draft {
		t.Errorf("served playbook id = %d, want the revision %d", got.Playbook.ID, draft)
	}
	if got.Playbook.LastReviewedAt == nil {
		t.Error("publishing must stamp last_reviewed_at")
	}

	// The replaced page is retired, not deleted: it is the only record of what
	// the page used to say.
	var status string
	if err := pg.Pool().QueryRow(ctx, `SELECT status FROM playbooks WHERE id=$1`, live).Scan(&status); err != nil {
		t.Fatalf("the replaced page was deleted, losing its content: %v", err)
	}
	if status != "superseded" {
		t.Errorf("replaced page status = %q, want superseded", status)
	}
}

// Only one page may be live per slot. Without this the public site would have
// two pages competing for one URL.
func TestRevision_onlyOnePublishedPerSlot(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()

	seedPlaybook(t, pg, jID, tID, "published", "Live version")
	_, err := pg.Pool().Exec(ctx, `
		INSERT INTO playbooks (jurisdiction_id, topic_id, language, slug, title, intro_md, status)
		VALUES ($1, $2, 'en', 'dupe', 'Second live page', '', 'published')`, jID, tID)
	if err == nil {
		t.Fatal("a second published page was accepted for the same slot")
	}
}

// Re-drafting twice updates the revision rather than stacking them up.
func TestRevision_redraftingUpdatesTheSameDraft(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()

	seedPlaybook(t, pg, jID, tID, "published", "Live version")
	first := seedPlaybook(t, pg, jID, tID, "draft", "First attempt")
	second := seedPlaybook(t, pg, jID, tID, "draft", "Second attempt")

	if first != second {
		t.Errorf("re-drafting created a new row (%d then %d); it should update the existing revision", first, second)
	}
	var count int
	if err := pg.Pool().QueryRow(ctx,
		`SELECT count(*) FROM playbooks WHERE jurisdiction_id=$1 AND topic_id=$2 AND status='draft'`,
		jID, tID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("draft rows = %d, want 1", count)
	}
}

// The dashboard has to distinguish a revision from a brand-new page, because
// publishing one replaces something live and the other does not.
func TestRevision_dashboardFlagsRevisions(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()

	seedPlaybook(t, pg, jID, tID, "published", "Live version")
	draft := seedPlaybook(t, pg, jID, tID, "draft", "Proposed revision")

	rows, err := pg.AuthorListPlaybooks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, r := range rows {
		if r.ID == draft {
			found = true
			if !r.RevisesPublished {
				t.Error("draft over a live page should be flagged as a revision")
			}
		}
	}
	if !found {
		t.Fatal("the revision is missing from the dashboard list")
	}
}

// Search must never surface a page that is not live. Statements and playbooks
// are indexed for full-text search regardless of status, so without an explicit
// filter the text of unreviewed AI drafts appears in public search results —
// which it did in production until 2026-08-09.
func TestSearch_excludesDraftsAndSupersededPages(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()

	seedPlaybook(t, pg, jID, tID, "published", "Live version zzqx")
	seedPlaybook(t, pg, jID, tID, "draft", "Unreviewed revision zzqx")

	results, err := pg.Search(ctx, "zzqx", nil, "en")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if strings.Contains(r.PlaybookTitle, "Unreviewed") || strings.Contains(r.Snippet, "Unreviewed") {
			t.Fatalf("search returned an unpublished page: %q / %q", r.PlaybookTitle, r.Snippet)
		}
	}
	if len(results) == 0 {
		t.Error("the published page should still be findable")
	}
}
