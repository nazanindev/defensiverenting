package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nazanindev/defensiverenting/internal/content"
	"github.com/nazanindev/defensiverenting/internal/store"
	tmpl "github.com/nazanindev/defensiverenting/web/templates"
)

type browseStore interface {
	ListPublishedCityJurisdictions(ctx context.Context) ([]store.Jurisdiction, error)
	GetJurisdictionBySlug(ctx context.Context, slug string) (store.Jurisdiction, error)
	ListTopicsByJurisdiction(ctx context.Context, id int64, lang string) ([]store.Topic, error)
	GetPlaybook(ctx context.Context, jurisdictionSlug, topicSlug, language string) (store.PlaybookWithStatements, error)
	GetTopicBySlug(ctx context.Context, slug string) (store.Topic, error)
	ListPublishedTopics(ctx context.Context, language string) ([]store.Topic, error)
	ListJurisdictionsByTopic(ctx context.Context, topicID int64, language string) ([]store.Jurisdiction, error)
}

// Reviewer is the human who verifies every playbook before publishing.
// Surfaced in the byline and as reviewedBy in JSON-LD; bio at ReviewerPath.
const (
	ReviewerName = "Cameron Monteith"
	ReviewerPath = "/authors/cameron-monteith"
)

// Browse wires the browse routes onto a chi.Router.
func Browse(r chi.Router, db browseStore, logger *slog.Logger) {
	r.Get("/", index(db, logger))
	r.Get("/j/{jurisdiction}", jurisdictionIndex(db, logger))
	r.Get("/j/{jurisdiction}/{topic}", playbook(db, logger))
	r.Get("/t/{topic}", topicHub(db, logger))
	r.Get(ReviewerPath, author)
}

func index(db browseStore, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jurisdictions, err := db.ListPublishedCityJurisdictions(r.Context())
		if err != nil {
			logger.ErrorContext(r.Context(), "list jurisdictions", slog.Any("err", err))
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		topics, err := db.ListPublishedTopics(r.Context(), "en")
		if err != nil {
			logger.ErrorContext(r.Context(), "list published topics", slog.Any("err", err))
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		render(w, r, http.StatusOK, tmpl.IndexPage{
			Jurisdictions:  jurisdictions,
			Topics:         topics,
			StructuredData: siteSchema(),
		})
	}
}

func jurisdictionIndex(db browseStore, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "jurisdiction")
		j, err := db.GetJurisdictionBySlug(r.Context(), slug)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				http.NotFound(w, r)
				return
			}
			logger.ErrorContext(r.Context(), "get jurisdiction", slog.Any("err", err))
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		topics, err := db.ListTopicsByJurisdiction(r.Context(), j.ID, "en")
		if err != nil {
			logger.ErrorContext(r.Context(), "list topics", slog.Any("err", err))
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		render(w, r, http.StatusOK, tmpl.JurisdictionPage{Jurisdiction: j, Topics: topics})
	}
}

func playbook(db browseStore, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jSlug := chi.URLParam(r, "jurisdiction")
		tSlug := chi.URLParam(r, "topic")

		pb, err := db.GetPlaybook(r.Context(), jSlug, tSlug, "en")
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				http.NotFound(w, r)
				return
			}
			logger.ErrorContext(r.Context(), "get playbook", slog.Any("err", err))
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		page := BuildPlaybookPage(r.Context(), pb, logger)

		// Cross-links: other topics in this city, and this topic in other cities.
		// Best-effort; a failure degrades the page rather than 500ing it.
		if topics, err := db.ListTopicsByJurisdiction(r.Context(), pb.Jurisdiction.ID, "en"); err == nil {
			for _, t := range topics {
				if t.ID != pb.Topic.ID {
					page.SiblingTopics = append(page.SiblingTopics, t)
				}
			}
		} else {
			logger.ErrorContext(r.Context(), "list sibling topics", slog.Any("err", err))
		}
		if cities, err := db.ListJurisdictionsByTopic(r.Context(), pb.Topic.ID, "en"); err == nil {
			for _, c := range cities {
				if c.ID != pb.Jurisdiction.ID {
					page.OtherCities = append(page.OtherCities, c)
				}
			}
		} else {
			logger.ErrorContext(r.Context(), "list other cities", slog.Any("err", err))
		}

		render(w, r, http.StatusOK, page)
	}
}

