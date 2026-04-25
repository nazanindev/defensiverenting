package store_test

import (
	"context"
	"os"
	"testing"

	dbpkg "github.com/nazanin212/bostontenantsrights/db"
	"github.com/nazanin212/bostontenantsrights/internal/store"
)

func testDB(t *testing.T) *store.PG {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}
	ctx := context.Background()
	pg, err := store.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pg.Close)

	if err := dbpkg.Migrate(ctx, pg.Pool()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return pg
}

func TestPing(t *testing.T) {
	pg := testDB(t)
	if err := pg.Ping(context.Background()); err != nil {
		t.Errorf("ping: %v", err)
	}
}

func TestUpsertJurisdiction(t *testing.T) {
	pg := testDB(t)
	ctx := context.Background()

	j, err := pg.UpsertJurisdiction(ctx, store.UpsertJurisdictionParams{
		Kind: "city",
		Name: "Test City",
		Slug: "test-city-" + t.Name(),
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if j.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if j.Slug == "" {
		t.Error("expected non-empty slug")
	}

	// Idempotency: second upsert returns same ID
	j2, err := pg.UpsertJurisdiction(ctx, store.UpsertJurisdictionParams{
		Kind: "city",
		Name: "Test City Updated",
		Slug: j.Slug,
	})
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if j2.ID != j.ID {
		t.Errorf("idempotency violated: ID changed from %d to %d", j.ID, j2.ID)
	}
}

func TestGetJurisdictionBySlug_notFound(t *testing.T) {
	pg := testDB(t)
	_, err := pg.GetJurisdictionBySlug(context.Background(), "does-not-exist-xyz")
	if err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestIngestPlaybook_citationEnforcement(t *testing.T) {
	pg := testDB(t)
	ctx := context.Background()

	j, _ := pg.UpsertJurisdiction(ctx, store.UpsertJurisdictionParams{
		Kind: "city", Name: "EnforcementCity", Slug: "enforcement-city-" + t.Name(),
	})
	topic, _ := pg.UpsertTopic(ctx, store.UpsertTopicParams{
		Slug: "enforcement-topic-" + t.Name(), Name: "Enforcement Topic",
	})

	err := pg.IngestPlaybook(ctx, store.IngestPlaybookParams{
		JurisdictionID: j.ID,
		TopicID:        topic.ID,
		Language:       "en",
		Slug:           "enforcement-" + t.Name(),
		Title:          "Enforcement Test",
		Statements: []store.IngestStatementParams{
			{BodyMD: "Statement without sources", Language: "en", Sources: nil},
		},
	})
	if err == nil {
		t.Fatal("expected error for statement without citations, got nil")
	}
}

func TestIngestPlaybook_roundTrip(t *testing.T) {
	pg := testDB(t)
	ctx := context.Background()

	j, err := pg.UpsertJurisdiction(ctx, store.UpsertJurisdictionParams{
		Kind: "city", Name: "Roundtrip City", Slug: "roundtrip-city-" + t.Name(),
	})
	if err != nil {
		t.Fatal(err)
	}
	topic, err := pg.UpsertTopic(ctx, store.UpsertTopicParams{
		Slug: "roundtrip-topic-" + t.Name(), Name: "Roundtrip Topic",
	})
	if err != nil {
		t.Fatal(err)
	}
	src, err := pg.UpsertSource(ctx, store.UpsertSourceParams{
		URL:       "https://example.com/law-" + t.Name(),
		Publisher: "Example Publisher",
		Kind:      "statute",
	})
	if err != nil {
		t.Fatal(err)
	}

	err = pg.IngestPlaybook(ctx, store.IngestPlaybookParams{
		JurisdictionID: j.ID,
		TopicID:        topic.ID,
		Language:       "en",
		Slug:           "roundtrip-" + t.Name(),
		Title:          "Roundtrip Test",
		IntroMD:        "An intro.",
		Statements: []store.IngestStatementParams{
			{
				BodyMD:   "A test statement.",
				Language: "en",
				Sources:  []store.IngestCitationParams{{SourceID: src.ID, Locator: "§ 1"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}

	pb, err := pg.GetPlaybook(ctx, j.Slug, topic.Slug, "en")
	if err != nil {
		t.Fatalf("get playbook: %v", err)
	}
	if len(pb.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(pb.Statements))
	}
	if len(pb.Statements[0].Citations) != 1 {
		t.Fatalf("expected 1 citation, got %d", len(pb.Statements[0].Citations))
	}
	if pb.Statements[0].Citations[0].Locator != "§ 1" {
		t.Errorf("locator = %q, want § 1", pb.Statements[0].Citations[0].Locator)
	}
}

func TestSearch_recursive(t *testing.T) {
	pg := testDB(t)
	ctx := context.Background()

	// Create parent (state) → child (city) jurisdiction
	state, _ := pg.UpsertJurisdiction(ctx, store.UpsertJurisdictionParams{
		Kind: "state", Name: "Search State", Slug: "search-state-" + t.Name(),
	})
	city, _ := pg.UpsertJurisdiction(ctx, store.UpsertJurisdictionParams{
		ParentID: &state.ID, Kind: "city", Name: "Search City", Slug: "search-city-" + t.Name(),
	})
	topic, _ := pg.UpsertTopic(ctx, store.UpsertTopicParams{
		Slug: "search-topic-" + t.Name(), Name: "Search Topic",
	})
	src, _ := pg.UpsertSource(ctx, store.UpsertSourceParams{
		URL: "https://example.com/search-" + t.Name(), Publisher: "Test", Kind: "statute",
	})

	// Ingest a playbook under the state (not the city)
	_ = pg.IngestPlaybook(ctx, store.IngestPlaybookParams{
		JurisdictionID: state.ID,
		TopicID:        topic.ID,
		Language:       "en",
		Slug:           "search-" + t.Name(),
		Title:          "searchableuniquexyz title",
		IntroMD:        "searchableuniquexyz intro.",
		Statements: []store.IngestStatementParams{
			{
				BodyMD:   "The searchableuniquexyz claim.",
				Language: "en",
				Sources:  []store.IngestCitationParams{{SourceID: src.ID}},
			},
		},
	})

	// Search scoped to the city — should find the state-level statement via ancestor expansion
	results, err := pg.Search(ctx, "searchableuniquexyz", &city.ID, "en")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected results via recursive jurisdiction expansion, got none")
	}
}
