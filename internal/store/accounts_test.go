package store_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// A link signs in once and creates the user on first use; the session then
// resolves to that user, carries the location, and dies with sign-out.
func TestAccounts_linkSessionLocation(t *testing.T) {
	pg := testDB(t)
	ctx := context.Background()
	now := time.Now()
	// The test database persists between runs, so every key carries the clock.
	run := fmt.Sprintf("%s-%d", t.Name(), now.UnixNano())
	email := "accounts-" + run + "@example.org"
	hash := "tok-" + run

	if err := pg.CreateLoginToken(ctx, hash, email, now.Add(time.Minute)); err != nil {
		t.Fatalf("create token: %v", err)
	}
	if n, _ := pg.CountRecentLoginTokens(ctx, email, now.Add(-time.Hour)); n != 1 {
		t.Errorf("recent tokens = %d, want 1", n)
	}
	userID, err := pg.ConsumeLoginToken(ctx, hash, now)
	if err != nil {
		t.Fatalf("consume: %v", err)
	}
	if _, err := pg.ConsumeLoginToken(ctx, hash, now); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("second consume err = %v, want ErrNotFound", err)
	}

	// An expired link never signs in, even unused.
	if err := pg.CreateLoginToken(ctx, hash+"-old", email, now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := pg.ConsumeLoginToken(ctx, hash+"-old", now); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expired consume err = %v, want ErrNotFound", err)
	}

	// Same address again resolves to the same user.
	if err := pg.CreateLoginToken(ctx, hash+"-2", email, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if again, _ := pg.ConsumeLoginToken(ctx, hash+"-2", now); again != userID {
		t.Errorf("second sign-in user = %d, want %d", again, userID)
	}

	sess := "sess-" + run
	if err := pg.CreateSession(ctx, sess, userID, now.Add(time.Hour)); err != nil {
		t.Fatalf("create session: %v", err)
	}
	a, err := pg.GetAccountBySession(ctx, sess, now)
	if err != nil || a.Email != email || a.Location.Slug != "" {
		t.Fatalf("account = %+v, %v", a, err)
	}

	j, err := pg.UpsertJurisdiction(ctx, store.UpsertJurisdictionParams{Kind: "city", Name: "Acct City", Slug: "acct-city-" + run})
	if err != nil {
		t.Fatal(err)
	}
	if err := pg.SetUserLocation(ctx, userID, "no-such-place-"+run); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("unknown slug err = %v, want ErrNotFound", err)
	}
	if err := pg.SetUserLocation(ctx, userID, j.Slug); err != nil {
		t.Fatalf("set location: %v", err)
	}
	if a, _ = pg.GetAccountBySession(ctx, sess, now); a.Location.Slug != j.Slug {
		t.Errorf("location = %q, want %q", a.Location.Slug, j.Slug)
	}
	if err := pg.SetUserLocation(ctx, userID, ""); err != nil {
		t.Fatalf("clear location: %v", err)
	}
	if a, _ = pg.GetAccountBySession(ctx, sess, now); a.Location.Slug != "" {
		t.Errorf("location after clear = %q", a.Location.Slug)
	}

	if _, err := pg.GetAccountBySession(ctx, sess, now.Add(2*time.Hour)); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expired session err = %v", err)
	}
	if err := pg.DeleteSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	if _, err := pg.GetAccountBySession(ctx, sess, now); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("deleted session err = %v", err)
	}
}
