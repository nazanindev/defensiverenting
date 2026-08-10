package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// freshSlugs clears any rows left by an earlier run and removes this run's rows
// afterwards. Unlike the upsert-based tests, rename tests mint a new slug every
// time, so without this the second run collides with the first run's leftovers.
// slug_aliases rows go with their target via ON DELETE CASCADE.
func freshSlugs(t *testing.T, pg *store.PG, prefixes ...string) {
	t.Helper()
	// Content first, then the rows it points at. Deleting a jurisdiction that
	// still has playbooks fails on the foreign key, and because these deletes
	// are best-effort the failure was silent: the fixture survived, the next
	// run's UpsertJurisdiction returned the same row, and the test inherited
	// the previous run's playbooks. That is invisible until a test cares what
	// else is in the slot — one draft per slot, for instance.
	purge := func() {
		ctx := context.Background()
		for _, p := range prefixes {
			like := p + "%"
			_, _ = pg.Pool().Exec(ctx, `
				DELETE FROM playbooks WHERE jurisdiction_id IN (SELECT id FROM jurisdictions WHERE slug LIKE $1)
				   OR topic_id IN (SELECT id FROM topics WHERE slug LIKE $1)`, like)
			_, _ = pg.Pool().Exec(ctx, `
				DELETE FROM statements WHERE jurisdiction_id IN (SELECT id FROM jurisdictions WHERE slug LIKE $1)`, like)
			_, _ = pg.Pool().Exec(ctx, `DELETE FROM jurisdictions WHERE slug LIKE $1`, like)
			_, _ = pg.Pool().Exec(ctx, `DELETE FROM topics WHERE slug LIKE $1`, like)
		}
	}
	purge()
	t.Cleanup(purge)
}

func TestRenameJurisdiction_oldSlugResolves(t *testing.T) {
	pg := testDB(t)
	ctx := context.Background()

	freshSlugs(t, pg, "alias-city-"+t.Name())
	old := "alias-city-" + t.Name()
	j, err := pg.UpsertJurisdiction(ctx, store.UpsertJurisdictionParams{
		Kind: "city", Name: "Alias City", Slug: old,
	})
	if err != nil {
		t.Fatal(err)
	}
	renamed := old + "-renamed"
	if err := pg.RenameJurisdiction(ctx, old, renamed); err != nil {
		t.Fatalf("rename: %v", err)
	}

	// The new slug is live.
	if got, err := pg.GetJurisdictionBySlug(ctx, renamed); err != nil || got.ID != j.ID {
		t.Fatalf("new slug lookup = (%+v, %v), want id %d", got, err, j.ID)
	}
	// The old slug is gone from the live namespace but resolves via alias.
	if _, err := pg.GetJurisdictionBySlug(ctx, old); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("old slug should no longer be live, got err %v", err)
	}
	got, err := pg.ResolveJurisdictionAlias(ctx, old)
	if err != nil {
		t.Fatalf("resolve alias: %v", err)
	}
	if got.ID != j.ID || got.Slug != renamed {
		t.Errorf("alias resolved to (id=%d slug=%q), want (id=%d slug=%q)", got.ID, got.Slug, j.ID, renamed)
	}
}

