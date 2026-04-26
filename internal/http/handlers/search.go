package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/nazanin212/bostontenantsrights/internal/store"
	tmpl "github.com/nazanin212/bostontenantsrights/web/templates"
)

type searchStore interface {
	GetJurisdictionBySlug(ctx context.Context, slug string) (store.Jurisdiction, error)
	Search(ctx context.Context, query string, jurisdictionID *int64, language string) ([]store.SearchResult, error)
}

// Search handles GET /search?q=...&j=...
func Search(db searchStore, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		jSlug := r.URL.Query().Get("j")

		var jurisdictionID *int64
		var jurisdictionSlug string
		if jSlug != "" {
			j, err := db.GetJurisdictionBySlug(r.Context(), jSlug)
			if err == nil {
				jurisdictionID = &j.ID
				jurisdictionSlug = j.Slug
			}
		}

		var results []store.SearchResult
		if q != "" {
			var err error
			results, err = db.Search(r.Context(), q, jurisdictionID, "en")
			if err != nil {
				logger.ErrorContext(r.Context(), "search", slog.Any("err", err))
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
		}

		render(w, r, http.StatusOK, tmpl.SearchPage{
			Query:            q,
			JurisdictionSlug: jurisdictionSlug,
			Results:          results,
		})
	}
}

