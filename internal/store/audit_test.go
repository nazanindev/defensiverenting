package store_test

import (
	"context"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// The audit lists the country and every state for a topic whether or not a
// page exists, and reads the topic's must-answer questions (topic_questions).
func TestAuditTopic(t *testing.T) {
	pg := testDB(t)
	a, err := pg.AuditTopic(context.Background(), "eviction-defense")
	if err != nil {
		t.Fatalf("AuditTopic: %v", err)
	}
	if len(a.Rows) == 0 {
		t.Fatal("want a row for the country and each state, pages or not")
	}
	if _, err := pg.AuditTopic(context.Background(), "no-such-topic"); err == nil {
		t.Fatal("want an error for an unknown topic")
	}
}

func TestAuditGaps(t *testing.T) {
	in := store.AuditInput{
		Core:       []store.AuditConcept{{Slug: "pay-and-stay"}, {Slug: "eviction-appeal"}, {Slug: "default-judgment"}},
		NoLaw:      map[string]bool{"eviction-appeal": true},
		Statements: []store.AuditStatement{{Key: "k", Concept: "pay-and-stay"}},
	}
	got := in.Gaps()
	if len(got) != 1 || got[0].Slug != "default-judgment" {
		t.Fatalf("want only default-judgment as a gap, got %v", got)
	}
}