func topicHub(db browseStore, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "topic")
		t, err := db.GetTopicBySlug(r.Context(), slug)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				http.NotFound(w, r)
				return
			}
			logger.ErrorContext(r.Context(), "get topic", slog.Any("err", err))
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		cities, err := db.ListJurisdictionsByTopic(r.Context(), t.ID, "en")
		if err != nil {
			logger.ErrorContext(r.Context(), "list cities for topic", slog.Any("err", err))
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if len(cities) == 0 {
			// A topic with no published playbooks is not a public page.
			http.NotFound(w, r)
			return
		}
		render(w, r, http.StatusOK, tmpl.TopicHubPage{Topic: t, Jurisdictions: cities})
	}
}

func author(w http.ResponseWriter, r *http.Request) {
	render(w, r, http.StatusOK, tmpl.AuthorPage{})
}

// BuildPlaybookPage converts a stored playbook into the public page model,
// rendering markdown and enforcing the citation invariant. Shared with the
// authoring tool's draft preview so previews match the live site exactly.
func BuildPlaybookPage(ctx context.Context, pb store.PlaybookWithStatements, logger *slog.Logger) tmpl.PlaybookPage {
	// Render markdown intro to HTML
	introHTML := content.RenderMarkdown(pb.IntroMD)

	// Render each statement's body markdown and validate citation invariant
	statements := make([]tmpl.RenderedStatement, 0, len(pb.Statements))
	var sourceURLs []string
	seenSources := make(map[string]bool)
	for _, s := range pb.Statements {
		if len(s.Citations) == 0 {
			// Log the violation and skip rather than panic in production.
			// Development builds should catch this via tests.
			logger.ErrorContext(ctx, "statement missing citations — invariant violated",
				slog.Int64("statement_id", s.ID))
			continue
		}
		chips := make([]tmpl.CitationChip, 0, len(s.Citations))
		for _, c := range s.Citations {
			chips = append(chips, tmpl.CitationChip{
				URL:        c.SourceURL + anchorFragment(c.Locator),
				Label:      c.Publisher,
				Locator:    c.Locator,
				SourceKind: c.SourceKind,
			})
			if !seenSources[c.SourceURL] && c.SourceKind != "editorial" {
				seenSources[c.SourceURL] = true
				sourceURLs = append(sourceURLs, c.SourceURL)
			}
		}
		statements = append(statements, tmpl.RenderedStatement{
			BodyHTML:  content.RenderMarkdown(s.BodyMD),
			Citations: chips,
		})
	}

	canonical := tmpl.BaseURL() + "/j/" + pb.Jurisdiction.Slug + "/" + pb.Topic.Slug

	var reviewedOn string
	switch {
	case pb.Playbook.LastReviewedAt != nil:
		reviewedOn = pb.Playbook.LastReviewedAt.Format("January 2, 2006")
	case !pb.Playbook.UpdatedAt.IsZero():
		reviewedOn = pb.Playbook.UpdatedAt.Format("January 2, 2006")
	}

	return tmpl.PlaybookPage{
		Playbook:       pb.Playbook,
		Jurisdiction:   pb.Jurisdiction,
		Topic:          pb.Topic,
		IntroHTML:      introHTML,
		Statements:     statements,
		Description:    metaDescription(pb.IntroMD, pb.Playbook.Title, pb.Jurisdiction.Name),
		Canonical:      canonical,
		StructuredData: playbookSchema(pb, canonical, sourceURLs),
		ReviewedOn:     reviewedOn,
	}
}

// anchorFragment returns a URL fragment for a locator if non-empty.
func anchorFragment(locator string) string {
	if locator == "" {
		return ""
	}
	return "#" + locator
}

const siteName = "RenterLaw"

