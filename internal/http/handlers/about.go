package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/nazanindev/defensiverenting/internal/sitecopy"
	"github.com/nazanindev/defensiverenting/internal/store"
	tmpl "github.com/nazanindev/defensiverenting/web/templates"
)

// SitePageStore reads site copy saved from the authoring app's copy screen.
type SitePageStore interface {
	GetSitePage(ctx context.Context, slug string) (store.SitePage, bool, error)
}

// SitePage serves one prose page (/about, /editorial, /privacy, /support).
// The saved copy wins when there is one; otherwise, and whenever the store
// cannot be read, the built-in text is served. A copy page must never 500
// over a database hiccup: it is the page a reader opens to decide whether to
// trust the rest.
func SitePage(db SitePageStore, logger *slog.Logger, slug string) http.HandlerFunc {
	info, ok := sitecopy.Lookup(slug)
	if !ok {
		panic("sitecopy: unknown page " + slug)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		page := sitecopy.Default(slug)
		if db != nil {
			saved, found, err := db.GetSitePage(r.Context(), slug)
			switch {
			case err != nil:
				logger.ErrorContext(r.Context(), "read site page", slog.String("slug", slug), slog.Any("err", err))
			case found:
				page = sitecopy.Page{Slug: slug, Title: saved.Title, Description: saved.Description, Body: saved.Body}
			}
		}
		body, err := sitecopy.Render(page.Body)
		if err != nil {
			logger.ErrorContext(r.Context(), "render site page", slog.String("slug", slug), slog.Any("err", err))
			page = sitecopy.Default(slug)
			body, _ = sitecopy.Render(page.Body)
		}
		render(w, r, http.StatusOK, tmpl.SitePage{
			Path:        info.Path,
			Crumb:       info.Crumb,
			Title:       page.Title,
			Description: page.Description,
			BodyHTML:    body,
		})
	}
}
