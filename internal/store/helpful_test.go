package store_test

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// newClient is an address no earlier run used: the test database keeps
// today's answers between runs.
func newClient(t *testing.T) string {
	t.Helper()
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return "test-" + hex.EncodeToString(b)
}

func TestHelpful_oncePerReaderPerPageAndOnlyLivePages(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	live := seedPlaybook(t, pg, jID, tID, "published", "Live")
	draft := seedPlaybook(t, pg, jID, tID, "draft", "Draft")
	// The test database outlives a run, so compare against what was there.
	yes0, no0, err := pg.PageHelpful(ctx, jID, tID, "en")
	if err != nil {
		t.Fatal(err)
	}

	a, b := newClient(t), newClient(t)
	cases := []struct {
		name    string
		page    int64
		helpful bool
		client  string
		want    bool
	}{
		{"first answer", live, true, a, true},
		{"same reader again", live, false, a, false},
		{"another reader", live, false, b, true},
		{"a draft", draft, true, newClient(t), false},
		{"an unknown page", 999999, true, newClient(t), false},
		{"no address", live, true, "", false},
	}
	for _, c := range cases {
		got, err := pg.RecordHelpful(ctx, c.page, c.helpful, c.client)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got != c.want {
			t.Errorf("%s: counted = %v, want %v", c.name, got, c.want)
		}
	}

	yes, no, err := pg.PageHelpful(ctx, jID, tID, "en")
	if err != nil {
		t.Fatal(err)
	}
	if yes-yes0 != 1 || no-no0 != 1 {
		t.Fatalf("added %d yes, %d no; want 1 yes, 1 no", yes-yes0, no-no0)
	}
}

func TestHelpful_dailyCapPerReader(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	live := seedPlaybook(t, pg, jID, tID, "published", "Live")
	c := newClient(t)

	// Make today's salt exist, then fill this reader's day with answers on
	// other pages, the way the store would have recorded them.
	if _, err := pg.RecordHelpful(ctx, 999999, true, c); err != nil {
		t.Fatal(err)
	}
	var salt []byte
	if err := pg.Pool().QueryRow(ctx, `SELECT salt FROM helpful_salt WHERE day = CURRENT_DATE`).Scan(&salt); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(append(salt, c...))
	for i := 0; i < store.HelpfulDailyCap; i++ {
		if _, err := pg.Pool().Exec(ctx, `
			INSERT INTO helpful_seen (day, reader, jurisdiction_id, topic_id, language)
			VALUES (CURRENT_DATE, $1, $2, 0, 'en')`, sum[:], -1-int64(i)); err != nil {
			t.Fatal(err)
		}
	}

	if got, err := pg.RecordHelpful(ctx, live, true, c); err != nil || got {
		t.Fatalf("an answer over the daily cap was counted: got=%v err=%v", got, err)
	}
	if got, err := pg.RecordHelpful(ctx, live, true, newClient(t)); err != nil || !got {
		t.Fatalf("another reader was blocked by the cap: got=%v err=%v", got, err)
	}
}
