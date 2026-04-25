package content_test

import (
	"strings"
	"testing"

	"github.com/nazanin212/bostontenantsrights/internal/content"
)

var validFile = []byte(`---
jurisdiction: boston
topic: heat-not-working
language: en
title: "Heat Not Working"
intro: "A livable home is your right."
sources:
  - id: ma-105cmr410
    url: "https://www.mass.gov/regulations/105-CMR-41000"
    publisher: "Massachusetts DPH"
    jurisdiction: massachusetts
    kind: regulation
    locator: "§ 410.200"
  - id: boston-311
    url: "https://www.boston.gov/departments/boston-311"
    publisher: "City of Boston"
    jurisdiction: boston
    kind: gov_guidance
---

Massachusetts landlords must provide heat of at least 68°F during the heating season. [ma-105cmr410]

Tenants can report a heat outage to Boston 311 to trigger a housing inspection. [boston-311]
`)

func TestParse_valid(t *testing.T) {
	pb, err := content.Parse(validFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pb.Topic != "heat-not-working" {
		t.Errorf("topic = %q, want heat-not-working", pb.Topic)
	}
	if len(pb.Statements) != 2 {
		t.Fatalf("statements = %d, want 2", len(pb.Statements))
	}
	if len(pb.Statements[0].Citations) != 1 {
		t.Errorf("statement[0] citations = %d, want 1", len(pb.Statements[0].Citations))
	}
	if pb.Statements[0].Citations[0].SourceID != "ma-105cmr410" {
		t.Errorf("citation source = %q, want ma-105cmr410", pb.Statements[0].Citations[0].SourceID)
	}
	// Body must not contain the citation token
	if strings.Contains(pb.Statements[0].Body, "[ma-105cmr410]") {
		t.Error("citation token should be stripped from body")
	}
}

func TestParse_missingCitation(t *testing.T) {
	bad := []byte(`---
jurisdiction: boston
topic: heat-not-working
language: en
title: "Heat Not Working"
intro: ""
sources:
  - id: ma-105cmr410
    url: "https://example.com"
    publisher: "MA DPH"
    jurisdiction: massachusetts
    kind: regulation
---

This paragraph has no citation reference at the end.
`)
	_, err := content.Parse(bad)
	if err == nil {
		t.Fatal("expected error for missing citation, got nil")
	}
	if !strings.Contains(err.Error(), "no citation reference") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestParse_missingFrontmatterFields(t *testing.T) {
	bad := []byte(`---
language: en
---

Some text. [editorial]
`)
	_, err := content.Parse(bad)
	if err == nil {
		t.Fatal("expected error for missing fields")
	}
}

func TestParse_editorialImplicit(t *testing.T) {
	src := []byte(`---
jurisdiction: boston
topic: heat-not-working
language: en
title: "Heat Not Working"
intro: ""
sources:
  - id: ma-105cmr410
    url: "https://example.com"
    publisher: "MA DPH"
    jurisdiction: massachusetts
    kind: regulation
---

A statute-backed claim. [ma-105cmr410]

Editorial guidance not derived from a single statute. [editorial]
`)
	pb, err := content.Parse(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pb.Statements) != 2 {
		t.Fatalf("statements = %d, want 2", len(pb.Statements))
	}
	if pb.Statements[1].Citations[0].SourceID != "editorial" {
		t.Errorf("expected editorial citation, got %q", pb.Statements[1].Citations[0].SourceID)
	}
}

func TestParse_locatorOverride(t *testing.T) {
	src := []byte(`---
jurisdiction: boston
topic: heat-not-working
language: en
title: "Heat Not Working"
intro: ""
sources:
  - id: ma-mgl-c186
    url: "https://example.com"
    publisher: "MA Legislature"
    jurisdiction: massachusetts
    kind: statute
---

Tenants at will may terminate tenancy with 30 days notice. [ma-mgl-c186:§12]
`)
	pb, err := content.Parse(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cite := pb.Statements[0].Citations[0]
	if cite.Locator != "§12" {
		t.Errorf("locator = %q, want §12", cite.Locator)
	}
}
