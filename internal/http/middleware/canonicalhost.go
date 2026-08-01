package middleware

import (
	"net/http"
	"net/url"
	"strings"
)

// CanonicalHost 301-redirects requests whose Host does not match the canonical
// site URL (e.g. the *.fly.dev host after the custom domain cutover). Health
// endpoints are exempt so platform checks keep working. Returns a no-op
// middleware when enabled is false or siteURL cannot be parsed.
func CanonicalHost(siteURL string, enabled bool) func(http.Handler) http.Handler {
	parsed, err := url.Parse(siteURL)
	if !enabled || err != nil || parsed.Host == "" {
		return func(next http.Handler) http.Handler { return next }
	}
	canonicalHost := parsed.Host
	base := strings.TrimRight(siteURL, "/")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Host != canonicalHost &&
				r.URL.Path != "/healthz" && r.URL.Path != "/readyz" {
				http.Redirect(w, r, base+r.URL.RequestURI(), http.StatusMovedPermanently)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
