package handlers

import (
	"net/http"

	tmpl "github.com/nazanin212/bostontenantsrights/web/templates"
)

// render executes the appropriate template for the given page data.
func render(w http.ResponseWriter, r *http.Request, status int, page any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := tmpl.Render(w, page); err != nil {
		// Headers already sent; log only.
		_ = err
	}
}
