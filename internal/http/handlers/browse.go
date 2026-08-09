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
	ListPublishedChildCities(ctx context.Context, parentID int64) ([]store.Jurisdiction, error)
	GetJurisdictionBySlug(ctx context.Context, slug string) (store.Jurisdiction, error)
	ListTopicsByJurisdiction(ctx context.Context, id int64, lang string) ([]store.Topic, error)
	GetPlaybook(ctx context.Context, jurisdictionSlug, topicSlug, language string) (store.PlaybookWithStatements, error)
	GetTopicBySlug(ctx context.Context, slug string) (store.Topic, error)
	ListPublishedTopics(ctx context.Context, language string) ([]store.Topic, error)
	ListJurisdictionsByTopic(ctx context.Context, topicID int64, language string) ([]store.Jurisdiction, error)
	ResolveJurisdictionAlias(ctx context.Context, alias string) (store.Jurisdiction, error)
	ResolveTopicAlias(ctx context.Context, alias string) (store.Topic, error)
}

// Reviewer is the human who verifies every playbook before publishing.
// Surfaced in the byline and as reviewedBy in JSON-LD; bio at ReviewerPath.
const (
	ReviewerName = "Cameron Monteith"
	ReviewerPath = "/authors/cameron-monteith"
)

// Browse wires the browse routes onto a chi.Router.
//
// URLs are hierarchical (ADR-005 D2): /j/{state}/{city}/{topic}. Two segments
// are ambiguous in shape between a state playbook (/j/texas/security-deposits)
// and a city hub (/j/texas/austin), so segment two is classified with one
// indexed lookup. That cost buys collision-free city slugs: Portland OR and
// Portland ME are distinct without either carrying a state suffix.
//
// The flat routes this replaces are not aliased. A flat URL redirects because
// the handler can read the parent off the jurisdiction row, so moving 19 live
// URLs under their states needed no migration data at all — only the topic
// renames do.
func Browse(r chi.Router, db browseStore, logger *slog.Logger) {
	r.Get("/", index(db, logger))
	r.Get("/j/{a}", oneSegment(db, logger))
	r.Get("/j/{a}/{b}", twoSegment(db, logger))
	r.Get("/j/{a}/{b}/{c}", cityPlaybook(db, logger))
	r.Get("/t/{topic}", topicHub(db, logger))
	r.Get("/locations", locations(db, logger))
	r.Get(ReviewerPath, author)
}

// oneSegment serves /j/{a}: a state or country hub. A city here is a flat URL
// from before the hierarchy and redirects to its full address.
func oneSegment(db browseStore, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "a")
		j, err := db.GetJurisdictionBySlug(r.Context(), slug)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				if moved, aerr := db.ResolveJurisdictionAlias(r.Context(), slug); aerr == nil {
					// Not an open redirect: the destination is built by Path()
					// from a slug the database returned. The request supplies
					// only the lookup key, never the target.
					http.Redirect(w, r, moved.Path(), http.StatusMovedPermanently) // #nosec G710
					return
				}
				http.NotFound(w, r)
				return
			}
			logger.ErrorContext(r.Context(), "get jurisdiction", slog.Any("err", err))
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if j.Kind == "city" && j.ParentSlug != "" {
			// Not an open redirect: j came from the database.
			http.Redirect(w, r, j.Path(), http.StatusMovedPermanently) // #nosec G710
			return
		}
		renderJurisdiction(w, r, db, logger, j)
	}
}

