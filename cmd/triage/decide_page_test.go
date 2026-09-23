package main

import (
	"strings"
	"testing"
	"time"

	"github.com/nazanindev/defensiverenting/internal/store"
)

func testPage(status string) store.PlaybookWithStatements {
	now := time.Now()
	pw := store.PlaybookWithStatements{Statements: []store.CitedStatement{
		{Key: "k1", BodyMD: "One.", ReviewedAt: &now},
		{Key: "k2", BodyMD: "Two.", ReviewedAt: &now},
	}}
	pw.Status = status
	return pw
}

func TestCheckFinding(t *testing.T) {
	pw := testPage("draft")
	ok := pageFinding{Kind: "duplicate", Keys: []string{"k1", "k2"}, Note: "Both say the same thing."}
	if why := checkFinding(pw, ok); why != "" {
		t.Errorf("a valid finding was refused: %s", why)
	}
	for name, f := range map[string]pageFinding{
		"unknown kind":    {Kind: "style", Keys: []string{"k1"}, Note: "x"},
		"no note":         {Kind: "order", Keys: []string{"k1"}},
		"no keys":         {Kind: "order", Note: "x"},
		"key not on page": {Kind: "order", Keys: []string{"k9"}, Note: "x"},
	} {
		if checkFinding(pw, f) == "" {
			t.Errorf("%s: want refused", name)
		}
	}
	if why := checkFinding(testPage("published"), ok); !strings.Contains(why, "draft") {
		t.Errorf("a published page must be refused, got %q", why)
	}
}

func TestPageReady(t *testing.T) {
	if !pageReady(testPage("draft")) {
		t.Error("all stamped, nothing open: ready")
	}
	pw := testPage("draft")
	pw.Statements[1].ReviewedAt = nil
	if pageReady(pw) {
		t.Error("an unstamped statement: not ready")
	}
	pw = testPage("draft")
	pw.Statements[0].Undecided = true
	if pageReady(pw) {
		t.Error("an open queue item: not ready")
	}
}
