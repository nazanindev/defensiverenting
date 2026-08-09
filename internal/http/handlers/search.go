package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/nazanindev/defensiverenting/internal/store"
	tmpl "github.com/nazanindev/defensiverenting/web/templates"
)

type searchStore interface {
	GetJurisdictionBySlug(ctx context.Context, slug string) (store.Jurisdiction, error)
	ListPublishedCityJurisdictions(ctx context.Context) ([]store.Jurisdiction, error)
	Search(ctx context.Context, query string, jurisdictionID *int64, language string) ([]store.SearchResult, error)
}

// Search handles GET /search?q=...&j=...
func Search(db searchStore, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		jSlug := r.URL.Query().Get("j")

		var jurisdictionID *int64
		var jurisdictionSlug, jurisdictionName string
		if jSlug != "" {
			j, err := db.GetJurisdictionBySlug(r.Context(), jSlug)
			if err == nil {
				jurisdictionID = &j.ID
				jurisdictionSlug = j.Slug
				jurisdictionName = j.Name
			}
		}

		// Powers the location picker on the results page, so the scope a search
		// ran under can be changed without going back. Best-effort: losing the
		// list costs the picker, not the results.
		var groups []tmpl.StateGroup
		if cities, cerr := db.ListPublishedCityJurisdictions(r.Context()); cerr == nil {
			groups = tmpl.GroupByState(cities)
		} else {
			logger.ErrorContext(r.Context(), "list jurisdictions for scope picker", slog.Any("err", cerr))
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
			JurisdictionName: jurisdictionName,
			LocationGroups:   groups,
			Results:          results,
		})
	}
}
