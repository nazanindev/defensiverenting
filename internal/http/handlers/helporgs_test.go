package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/nazanindev/defensiverenting/internal/http/handlers"
	"github.com/nazanindev/defensiverenting/internal/store"
)

// ADR-029 D3: a statement held back by an org that has not said yes is left
// off the live page; the rest of the page still serves.
func TestPlaybookHandler_leavesOutHiddenStatements(t *testing.T) {
	stub := localHelpStub([]store.Topic{localHelpTopic}, localHelpTopic)
	stub.playbook.Statements = append(stub.playbook.Statements, store.CitedStatement{
		ID: 2, BodyMD: "Call the Small Tenant Union.",
		Citations: []store.CitationWithSource{{
			SourceID: 2, SourceURL: "https://smalltenants.example/",
			Publisher: "Small Tenant Union", SourceKind: "nonprofit",
		}},
	})
	stub.hidden = map[int64]string{2: "smalltenants.example"}

	body := getPlaybookBody(t, stub, "/j/massachusetts/boston/resource-directory")
	if strings.Contains(body, "Small Tenant Union") {
		t.Error("a hidden org's statement reached the live page")
	}
	if !strings.Contains(body, "A claim.") {
		t.Error("the page lost a statement nothing holds back")
	}
}

// ADR-029 D6: chips and phone numbers carry the source id the page script
// reports a click against; the chip still links straight to the source.
func TestPlaybookHandler_clickableSourcesCarryTheirID(t *testing.T) {
	stub := localHelpStub([]store.Topic{localHelpTopic}, localHelpTopic)
	stub.playbook.Statements = append(stub.playbook.Statements, store.CitedStatement{
		ID: 2, BodyMD: "Call the hotline at 412-555-0134.",
		Citations: []store.CitationWithSource{{
			SourceID: 42, SourceURL: "https://hotline.example/",
			Publisher: "Hotline", SourceKind: "gov_guidance",
		}},
	})
	body := getPlaybookBody(t, stub, "/j/massachusetts/boston/resource-directory")
	for _, want := range []string{
		`href="tel:+14125550134" data-out="42"`,
		`data-out="42"`,
		`href="https://hotline.example/"`,
		"navigator.sendBeacon('/out'",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page lacks %s", want)
		}
	}
}

// A page whose every statement is hidden is not served at all.
func TestPlaybookHandler_allHiddenIsNotFound(t *testing.T) {
	stub := localHelpStub([]store.Topic{localHelpTopic}, localHelpTopic)
	stub.hidden = map[int64]string{1: "smalltenants.example"}

	r := chi.NewRouter()
	handlers.Browse(r, stub, logger())
	req := httptest.NewRequest(http.MethodGet, "/j/massachusetts/boston/resource-directory", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}
