package main

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	dbpkg "github.com/nazanindev/defensiverenting/db"
	"github.com/nazanindev/defensiverenting/internal/store"
)

// Topic selection is no longer string manipulation over a hardcoded list; it is
// a lookup against the topics registry, so these run against a real database.
func testSrv(t *testing.T) *srv {
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
	return &srv{pg: pg, log: slog.New(slog.NewTextHandler(os.Stderr, nil))}
}

func resolveFromForm(t *testing.T, s *srv, topicKey string) (store.Topic, string) {
	t.Helper()
	form := url.Values{}
	form.Set("topic_key", topicKey)
	r := httptest.NewRequest("POST", "/new", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return s.resolveTopic(context.Background(), r)
}

func TestResolveTopic_registrySlugResolves(t *testing.T) {
	s := testSrv(t)
	topic, msg := resolveFromForm(t, s, "security-deposits")
	if msg != "" {
		t.Fatalf("unexpected rejection: %s", msg)
	}
	if topic.Slug != "security-deposits" {
		t.Errorf("slug = %q, want security-deposits", topic.Slug)
	}
	if topic.Name != "Security Deposits" {
		t.Errorf("name = %q, want the registry's name", topic.Name)
	}
}

// The 2026-08-01 incident: both form paths built the slug as
// citySlug + "-" + topicKey. A city-prefixed slug is not in the registry, so
// the shape is now unrepresentable rather than merely discouraged.
//
// This asserts the invariant over the canonical set rather than over whatever
// rows happen to exist. A database that still holds pre-cleanup topics (the
// dev copy does; prod was cleaned on 2026-08-01, and D7 step 4 retires the
// rest) would otherwise fail here for reasons that are not this code's doing.
func TestResolveTopic_cityPrefixedSlugIsNotInTheRegistry(t *testing.T) {
	s := testSrv(t)
	if topic, msg := resolveFromForm(t, s, "boston-security-deposits"); msg == "" {
		t.Errorf("city-prefixed slug was accepted as topic %d", topic.ID)
	}

	cities, err := s.pg.ListCityJurisdictions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	topics, err := s.pg.ListTopicRegistry(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, tp := range topics {
		if !tp.IsCore {
			continue // legacy rows are non-core and retired by the URL migration
		}
		for _, c := range cities {
			if strings.HasPrefix(tp.Slug, c.Slug+"-") {
				t.Errorf("core topic %q is prefixed with city %q", tp.Slug, c.Slug)
			}
		}
	}
}

func TestResolveTopic_unknownSlugRejected(t *testing.T) {
	s := testSrv(t)
	if _, msg := resolveFromForm(t, s, "not-a-real-topic"); msg == "" {
		t.Error("expected a rejection for a slug that is not in the registry")
	}
}

func TestResolveTopic_requiresASelection(t *testing.T) {
	s := testSrv(t)
	if _, msg := resolveFromForm(t, s, ""); msg == "" {
		t.Error("expected a message when no topic is selected")
	}
}

// The registry is the vocabulary the drafting agent and the form share, so the
// canonical set has to actually be there after migration.
func TestTopicRegistry_seededWithCanonicalSet(t *testing.T) {
	s := testSrv(t)
	topics, err := s.pg.ListTopicRegistry(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	byslug := map[string]store.Topic{}
	for _, tp := range topics {
		byslug[tp.Slug] = tp
	}
	core := []string{
		"cant-pay-rent", "eviction-defense", "repairs-and-habitability",
		"security-deposits", "landlord-entry", "rent-increase", "resource-directory",
	}
	for _, slug := range core {
		tp, ok := byslug[slug]
		if !ok {
			t.Errorf("core topic %q missing from the registry", slug)
			continue
		}
		if !tp.IsCore {
			t.Errorf("topic %q should be flagged is_core", slug)
		}
	}
	// heat-not-working is deliberately kept out of the core set: it stays as a
	// non-core topic so cold-weather cities keep the page and its URL.
	if tp, ok := byslug["heat-not-working"]; !ok {
		t.Error("heat-not-working missing from the registry")
	} else if tp.IsCore {
		t.Error("heat-not-working should be non-core")
	}
}
