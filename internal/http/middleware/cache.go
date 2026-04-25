package middleware

import "net/http"

// StaticCache sets aggressive public caching headers for browse/search routes.
// Tenant-rights statutes change on legislative timescales, not minutes.
func StaticCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400, stale-while-revalidate=604800")
		next.ServeHTTP(w, r)
	})
}