type schemaOrg struct {
	Type string `json:"@type"`
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

type breadcrumbItem struct {
	Type     string `json:"@type"`
	Position int    `json:"position"`
	Name     string `json:"name"`
	Item     string `json:"item,omitempty"`
}

// playbookSchema builds a Schema.org @graph (Article + BreadcrumbList) for a
// playbook page. isBasedOn lists the primary sources the playbook cites.
func playbookSchema(pb store.PlaybookWithStatements, canonical string, sourceURLs []string) template.JS {
	article := struct {
		Type          string    `json:"@type"`
		Headline      string    `json:"headline"`
		Description   string    `json:"description"`
		URL           string    `json:"url"`
		MainEntity    string    `json:"mainEntityOfPage"`
		DatePublished string    `json:"datePublished,omitempty"`
		DateModified  string    `json:"dateModified,omitempty"`
		IsBasedOn     []string  `json:"isBasedOn,omitempty"`
		Author        schemaOrg `json:"author"`
		Publisher     schemaOrg `json:"publisher"`
		ReviewedBy    schemaOrg `json:"reviewedBy"`
	}{
		Type:        "Article",
		Headline:    pb.Playbook.Title + " — " + pb.Jurisdiction.Name + " Tenant Rights",
		Description: metaDescription(pb.IntroMD, pb.Playbook.Title, pb.Jurisdiction.Name),
		URL:         canonical,
		MainEntity:  canonical,
		IsBasedOn:   sourceURLs,
		Author:      schemaOrg{Type: "Organization", Name: siteName, URL: tmpl.BaseURL()},
		Publisher:   schemaOrg{Type: "Organization", Name: siteName, URL: tmpl.BaseURL()},
		ReviewedBy:  schemaOrg{Type: "Person", Name: ReviewerName, URL: tmpl.BaseURL() + ReviewerPath},
	}
	if pb.Playbook.PublishedAt != nil {
		article.DatePublished = pb.Playbook.PublishedAt.Format(time.DateOnly)
	}
	switch {
	case pb.Playbook.LastReviewedAt != nil:
		article.DateModified = pb.Playbook.LastReviewedAt.Format(time.DateOnly)
	case !pb.Playbook.UpdatedAt.IsZero():
		article.DateModified = pb.Playbook.UpdatedAt.Format(time.DateOnly)
	}

	breadcrumbs := struct {
		Type  string           `json:"@type"`
		Items []breadcrumbItem `json:"itemListElement"`
	}{
		Type: "BreadcrumbList",
		Items: []breadcrumbItem{
			{Type: "ListItem", Position: 1, Name: "Home", Item: tmpl.BaseURL() + "/"},
			{Type: "ListItem", Position: 2, Name: pb.Jurisdiction.Name, Item: tmpl.BaseURL() + "/j/" + pb.Jurisdiction.Slug},
			{Type: "ListItem", Position: 3, Name: pb.Topic.Name},
		},
	}

	graph := struct {
		Context string `json:"@context"`
		Graph   []any  `json:"@graph"`
	}{
		Context: "https://schema.org",
		Graph:   []any{article, breadcrumbs},
	}
	data, _ := json.Marshal(graph)
	return template.JS(data) //nolint:gosec // safe: json.Marshal HTML-escapes all string fields; no raw user input reaches this value
}

// siteSchema builds WebSite + Organization JSON-LD for the homepage.
func siteSchema() template.JS {
	base := tmpl.BaseURL()
	graph := struct {
		Context string `json:"@context"`
		Graph   []any  `json:"@graph"`
	}{
		Context: "https://schema.org",
		Graph: []any{
			struct {
				Type        string    `json:"@type"`
				Name        string    `json:"name"`
				URL         string    `json:"url"`
				Description string    `json:"description"`
				Publisher   schemaOrg `json:"publisher"`
			}{
				Type:        "WebSite",
				Name:        siteName,
				URL:         base + "/",
				Description: "Free tenant rights guides backed by primary legal sources, organized by city and situation.",
				Publisher:   schemaOrg{Type: "Organization", Name: siteName, URL: base},
			},
			schemaOrg{Type: "Organization", Name: siteName, URL: base},
		},
	}
	data, _ := json.Marshal(graph)
	return template.JS(data) //nolint:gosec // safe: json.Marshal HTML-escapes all string fields; no raw user input reaches this value
}

var mdLinkRe = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
var mdMarkupRe = regexp.MustCompile("[#*_`>]+")

// metaDescription derives a plain-text meta description from the playbook
// intro, falling back to a templated line when there is no intro. Truncates
// near 155 characters on a word boundary.
func metaDescription(introMD, title, jurisdiction string) string {
	text := mdLinkRe.ReplaceAllString(introMD, "$1")
	text = mdMarkupRe.ReplaceAllString(text, "")
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return title + " — free, step-by-step tenant rights guide for " + jurisdiction + ", backed by primary sources."
	}
	const limit = 155
	if len(text) <= limit {
		return text
	}
	cut := strings.LastIndex(text[:limit], " ")
	if cut <= 0 {
		cut = limit
	}
	return text[:cut] + "…"
}
