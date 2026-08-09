package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// Robots serves /robots.txt.
func Robots(siteURL string) http.HandlerFunc {
	body := "User-agent: *\nAllow: /\nDisallow: /search\n\nSitemap: " + siteURL + "/sitemap.xml\n"
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprint(w, body)
	}
}

type sitemapStore interface {
	// Published only: a city with nothing published has an empty hub page, and
	// submitting it invites Google to index a page with no content on it.
	ListPublishedCityJurisdictions(ctx context.Context) ([]store.Jurisdiction, error)
	ListSitemapURLs(ctx context.Context) ([]store.SitemapEntry, error)
	ListPublishedTopics(ctx context.Context, language string) ([]store.Topic, error)
}

// Sitemap serves /sitemap.xml listing all jurisdiction and playbook pages.
func Sitemap(db sitemapStore, siteURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jurisdictions, err := db.ListPublishedCityJurisdictions(r.Context())
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		entries, err := db.ListSitemapURLs(r.Context())
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		topics, err := db.ListPublishedTopics(r.Context(), "en")
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		fmt.Fprintf(w, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
		fmt.Fprintf(w, "<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n")
		fmt.Fprintf(w, "  <url><loc>%s/</loc><changefreq>weekly</changefreq></url>\n", siteURL)
		// /locations carries the crawlable link to every city hub. The homepage
		// stopped enumerating cities, so this is now the page that spreads
		// link equity to them.
		fmt.Fprintf(w, "  <url><loc>%s/locations</loc><changefreq>weekly</changefreq></url>\n", siteURL)
		for _, p := range []string{"/about", "/support", "/editorial", ReviewerPath} {
			fmt.Fprintf(w, "  <url><loc>%s%s</loc><changefreq>yearly</changefreq></url>\n", siteURL, p)
		}

		for _, t := range topics {
			fmt.Fprintf(w, "  <url><loc>%s/t/%s</loc><changefreq>weekly</changefreq></url>\n", siteURL, t.Slug)
		}

		for _, j := range jurisdictions {
			fmt.Fprintf(w, "  <url><loc>%s%s</loc><changefreq>weekly</changefreq></url>\n", siteURL, j.Path())
		}

		for _, e := range entries {
			if e.LastMod != nil {
				fmt.Fprintf(w, "  <url><loc>%s%s</loc><lastmod>%s</lastmod><changefreq>monthly</changefreq></url>\n",
					siteURL, e.Path(), e.LastMod.Format(time.DateOnly))
			} else {
				fmt.Fprintf(w, "  <url><loc>%s%s</loc><changefreq>monthly</changefreq></url>\n",
					siteURL, e.Path())
			}
		}

		fmt.Fprintf(w, "</urlset>\n")
	}
}