// twoSegment serves /j/{a}/{b}, which is either a state playbook or a city hub,
// or the flat pre-hierarchy form of a city playbook.
func twoSegment(db browseStore, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, b := chi.URLParam(r, "a"), chi.URLParam(r, "b")

		parent, err := db.GetJurisdictionBySlug(r.Context(), a)
		if err != nil {
			if !errors.Is(err, store.ErrNotFound) {
				logger.ErrorContext(r.Context(), "get jurisdiction", slog.Any("err", err))
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			if moved, aerr := db.ResolveJurisdictionAlias(r.Context(), a); aerr == nil {
				// Not an open redirect: both halves of the target are
				// database-resolved, moved from the alias table and the topic
				// from canonicalTopic.
				http.Redirect(w, r, moved.TopicPath(canonicalTopic(r.Context(), db, b)), http.StatusMovedPermanently) // #nosec G710
				return
			}
			http.NotFound(w, r)
			return
		}

		// /j/{city}/{topic}: the flat form. Redirect to the full address,
		// canonicalising the topic on the way so one hop is always enough.
		if parent.Kind == "city" {
			if target := parent.TopicPath(canonicalTopic(r.Context(), db, b)); target != r.URL.Path {
				// Not an open redirect: parent and the canonical topic are both
				// database-resolved.
				http.Redirect(w, r, target, http.StatusMovedPermanently) // #nosec G710
				return
			}
			// Already canonical. A city with no parent addresses itself flatly,
			// so redirecting would send this URL to itself forever.
			servePlaybook(w, r, db, logger, parent, b)
			return
		}

		// /j/{state}/{city}: a city hub.
		if child, cerr := db.GetJurisdictionBySlug(r.Context(), b); cerr == nil && child.Kind == "city" {
			if child.ParentSlug != parent.Slug {
				// Right city, wrong state in the URL. Not an open redirect:
				// child came from the database.
				http.Redirect(w, r, child.Path(), http.StatusMovedPermanently) // #nosec G710
				return
			}
			renderJurisdiction(w, r, db, logger, child)
			return
		}

		// /j/{state}/{topic}: a state playbook.
		servePlaybook(w, r, db, logger, parent, b)
	}
}

// cityPlaybook serves /j/{state}/{city}/{topic}.
func cityPlaybook(db browseStore, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stateSlug, citySlug, topicSlug := chi.URLParam(r, "a"), chi.URLParam(r, "b"), chi.URLParam(r, "c")

		city, err := db.GetJurisdictionBySlug(r.Context(), citySlug)
		if err != nil {
			if !errors.Is(err, store.ErrNotFound) {
				logger.ErrorContext(r.Context(), "get jurisdiction", slog.Any("err", err))
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			if moved, aerr := db.ResolveJurisdictionAlias(r.Context(), citySlug); aerr == nil {
				// Not an open redirect: alias target and canonical topic both
				// come from the database.
				http.Redirect(w, r, moved.TopicPath(canonicalTopic(r.Context(), db, topicSlug)), http.StatusMovedPermanently) // #nosec G710
				return
			}
			http.NotFound(w, r)
			return
		}
		if city.ParentSlug != stateSlug {
			if target := city.TopicPath(canonicalTopic(r.Context(), db, topicSlug)); target != r.URL.Path {
				// Not an open redirect: city and the canonical topic are both
				// database-resolved.
				http.Redirect(w, r, target, http.StatusMovedPermanently) // #nosec G710
				return
			}
			http.NotFound(w, r)
			return
		}
		servePlaybook(w, r, db, logger, city, topicSlug)
	}
}

// canonicalTopic maps a retired topic slug to the live one, so a redirect built
// from it lands on a real page instead of bouncing again.
func canonicalTopic(ctx context.Context, db browseStore, slug string) string {
	if _, err := db.GetTopicBySlug(ctx, slug); err == nil {
		return slug
	}
	if moved, err := db.ResolveTopicAlias(ctx, slug); err == nil {
		return moved.Slug
	}
	return slug
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
			LocationGroups: tmpl.GroupByState(jurisdictions),
			CityCount:      len(jurisdictions),
			Topics:         topics,
			StructuredData: siteSchema(),
		})
	}
}

// locations serves /locations: every covered city, grouped by state.
//
// The homepage used to carry this list itself, which meant it grew one card per
// city without bound. Moving it here keeps every city one crawlable link from
// the homepage while the homepage stays a fixed size.
func locations(db browseStore, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cities, err := db.ListPublishedCityJurisdictions(r.Context())
		if err != nil {
			logger.ErrorContext(r.Context(), "list jurisdictions", slog.Any("err", err))
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		render(w, r, http.StatusOK, tmpl.LocationsPage{
			Groups:    tmpl.GroupByState(cities),
			CityCount: len(cities),
		})
	}
}

