package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/nazanin212/bostontenantsrights/internal/http/handlers"
	"github.com/nazanin212/bostontenantsrights/internal/http/middleware"
	"github.com/nazanin212/bostontenantsrights/internal/store"
)

// NewRouter wires all routes and middleware onto a chi.Router.
func NewRouter(db *store.PG, logger *slog.Logger) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Logger(logger))
	r.Use(chimw.Recoverer)

	// Health endpoints — no caching
	r.Get("/healthz", handlers.Healthz)
	r.Get("/readyz", handlers.Readyz(db))

	// Browse + search — aggressive public caching
	r.Group(func(r chi.Router) {
		r.Use(middleware.StaticCache)
		handlers.Browse(r, db, logger)
		r.Get("/search", handlers.Search(db, logger))
	})

	// Static assets
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	return r
}