// Renaming twice must leave the very first URL resolving to the current row,
// not to the intermediate slug — otherwise the oldest links break silently.
func TestRenameJurisdiction_chainResolvesToCurrent(t *testing.T) {
	pg := testDB(t)
	ctx := context.Background()

	freshSlugs(t, pg, "chain-city-"+t.Name())
	first := "chain-city-" + t.Name()
	j, err := pg.UpsertJurisdiction(ctx, store.UpsertJurisdictionParams{
		Kind: "city", Name: "Chain City", Slug: first,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, third := first+"-2", first+"-3"
	if err := pg.RenameJurisdiction(ctx, first, second); err != nil {
		t.Fatal(err)
	}
	if err := pg.RenameJurisdiction(ctx, second, third); err != nil {
		t.Fatal(err)
	}

	for _, alias := range []string{first, second} {
		got, err := pg.ResolveJurisdictionAlias(ctx, alias)
		if err != nil {
			t.Fatalf("resolve %q: %v", alias, err)
		}
		if got.ID != j.ID || got.Slug != third {
			t.Errorf("%q resolved to slug %q, want %q", alias, got.Slug, third)
		}
	}
}

func TestAddJurisdictionAlias_refusesToShadowLiveSlug(t *testing.T) {
	pg := testDB(t)
	ctx := context.Background()

	freshSlugs(t, pg, "shadow-city-"+t.Name(), "other-city-"+t.Name())
	live, err := pg.UpsertJurisdiction(ctx, store.UpsertJurisdictionParams{
		Kind: "city", Name: "Shadow City", Slug: "shadow-city-" + t.Name(),
	})
	if err != nil {
		t.Fatal(err)
	}
	other, err := pg.UpsertJurisdiction(ctx, store.UpsertJurisdictionParams{
		Kind: "city", Name: "Other City", Slug: "other-city-" + t.Name(),
	})
	if err != nil {
		t.Fatal(err)
	}
	err = pg.AddJurisdictionAlias(ctx, live.Slug, other.ID)
	if !errors.Is(err, store.ErrAliasShadowsLiveSlug) {
		t.Fatalf("err = %v, want ErrAliasShadowsLiveSlug", err)
	}
	// The live row must be untouched by the refused alias.
	if got, err := pg.GetJurisdictionBySlug(ctx, live.Slug); err != nil || got.ID != live.ID {
		t.Errorf("live slug now resolves to (%+v, %v), want id %d", got, err, live.ID)
	}
}

// Reusing a retired slug as a live one again must drop the stale alias, or the
// alias would shadow the row now legitimately holding that slug.
func TestRenameJurisdiction_reusingARetiredSlugClearsItsAlias(t *testing.T) {
	pg := testDB(t)
	ctx := context.Background()

	freshSlugs(t, pg, "reuse-city-"+t.Name())
	a := "reuse-city-" + t.Name()
	b := a + "-b"
	j, err := pg.UpsertJurisdiction(ctx, store.UpsertJurisdictionParams{
		Kind: "city", Name: "Reuse City", Slug: a,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := pg.RenameJurisdiction(ctx, a, b); err != nil { // a is now an alias
		t.Fatal(err)
	}
	if err := pg.RenameJurisdiction(ctx, b, a); err != nil { // a is live again
		t.Fatal(err)
	}
	if _, err := pg.ResolveJurisdictionAlias(ctx, a); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("alias for %q should have been cleared when it went live again, got %v", a, err)
	}
	if got, err := pg.GetJurisdictionBySlug(ctx, a); err != nil || got.ID != j.ID {
		t.Errorf("live lookup = (%+v, %v), want id %d", got, err, j.ID)
	}
}

func TestRenameTopic_oldSlugResolves(t *testing.T) {
	pg := testDB(t)
	ctx := context.Background()

	freshSlugs(t, pg, "alias-topic-"+t.Name())
	old := "alias-topic-" + t.Name()
	tp, err := pg.UpsertTopic(ctx, store.UpsertTopicParams{Slug: old, Name: "Alias Topic"})
	if err != nil {
		t.Fatal(err)
	}
	renamed := old + "-renamed"
	if err := pg.RenameTopic(ctx, old, renamed); err != nil {
		t.Fatalf("rename: %v", err)
	}
	got, err := pg.ResolveTopicAlias(ctx, old)
	if err != nil {
		t.Fatalf("resolve alias: %v", err)
	}
	if got.ID != tp.ID || got.Slug != renamed {
		t.Errorf("alias resolved to (id=%d slug=%q), want (id=%d slug=%q)", got.ID, got.Slug, tp.ID, renamed)
	}
}

func TestRenameJurisdiction_unknownSlug(t *testing.T) {
	pg := testDB(t)
	err := pg.RenameJurisdiction(context.Background(), "no-such-slug-"+t.Name(), "whatever")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}