// renderJurisdiction renders a hub page for a state, country, or city.
func renderJurisdiction(w http.ResponseWriter, r *http.Request, db browseStore, logger *slog.Logger, j store.Jurisdiction) {
	topics, err := db.ListTopicsByJurisdiction(r.Context(), j.ID, "en")
	if err != nil {
		logger.ErrorContext(r.Context(), "list topics", slog.Any("err", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// A state or country also lists the cities beneath it. Without this the
	// middle of the URL tree was unreachable: /j/massachusetts rendered "no
	// playbooks yet" while Boston sat directly under it. Best-effort, since a
	// missing city list should degrade the page rather than 500 it.
	var cities []store.Jurisdiction
	if j.Kind != "city" {
		if children, cerr := db.ListPublishedChildCities(r.Context(), j.ID); cerr == nil {
			cities = children
		} else {
			logger.ErrorContext(r.Context(), "list child cities", slog.Any("err", cerr))
		}
	}

	render(w, r, http.StatusOK, tmpl.JurisdictionPage{Jurisdiction: j, Topics: topics, Cities: cities})
}

// servePlaybook renders one playbook for an already-resolved jurisdiction, or
// redirects when the topic slug was renamed.
func servePlaybook(w http.ResponseWriter, r *http.Request, db browseStore, logger *slog.Logger, j store.Jurisdiction, topicSlug string) {
	pb, err := db.GetPlaybook(r.Context(), j.Slug, topicSlug, "en")
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			if moved, aerr := db.ResolveTopicAlias(r.Context(), topicSlug); aerr == nil {
				// Not an open redirect: j is the resolved jurisdiction and
				// moved.Slug comes from the topic alias table.
				http.Redirect(w, r, j.TopicPath(moved.Slug), http.StatusMovedPermanently) // #nosec G710
				return
			}
			http.NotFound(w, r)
			return
		}
		logger.ErrorContext(r.Context(), "get playbook", slog.Any("err", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	page := BuildPlaybookPage(r.Context(), pb, logger)

	// Cross-links: other topics in this jurisdiction, and this topic elsewhere.
	// Best-effort; a failure degrades the page rather than 500ing it.
	if topics, terr := db.ListTopicsByJurisdiction(r.Context(), pb.Jurisdiction.ID, "en"); terr == nil {
		for _, t := range topics {
			if t.ID != pb.Topic.ID {
				page.SiblingTopics = append(page.SiblingTopics, t)
			}
		}
	} else {
		logger.ErrorContext(r.Context(), "list sibling topics", slog.Any("err", terr))
	}
	if cities, cerr := db.ListJurisdictionsByTopic(r.Context(), pb.Topic.ID, "en"); cerr == nil {
		for _, c := range cities {
			if c.ID != pb.Jurisdiction.ID {
				page.OtherCities = append(page.OtherCities, c)
			}
		}
	} else {
		logger.ErrorContext(r.Context(), "list other cities", slog.Any("err", cerr))
	}

	render(w, r, http.StatusOK, page)
}

func topicHub(db browseStore, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "topic")
		t, err := db.GetTopicBySlug(r.Context(), slug)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				// The 2026-08-01 topic cleanup retired slugs like
				// pittsburgh-discrimination; those URLs land here.
				if moved, aerr := db.ResolveTopicAlias(r.Context(), slug); aerr == nil {
					http.Redirect(w, r, "/t/"+moved.Slug, http.StatusMovedPermanently)
					return
				}
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

		// ?j= carries the reader's chosen location. Resolving it here rather
		// than in the browser is what lets the homepage link situations
		// straight into a city without shipping a city×topic map to every
		// visitor to avoid 404s. A city that lacks this topic falls through to
		// the list instead of erroring. 302, not 301: coverage grows, so this
		// mapping must not be cached in browsers forever.
		if jSlug := r.URL.Query().Get("j"); jSlug != "" {
			if city, jerr := db.GetJurisdictionBySlug(r.Context(), jSlug); jerr == nil {
				for _, c := range cities {
					if c.ID == city.ID {
						http.Redirect(w, r, c.TopicPath(t.Slug), http.StatusFound)
						return
					}
				}
			}
		}

		// The query returns anywhere with this topic published, which includes
		// state-level guides. Split them: a state grouped by its parent would
		// file under "United States" and read as a city you could pick.
		var inCities, statewide []store.Jurisdiction
		for _, c := range cities {
			if c.Kind == "city" {
				inCities = append(inCities, c)
			} else {
				statewide = append(statewide, c)
			}
		}

		render(w, r, http.StatusOK, tmpl.TopicHubPage{
			Topic:     t,
			Groups:    tmpl.GroupByState(inCities),
			Statewide: statewide,
			CityCount: len(inCities),
		})
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

	canonical := tmpl.BaseURL() + pb.Jurisdiction.TopicPath(pb.Topic.Slug)

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
			{Type: "ListItem", Position: 2, Name: pb.Jurisdiction.Name, Item: tmpl.BaseURL() + pb.Jurisdiction.Path()},
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
