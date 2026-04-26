package handlers

import (
	"net/http"

	tmpl "github.com/nazanin212/bostontenantsrights/web/templates"
)

func Editorial(w http.ResponseWriter, r *http.Request) {
	render(w, r, http.StatusOK, tmpl.EditorialPage{})
}
