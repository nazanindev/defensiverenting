package middleware

import "net/http"

// StaticCache sets public caching headers for browse/search routes. HTML gets
// a short max-age so publishes, corrections, and URL changes propagate in
// minutes (a 24h max-age kept serving deleted links after the topic-slug
// migration), with stale-while-revalidate absorbing the revalidation cost.
func StaticCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=300, stale-while-revalidate=86400")
		next.ServeHTTP(w, r)
	})
}
