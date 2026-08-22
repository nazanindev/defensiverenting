package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// A page that is wrong in front of renters has to come down before a correction
// can be researched. These tests cover taking it down without losing anything.

func TestUnpublish_makesALivePageTheDraftAgain(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	id := seedPlaybook(t, pg, jID, tID, "published", "Live")

	if err := pg.AuthorUnpublishPlaybook(ctx, id, false, "test"); err != nil {
		t.Fatalf("unpublish: %v", err)
	}

	var status string
	if err := pg.Pool().QueryRow(ctx, `SELECT status FROM playbooks WHERE id = $1`, id).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "draft" {
		t.Fatalf("want the page to become a draft, got %q", status)
	}
}

func TestUnpublish_keepsTheStatementsAndCitations(t *testing.T) {
	// The point of taking a page down rather than deleting it is that the text
	// survives to be corrected.
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	id := seedPlaybook(t, pg, jID, tID, "published", "Live")

	var before int
	if err := pg.Pool().QueryRow(ctx,
		`SELECT count(*) FROM playbook_statements WHERE playbook_id = $1`, id).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err := pg.AuthorUnpublishPlaybook(ctx, id, false, "test"); err != nil {
		t.Fatalf("unpublish: %v", err)
	}
	var after int
	if err := pg.Pool().QueryRow(ctx,
		`SELECT count(*) FROM playbook_statements WHERE playbook_id = $1`, id).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before == 0 || after != before {
		t.Fatalf("statements must survive: had %d, now %d", before, after)
	}
}

func TestUnpublish_asksBeforeDisplacingAWaitingDraft(t *testing.T) {
	// A slot holds one draft, so the waiting one has to move aside. That is the
	// reviewer's call, so the first attempt reports the situation and changes
	// nothing rather than deciding for them.
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	pubID := seedPlaybook(t, pg, jID, tID, "published", "Live")
	draftID := seedPlaybook(t, pg, jID, tID, "draft", "Proposed revision")

	err := pg.AuthorUnpublishPlaybook(ctx, pubID, false, "test")
	if !errors.Is(err, store.ErrDraftExists) {
		t.Fatalf("want ErrDraftExists, got %v", err)
	}

	for _, tc := range []struct {
		id   int64
		want string
	}{{pubID, "published"}, {draftID, "draft"}} {
		var status string
		if err := pg.Pool().QueryRow(ctx, `SELECT status FROM playbooks WHERE id = $1`, tc.id).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if status != tc.want {
			t.Errorf("playbook %d should still be %q, got %q", tc.id, tc.want, status)
		}
	}
}

func TestUnpublish_retiresTheWaitingDraftWhenConfirmed(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	pubID := seedPlaybook(t, pg, jID, tID, "published", "Live")
	draftID := seedPlaybook(t, pg, jID, tID, "draft", "Proposed revision")

	if err := pg.AuthorUnpublishPlaybook(ctx, pubID, true, "test"); err != nil {
		t.Fatalf("confirmed unpublish: %v", err)
	}

	var wasPublished, wasDraft string
	if err := pg.Pool().QueryRow(ctx, `SELECT status FROM playbooks WHERE id = $1`, pubID).Scan(&wasPublished); err != nil {
		t.Fatal(err)
	}
	if err := pg.Pool().QueryRow(ctx, `SELECT status FROM playbooks WHERE id = $1`, draftID).Scan(&wasDraft); err != nil {
		t.Fatal(err)
	}
	if wasPublished != "draft" {
		t.Errorf("the live page should become the draft, got %q", wasPublished)
	}
	if wasDraft != "superseded" {
		t.Errorf("the waiting draft should be retired, got %q", wasDraft)
	}
}

func TestUnpublish_retiredDraftKeepsItsContent(t *testing.T) {
	// Retired is not deleted. The whole reason for superseding rather than
	// removing is that the text stays readable and its sources reusable.
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	pubID := seedPlaybook(t, pg, jID, tID, "published", "Live")
	draftID := seedPlaybook(t, pg, jID, tID, "draft", "Proposed revision")

	var before int
	if err := pg.Pool().QueryRow(ctx,
		`SELECT count(*) FROM playbook_statements WHERE playbook_id = $1`, draftID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err := pg.AuthorUnpublishPlaybook(ctx, pubID, true, "test"); err != nil {
		t.Fatal(err)
	}
	var after, cites int
	if err := pg.Pool().QueryRow(ctx,
		`SELECT count(*) FROM playbook_statements WHERE playbook_id = $1`, draftID).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if err := pg.Pool().QueryRow(ctx, `
		SELECT count(*) FROM citations c
		JOIN playbook_statements ps ON ps.statement_id = c.statement_id
		WHERE ps.playbook_id = $1`, draftID).Scan(&cites); err != nil {
		t.Fatal(err)
	}
	if before == 0 || after != before {
		t.Fatalf("retired draft must keep its statements: had %d, now %d", before, after)
	}
	if cites == 0 {
		t.Error("retired draft must keep its citations")
	}
}

func TestUnpublish_refusesAPageThatIsNotLive(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	id := seedPlaybook(t, pg, jID, tID, "draft", "Not live")

	if err := pg.AuthorUnpublishPlaybook(context.Background(), id, false, "test"); !errors.Is(err, store.ErrNotPublished) {
		t.Fatalf("want ErrNotPublished, got %v", err)
	}
}

func TestUnpublish_reportsAMissingPage(t *testing.T) {
	pg, _, _ := revisionFixture(t)
	if err := pg.AuthorUnpublishPlaybook(context.Background(), 987654321, false, "test"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestUnpublish_thenPublishAgainRoundTrips(t *testing.T) {
	// Taking a page down and putting it back is the whole point: the slot must
	// end up exactly as it started, with one published page and no draft.
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	id := seedPlaybook(t, pg, jID, tID, "published", "Live")

	if err := pg.AuthorUnpublishPlaybook(ctx, id, false, "test"); err != nil {
		t.Fatalf("unpublish: %v", err)
	}
	if err := pg.AuthorPublishPlaybook(ctx, id, "test"); err != nil {
		t.Fatalf("republish: %v", err)
	}

	var published, drafts int
	if err := pg.Pool().QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE status = 'published'), count(*) FILTER (WHERE status = 'draft')
		FROM playbooks WHERE jurisdiction_id = $1 AND topic_id = $2 AND language = 'en'`,
		jID, tID).Scan(&published, &drafts); err != nil {
		t.Fatal(err)
	}
	if published != 1 || drafts != 0 {
		t.Fatalf("want 1 published and 0 drafts, got %d and %d", published, drafts)
	}
}
