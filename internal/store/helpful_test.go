package store_test

import (
	"context"
	"testing"
)

func TestHelpful_countsPublishedPagesOnly(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	live := seedPlaybook(t, pg, jID, tID, "published", "Live")
	draft := seedPlaybook(t, pg, jID, tID, "draft", "Draft")
	// The test database outlives a run, so compare against what was there.
	yes0, no0, err := pg.PageHelpful(ctx, jID, tID, "en")
	if err != nil {
		t.Fatal(err)
	}

	for _, answer := range []bool{true, true, false} {
		if ok, err := pg.RecordHelpful(ctx, live, answer); err != nil || !ok {
			t.Fatalf("live page answer not counted: ok=%v err=%v", ok, err)
		}
	}
	if ok, err := pg.RecordHelpful(ctx, draft, true); err != nil || ok {
		t.Fatalf("a draft was counted: ok=%v err=%v", ok, err)
	}
	if ok, err := pg.RecordHelpful(ctx, 999999, true); err != nil || ok {
		t.Fatalf("an unknown page was counted: ok=%v err=%v", ok, err)
	}

	yes, no, err := pg.PageHelpful(ctx, jID, tID, "en")
	if err != nil {
		t.Fatal(err)
	}
	if yes-yes0 != 2 || no-no0 != 1 {
		t.Fatalf("added %d yes, %d no; want 2 yes, 1 no", yes-yes0, no-no0)
	}
}
