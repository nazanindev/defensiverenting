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
				// Not an open redirect: base is the configured absolute site
				// URL, already parsed and checked for a host, so the target is
				// always on canonicalHost. The request contributes only the
				// path and query that follow it.
				http.Redirect(w, r, base+r.URL.RequestURI(), http.StatusMovedPermanently) // #nosec G710
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
