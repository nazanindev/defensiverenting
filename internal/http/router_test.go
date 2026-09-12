package http

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// HEAD must answer like GET. Link checkers and preview fetchers send HEAD, and
// a 405 there reads as a broken site even though GET works.
func TestHeadAnswersLikeGet(t *testing.T) {
	h := NewRouter(nil, slog.New(slog.NewTextHandler(io.Discard, nil)), RouterConfig{SiteURL: "http://example.test"})
	for _, path := range []string{"/healthz", "/about"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodHead, path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("HEAD %s: got %d, want 200", path, rec.Code)
		}
	}
}
