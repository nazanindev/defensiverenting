package handlers

import (
	"net/http"

	tmpl "github.com/nazanindev/defensiverenting/web/templates"
)

func About(w http.ResponseWriter, r *http.Request) {
	render(w, r, http.StatusOK, tmpl.AboutPage{})
}

func Support(w http.ResponseWriter, r *http.Request) {
	render(w, r, http.StatusOK, tmpl.SupportPage{})
}
