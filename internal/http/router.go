package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/nazanindev/defensiverenting/internal/http/handlers"
	"github.com/nazanindev/defensiverenting/internal/http/middleware"
	"github.com/nazanindev/defensiverenting/internal/store"
	webstatic "github.com/nazanindev/defensiverenting/web/static"
)

// RouterConfig carries the deployment-specific values the routes need. It is a
// struct rather than positional arguments because the list has outgrown what a
// call site can be read at a glance.
type RouterConfig struct {
	SiteURL string
	// CanonicalRedirect enables 301s from non-canonical hosts (off in development).
	CanonicalRedirect bool
	// FormsURL is the origin of the Worker the report and contact forms post to.
	FormsURL string
	// TurnstileSiteKey is the public spam-check key embedded in those forms.
	TurnstileSiteKey string
}

// NewRouter wires all routes and middleware onto a chi.Router.
func NewRouter(db *store.PG, logger *slog.Logger, cfg RouterConfig) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Logger(logger))
	r.Use(chimw.Recoverer)
	r.Use(middleware.CanonicalHost(cfg.SiteURL, cfg.CanonicalRedirect))

	// Health endpoints — no caching
	r.Get("/healthz", handlers.Healthz)
	r.Get("/readyz", handlers.Readyz(db))

	// Browse + search + SEO meta — aggressive public caching
	r.Group(func(r chi.Router) {
		r.Use(middleware.StaticCache)
		handlers.Browse(r, db, logger)
		r.Get("/search", handlers.Search(db, logger))
		r.Get("/editorial", handlers.Editorial)
		r.Get("/about", handlers.About)
		r.Get("/support", handlers.Support)
		r.Get("/report", handlers.Report(cfg.FormsURL, cfg.TurnstileSiteKey))
		r.Get("/contact", handlers.Contact(cfg.FormsURL, cfg.TurnstileSiteKey))
		r.Get("/thanks", handlers.Thanks)
		r.Get("/robots.txt", handlers.Robots(cfg.SiteURL))
		r.Get("/sitemap.xml", handlers.Sitemap(db, cfg.SiteURL))
	})

	// Static assets
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(webstatic.Files))))

	return r
}
