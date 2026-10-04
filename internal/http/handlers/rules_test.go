package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/nazanindev/defensiverenting/internal/http/handlers"
	"github.com/nazanindev/defensiverenting/internal/store"
)

func cited(id int64, key, body, stage string) store.CitedStatement {
	return store.CitedStatement{
		ID: id, Key: key, BodyMD: body, Stage: stage,
		Citations: []store.CitationWithSource{{SourceID: 1, SourceURL: "https://example.gov/law", Publisher: "Ohio", SourceKind: "statute"}},
	}
}

func serveOK(t *testing.T, stub *stubStore, path string) string {
	t.Helper()
	r := chi.NewRouter()
	handlers.Browse(r, stub, logger())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d", path, rec.Code)
	}
	return rec.Body.String()
}

// Stage headings render where the stage changes, numbering runs on, and
// every statement carries its key as a stable link (ADR-028 D10, D11).
func TestPlaybook_stageHeadingsAndKeyAnchors(t *testing.T) {
	ohio := store.Jurisdiction{ID: 1, Kind: "state", Name: "Ohio", Slug: "ohio"}
	stub := &stubStore{
		jurisdictions: []store.Jurisdiction{ohio},
		topics:        []store.Topic{{ID: 1, Slug: "security-deposits", Name: "Security Deposits"}},
		playbook: store.PlaybookWithStatements{
			Playbook:     store.Playbook{ID: 1, Title: "Deposit not returned in Ohio", Slug: "security-deposits", Language: "en", PageKind: "playbook"},
			Jurisdiction: ohio,
			Topic:        store.Topic{Slug: "security-deposits", Name: "Security Deposits"},
			Statements: []store.CitedStatement{
				cited(1, "aaaaaaaa-0000-4000-8000-000000000001", "Ohio law sets a 30 day deadline.", "What the law says"),
				cited(2, "aaaaaaaa-0000-4000-8000-000000000002", "Ohio law lets you sue.", "What the law says"),
				cited(3, "aaaaaaaa-0000-4000-8000-000000000003", "Write to your landlord in Ohio.", "Ask for your deposit back"),
			},
		},
	}
	body := serveOK(t, stub, "/j/ohio/security-deposits")
	if strings.Count(body, `class="stage-heading"`) != 2 {
		t.Errorf("want 2 stage headings:\n%s", body)
	}
	if !strings.Contains(body, `<ol class="statement-list" start="3">`) {
		t.Error("numbering does not run on under the second heading")
	}
	if !strings.Contains(body, `id="s-aaaaaaaa-0000-4000-8000-000000000003"`) {
		t.Error("statement has no key anchor")
	}
}

// A rules page asks each question with the place in it, shows a statement
// borrowed from the situation guide with a link to it, shows the dated
// no-law line, and leaves gaps off (ADR-028 D4, D5).
func TestRulesPage_questionsAnswersAndNoLaw(t *testing.T) {
	ohio := store.Jurisdiction{ID: 1, Kind: "state", Name: "Ohio", Slug: "ohio"}
	borrowed := cited(9, "bbbbbbbb-0000-4000-8000-000000000009", "Ohio law gives your landlord 30 days.", "")
	stub := &stubStore{
		jurisdictions: []store.Jurisdiction{ohio},
		topics:        []store.Topic{{ID: 2, Slug: "security-deposit-rules", Name: "Security Deposit Rules"}},
		playbook: store.PlaybookWithStatements{
			Playbook:     store.Playbook{ID: 2, Title: "Security Deposit Rules in Ohio: What Does the Law Say?", Slug: "security-deposit-rules", Language: "en", PageKind: "rules"},
			Jurisdiction: ohio,
			Topic:        store.Topic{Slug: "security-deposit-rules", Name: "Security Deposit Rules", RulesFor: 7},
			Statements:   []store.CitedStatement{cited(10, "cccccccc-0000-4000-8000-000000000010", "Ohio law sets no cap.", "")},
		},
		rulesAnswers: []store.RulesAnswer{
			{Concept: store.Concept{Slug: "deposit-cap", Question: "How much can my landlord charge for a deposit?"},
				NoLaw: &store.CoverageRecord{CheckedAt: time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)}},
			{Concept: store.Concept{Slug: "deposit-receipt", Question: "Do I get a receipt for my deposit?"}},
			{Concept: store.Concept{Slug: "deposit-return-deadline", Question: "How long does my landlord have to return my deposit?"},
				Statement: &borrowed, PageTitle: "Deposit Not Returned in Ohio: What Can I Do?", TopicSlug: "security-deposits", PageKind: "playbook"},
		},
	}
	body := serveOK(t, stub, "/j/ohio/security-deposit-rules")
	if !strings.Contains(body, "How much can my landlord charge for a deposit in Ohio?") {
		t.Error("question heading missing the place")
	}
	if !strings.Contains(body, "We did not find an Ohio law on this. Last checked October 4, 2026.") {
		t.Error("no-law line missing")
	}
	if strings.Contains(body, "receipt for my deposit in Ohio") {
		t.Error("a gap rendered as a heading over nothing")
	}
	if !strings.Contains(body, `href="/j/ohio/security-deposits#s-bbbbbbbb-0000-4000-8000-000000000009"`) {
		t.Error("borrowed answer does not link to its statement on the situation guide")
	}
	if !strings.Contains(body, `id="deposit-return-deadline"`) {
		t.Error("concept anchor missing")
	}
}
