package handlers

import (
	"net/http"

	tmpl "github.com/nazanindev/defensiverenting/web/templates"
)

func Editorial(w http.ResponseWriter, r *http.Request) {
	render(w, r, http.StatusOK, tmpl.EditorialPage{})
}
