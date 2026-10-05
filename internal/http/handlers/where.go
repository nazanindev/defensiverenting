package handlers

import (
	"encoding/json"
	"net/http"

	tmpl "github.com/nazanindev/defensiverenting/web/templates"
)

// WherePath answers "which state is this reader probably in?" (ADR-031 D2).
const WherePath = "/api/where"

// Where guesses the reader's state from the visitor location headers the CDN
// adds (Cloudflare's CF-IPCountry and CF-Region-Code). The guide page asks it
// only when the reader has no saved place, and shows the answer as a question
// ("Do you rent in Ohio?"). Nothing is stored or logged here: the guess lives
// only in the reader's browser until they tap it.
//
// With no such headers (local development, or the site not behind the CDN's
// proxy) it answers with no state, and the guide shows nothing.
//
// Never cached: the answer is about one reader.
func Where(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	out := map[string]string{}
	if r.Header.Get("CF-IPCountry") == "US" {
		if slug := tmpl.StateSlugForCode(r.Header.Get("CF-Region-Code")); slug != "" {
			out["j"] = slug
		}
	}
	_ = json.NewEncoder(w).Encode(out)
}
