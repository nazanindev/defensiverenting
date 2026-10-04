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
