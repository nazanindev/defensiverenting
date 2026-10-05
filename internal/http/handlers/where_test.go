package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/http/handlers"
)

// ADR-031 D2: the guess comes only from the CDN's visitor headers, only for a
// US state or DC, and is never cached.
func TestWhere(t *testing.T) {
	cases := []struct {
		country, region, want string
	}{
		{"US", "OH", `{"j":"ohio"}`},
		{"US", "dc", `{"j":"district-of-columbia"}`},
		{"US", "PR", `{}`},
		{"CA", "ON", `{}`},
		{"", "", `{}`},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodGet, handlers.WherePath, nil)
		if c.country != "" {
			req.Header.Set("CF-IPCountry", c.country)
			req.Header.Set("CF-Region-Code", c.region)
		}
		rec := httptest.NewRecorder()
		handlers.Where(rec, req)
		if got := strings.TrimSpace(rec.Body.String()); got != c.want {
			t.Errorf("%s/%s: got %s, want %s", c.country, c.region, got, c.want)
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s/%s: the guess must never be cached", c.country, c.region)
		}
	}
}
