package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/nazanin212/bostontenantsrights/internal/store"
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
	ListCityJurisdictions(ctx context.Context) ([]store.Jurisdiction, error)
	ListSitemapURLs(ctx context.Context) ([]store.SitemapEntry, error)
}

// Sitemap serves /sitemap.xml listing all jurisdiction and playbook pages.
func Sitemap(db sitemapStore, siteURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jurisdictions, err := db.ListCityJurisdictions(r.Context())
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		entries, err := db.ListSitemapURLs(r.Context())
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		fmt.Fprintf(w, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
		fmt.Fprintf(w, "<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n")
		fmt.Fprintf(w, "  <url><loc>%s/</loc><changefreq>weekly</changefreq></url>\n", siteURL)

		for _, j := range jurisdictions {
			fmt.Fprintf(w, "  <url><loc>%s/j/%s</loc><changefreq>weekly</changefreq></url>\n", siteURL, j.Slug)
		}

		for _, e := range entries {
			if e.LastMod != nil {
				fmt.Fprintf(w, "  <url><loc>%s/j/%s/%s</loc><lastmod>%s</lastmod><changefreq>monthly</changefreq></url>\n",
					siteURL, e.JurisdictionSlug, e.TopicSlug, e.LastMod.Format(time.DateOnly))
			} else {
				fmt.Fprintf(w, "  <url><loc>%s/j/%s/%s</loc><changefreq>monthly</changefreq></url>\n",
					siteURL, e.JurisdictionSlug, e.TopicSlug)
			}
		}

		fmt.Fprintf(w, "</urlset>\n")
	}
}
