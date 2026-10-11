package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/nazanindev/defensiverenting/internal/store"
	tmpl "github.com/nazanindev/defensiverenting/web/templates"
)

// placeCache feeds the site-wide place picker in the header (ADR-031 as
// amended 2026-10-11). The header renders on every page, so the list of
// published hubs is read once and kept for a few minutes rather than
// queried per render. Coverage grows by the week, not the minute.
type placeCache struct {
	db     browseStore
	logger *slog.Logger

	mu       sync.Mutex
	at       time.Time
	groups   []tmpl.StateGroup
	national []store.Jurisdiction
}

const placeCacheTTL = 5 * time.Minute

func (c *placeCache) get() ([]tmpl.StateGroup, []store.Jurisdiction) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Since(c.at) < placeCacheTTL {
		return c.groups, c.national
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	hubs, err := c.db.ListPublishedHubJurisdictions(ctx)
	if err != nil {
		c.logger.Error("place picker: list hubs", slog.Any("err", err))
		// Keep whatever was loaded last; an empty header is worse than a
		// stale one.
		return c.groups, c.national
	}
	var national, states, cities []store.Jurisdiction
	for _, j := range hubs {
		switch j.Kind {
		case "country":
			national = append(national, j)
		case "state":
			states = append(states, j)
		case "city":
			cities = append(cities, j)
		}
	}
	c.groups = tmpl.MarkStatewide(tmpl.GroupByState(cities), states)
	c.national = national
	c.at = time.Now()
	return c.groups, c.national
}

// goPlace serves /go?j={slug}: where the header picker sends a reader on a
// page that is not a guide. Without JavaScript the picker's Go button
// submits here; with it, the page submits the same form. A guide's picker
// submits to its topic hub instead, which resolves ?j= to the nearest guide.
func goPlace(db browseStore, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.URL.Query().Get("j")
		if slug == "" {
			http.Redirect(w, r, "/locations", http.StatusFound)
			return
		}
		j, err := db.GetJurisdictionBySlug(r.Context(), slug)
		if err != nil {
			if !errors.Is(err, store.ErrNotFound) {
				logger.ErrorContext(r.Context(), "go: get jurisdiction", slog.Any("err", err))
			}
			http.Redirect(w, r, "/locations", http.StatusFound)
			return
		}
		// j.Path() is a site-relative path built from the stored slug, never
		// from the query value, so this cannot redirect off the site.
		http.Redirect(w, r, j.Path(), http.StatusFound) //nolint:gosec
	}
}
