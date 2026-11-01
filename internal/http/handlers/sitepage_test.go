package handlers

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// With no store, every copy page serves its built-in text.
func TestSitePageServesDefaults(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for slug, want := range map[string]string{
		"about":     "How a guide gets made",
		"editorial": "How we check",
		"privacy":   "Delete your information",
		"support":   "Where the money will go",
	} {
		rec := httptest.NewRecorder()
		SitePage(nil, logger, slug)(rec, httptest.NewRequest(http.MethodGet, "/"+slug, nil))
		body := rec.Body.String()
		if rec.Code != http.StatusOK || !strings.Contains(body, "<h2>"+want+"</h2>") || !strings.Contains(body, `<article class="prose">`) {
			t.Errorf("%s: code %d, missing %q", slug, rec.Code, want)
		}
	}
}
