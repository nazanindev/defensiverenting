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
	"github.com/nazanindev/defensiverenting/internal/voice"
	tmpl "github.com/nazanindev/defensiverenting/web/templates"
)

type browseStore interface {
	ListPublishedCityJurisdictions(ctx context.Context) ([]store.Jurisdiction, error)
	ListPublishedHubJurisdictions(ctx context.Context) ([]store.Jurisdiction, error)
	ListPublishedChildCities(ctx context.Context, parentID int64) ([]store.Jurisdiction, error)
	GetJurisdictionBySlug(ctx context.Context, slug string) (store.Jurisdiction, error)
	GetNearestTopicJurisdiction(ctx context.Context, jurisdictionID, topicID int64, lang string) (store.Jurisdiction, error)
	ListTopicsByJurisdiction(ctx context.Context, id int64, lang string) ([]store.Topic, error)
	ListTopicsByJurisdictionRecursive(ctx context.Context, id int64, lang string) ([]store.Topic, error)
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
//
// Language (ADR-007 D1/D2): every /j/* and /t/* pattern above is registered
// once per voice.Supported() language, bare for English and under a literal
// "/es" (etc.) prefix otherwise — not a {lang} wildcard, so a typo'd or
// unsupported code 404s instead of silently resolving. Everything else
// (/, /search, /locations, the static pages) stays English-only: ADR-007 D2
// explains why that is the right scope, not a shortcut.
func Browse(r chi.Router, db browseStore, logger *slog.Logger) {
	r.Get("/", index(db, logger))
	r.Get("/locations", locations(db, logger))
	r.Get("/api/coverage", coverage(db, logger))
	r.Get(ReviewerPath, author)

	for _, lang := range voice.Supported() {
		prefix := store.LangPrefix(lang)
		r.Get(prefix+"/j/{a}", oneSegment(db, logger, lang))
		r.Get(prefix+"/j/{a}/{b}", twoSegment(db, logger, lang))
		r.Get(prefix+"/j/{a}/{b}/{c}", cityPlaybook(db, logger, lang))
		r.Get(prefix+"/t/{topic}", topicHub(db, logger, lang))
	}
}

// oneSegment serves /j/{a} (or /es/j/{a}, ...): a state or country hub. A city
// here is a flat URL from before the hierarchy and redirects to its full
// address, in the same language.
func oneSegment(db browseStore, logger *slog.Logger, lang string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "a")
		j, err := db.GetJurisdictionBySlug(r.Context(), slug)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				if moved, aerr := db.ResolveJurisdictionAlias(r.Context(), slug); aerr == nil {
					// Not an open redirect: the destination is built by PathIn()
					// from a slug the database returned and the language this
					// route was registered under. The request supplies only the
					// lookup key, never the target.
					http.Redirect(w, r, moved.PathIn(lang), http.StatusMovedPermanently) // #nosec G710
					return
				}
				render(w, r, http.StatusNotFound, tmpl.NotFoundPage{UncoveredPlace: true, Language: lang})
				return
			}
			logger.ErrorContext(r.Context(), "get jurisdiction", slog.Any("err", err))
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if j.Kind == "city" && j.ParentSlug != "" {
			// Not an open redirect: j came from the database.
			http.Redirect(w, r, j.PathIn(lang), http.StatusMovedPermanently) // #nosec G710
			return
		}
		renderJurisdiction(w, r, db, logger, j, lang)
	}
}

// twoSegment serves /j/{a}/{b} (or /es/j/{a}/{b}, ...), which is either a
// state playbook or a city hub, or the flat pre-hierarchy form of a city
// playbook.
func twoSegment(db browseStore, logger *slog.Logger, lang string) http.HandlerFunc {
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
				http.Redirect(w, r, moved.TopicPathIn(lang, canonicalTopic(r.Context(), db, b)), http.StatusMovedPermanently) // #nosec G710
				return
			}
			render(w, r, http.StatusNotFound, tmpl.NotFoundPage{UncoveredPlace: true, Language: lang})
			return
		}

		// /j/{city}/{topic}: the flat form. Redirect to the full address,
		// canonicalising the topic on the way so one hop is always enough.
		if parent.Kind == "city" {
			if target := parent.TopicPathIn(lang, canonicalTopic(r.Context(), db, b)); target != r.URL.Path {
				// Not an open redirect: parent and the canonical topic are both
				// database-resolved.
				http.Redirect(w, r, target, http.StatusMovedPermanently) // #nosec G710
				return
			}
			// Already canonical. A city with no parent addresses itself flatly,
			// so redirecting would send this URL to itself forever.
			servePlaybook(w, r, db, logger, parent, b, lang)
			return
		}

		// /j/{state}/{city}: a city hub.
		if child, cerr := db.GetJurisdictionBySlug(r.Context(), b); cerr == nil && child.Kind == "city" {
			if child.ParentSlug != parent.Slug {
				// Right city, wrong state in the URL. Not an open redirect:
				// child came from the database.
				http.Redirect(w, r, child.PathIn(lang), http.StatusMovedPermanently) // #nosec G710
				return
			}
			renderJurisdiction(w, r, db, logger, child, lang)
			return
		}

		// /j/{state}/{topic}: a state playbook. When b is not in the topic
		// registry and is not a retired topic slug either, the reader more
		// likely asked for a place under this state that we have not covered;
		// tell them that rather than "no such topic".
		if _, terr := db.GetTopicBySlug(r.Context(), b); errors.Is(terr, store.ErrNotFound) {
			if _, aerr := db.ResolveTopicAlias(r.Context(), b); aerr != nil {
				render(w, r, http.StatusNotFound, tmpl.NotFoundPage{UncoveredPlace: true, Parent: parent, Language: lang})
				return
			}
		}
		servePlaybook(w, r, db, logger, parent, b, lang)
	}
}

// cityPlaybook serves /j/{state}/{city}/{topic} (or /es/j/{state}/{city}/{topic}, ...).
func cityPlaybook(db browseStore, logger *slog.Logger, lang string) http.HandlerFunc {
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
				http.Redirect(w, r, moved.TopicPathIn(lang, canonicalTopic(r.Context(), db, topicSlug)), http.StatusMovedPermanently) // #nosec G710
				return
			}
			// An unknown city under a known state is an uncovered place; the
			// state hub is the best page we can offer for it.
			page := tmpl.NotFoundPage{UncoveredPlace: true, Language: lang}
			if st, serr := db.GetJurisdictionBySlug(r.Context(), stateSlug); serr == nil {
				page.Parent = st
			}
			render(w, r, http.StatusNotFound, page)
			return
		}
		if city.ParentSlug != stateSlug {
			if target := city.TopicPathIn(lang, canonicalTopic(r.Context(), db, topicSlug)); target != r.URL.Path {
				// Not an open redirect: city and the canonical topic are both
				// database-resolved.
				http.Redirect(w, r, target, http.StatusMovedPermanently) // #nosec G710
				return
			}
			render(w, r, http.StatusNotFound, tmpl.NotFoundPage{Language: lang})
			return
		}
		servePlaybook(w, r, db, logger, city, topicSlug, lang)
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
		hubs, err := db.ListPublishedHubJurisdictions(r.Context())
		if err != nil {
			logger.ErrorContext(r.Context(), "list jurisdictions", slog.Any("err", err))
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		// National hubs get their own section; a state with statewide content
		// is not listed separately because its heading in the grouped city
		// index already links to its hub.
		var national, cities []store.Jurisdiction
		for _, j := range hubs {
			switch j.Kind {
			case "country":
				national = append(national, j)
			case "city":
				cities = append(cities, j)
			}
		}
		render(w, r, http.StatusOK, tmpl.LocationsPage{
			National:  national,
			Groups:    tmpl.GroupByState(cities),
			CityCount: len(cities),
		})
	}
}

// coverage serves /api/coverage?j={slug}: the topic slugs that resolve to a
// published guide for that location, walking up its ancestor chain the same
// way the ?j= redirect on /t/{topic} does. The homepage's situation list
// fetches this after a location is chosen and hides the situations that would
// fall through to the picker. It is a separate request on purpose: browse HTML
// is served from a shared public cache, so the page itself cannot be
// personalised, and shipping every location's coverage with the page would
// grow it with every city (see the scope-script comment in layout.html).
//
// English-only, like the homepage and search page that consume it (ADR-007 D2).
func coverage(db browseStore, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.URL.Query().Get("j")
		if slug == "" {
			http.Error(w, "missing j parameter", http.StatusBadRequest)
			return
		}
		j, err := db.GetJurisdictionBySlug(r.Context(), slug)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				http.NotFound(w, r)
				return
			}
			logger.ErrorContext(r.Context(), "coverage: get jurisdiction", slog.Any("err", err))
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		topics, err := db.ListTopicsByJurisdictionRecursive(r.Context(), j.ID, "en")
		if err != nil {
			logger.ErrorContext(r.Context(), "coverage: list topics", slog.Any("err", err))
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		slugs := make([]string, 0, len(topics))
		for _, t := range topics {
			slugs = append(slugs, t.Slug)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"j": j.Slug, "topics": slugs})
	}
}

// renderJurisdiction renders a hub page for a state, country, or city, in lang.
func renderJurisdiction(w http.ResponseWriter, r *http.Request, db browseStore, logger *slog.Logger, j store.Jurisdiction, lang string) {
	topics, err := db.ListTopicsByJurisdiction(r.Context(), j.ID, lang)
	if err != nil {
		logger.ErrorContext(r.Context(), "list topics", slog.Any("err", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// A state or country also lists the cities beneath it. Without this the
	// middle of the URL tree was unreachable: /j/massachusetts rendered "no
	// playbooks yet" while Boston sat directly under it. Best-effort, since a
	// missing city list should degrade the page rather than 500 it.
	//
	// Not language-scoped: a jurisdiction hub lists its child cities
	// regardless of what language they have content in yet, same as it always
	// listed them regardless of whether they had ANY playbook at all.
	var cities []store.Jurisdiction
	if j.Kind != "city" {
		if children, cerr := db.ListPublishedChildCities(r.Context(), j.ID); cerr == nil {
			cities = children
		} else {
			logger.ErrorContext(r.Context(), "list child cities", slog.Any("err", cerr))
		}
	}

	render(w, r, http.StatusOK, tmpl.JurisdictionPage{Jurisdiction: j, Topics: topics, Cities: cities, Language: lang})
}

// servePlaybook renders one playbook for an already-resolved jurisdiction in
// lang, or redirects when the topic slug was renamed. A translation that does
// not exist yet 404s here rather than falling back to another language — see
// ADR-007 D3.
func servePlaybook(w http.ResponseWriter, r *http.Request, db browseStore, logger *slog.Logger, j store.Jurisdiction, topicSlug, lang string) {
	pb, err := db.GetPlaybook(r.Context(), j.Slug, topicSlug, lang)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			if moved, aerr := db.ResolveTopicAlias(r.Context(), topicSlug); aerr == nil {
				// Not an open redirect: j is the resolved jurisdiction and
				// moved.Slug comes from the topic alias table.
				http.Redirect(w, r, j.TopicPathIn(lang, moved.Slug), http.StatusMovedPermanently) // #nosec G710
				return
			}
			// A registry topic this place has no guide for yet gets the
			// explainer 404, offering the nearest ancestor's guide when one
			// exists — an ancestor's law applies here too. It links rather
			// than redirects: this URL may gain its own page later, and a
			// cached redirect would keep stealing its traffic. The self check
			// is defensive only; GetPlaybook and the nearest walk both see
			// published rows only, so they cannot disagree about j itself.
			page := tmpl.NotFoundPage{Language: lang}
			if t, terr := db.GetTopicBySlug(r.Context(), topicSlug); terr == nil {
				page.Place = j
				page.Topic = t
				if dest, derr := db.GetNearestTopicJurisdiction(r.Context(), j.ID, t.ID, lang); derr == nil && dest.ID != j.ID {
					page.NearestPath = dest.TopicPathIn(lang, t.Slug)
					page.NearestName = dest.Name
				}
			}
			render(w, r, http.StatusNotFound, page)
			return
		}
		logger.ErrorContext(r.Context(), "get playbook", slog.Any("err", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// A national page links each tagged statement to the topic hub, where
	// every covered jurisdiction is listed (ADR-011 D4, amended): the reader
	// picks their place rather than scanning a per-place link row that would
	// outgrow the statement it hangs under. Offered only when somewhere other
	// than the national page itself covers the topic — a hub listing only the
	// page the reader is already on is a link to nowhere. Best-effort like
	// the cross-links below.
	specificsPath := ""
	if pb.Jurisdiction.Kind == "country" {
		if hubJs, herr := db.ListJurisdictionsByTopic(r.Context(), pb.Topic.ID, pb.Playbook.Language); herr == nil {
			for _, hj := range hubJs {
				if hj.Kind != "country" {
					specificsPath = store.LangPrefix(pb.Playbook.Language) + "/t/" + pb.Topic.Slug
					break
				}
			}
		} else {
			logger.ErrorContext(r.Context(), "list topic jurisdictions", slog.Any("err", herr))
		}
	}

	page := BuildPlaybookPage(r.Context(), pb, specificsPath, logger)

	// Cross-links: other topics in this jurisdiction, and this topic elsewhere
	// — both scoped to this playbook's own language (ADR-007 D5), so every
	// link built from them is guaranteed to resolve rather than 404.
	// Best-effort; a failure degrades the page rather than 500ing it.
	if topics, terr := db.ListTopicsByJurisdiction(r.Context(), pb.Jurisdiction.ID, pb.Playbook.Language); terr == nil {
		for _, t := range topics {
			if t.ID != pb.Topic.ID {
				page.SiblingTopics = append(page.SiblingTopics, t)
				// Local Help is not just another sibling. Someone reading about
				// eviction at 11pm may need a phone number more than the next
				// paragraph of law, and until now the only way to offer one was
				// to restate the whole referral list on every page. The link is
				// drawn from the same already-loaded list, so it costs no query
				// and cannot point at a page that is not published.
				if t.Slug == tmpl.LocalHelpTopic {
					page.LocalHelpPath = pb.Jurisdiction.TopicPathIn(pb.Playbook.Language, t.Slug)
					page.LocalHelpName = t.Name
				}
			}
		}
	} else {
		logger.ErrorContext(r.Context(), "list sibling topics", slog.Any("err", terr))
	}
	if cities, cerr := db.ListJurisdictionsByTopic(r.Context(), pb.Topic.ID, pb.Playbook.Language); cerr == nil {
		for _, c := range cities {
			if c.ID != pb.Jurisdiction.ID {
				page.OtherCities = append(page.OtherCities, c)
			}
		}
	} else {
		logger.ErrorContext(r.Context(), "list other cities", slog.Any("err", cerr))
	}

	// Language alternate: does this exact page exist in the other language?
	// (ADR-007 D6). Best-effort and silent on any error, same as the
	// cross-links above — a reader still gets the page either way.
	if other := otherLanguage(pb.Playbook.Language); other != "" {
		if op, oerr := db.GetPlaybook(r.Context(), j.Slug, topicSlug, other); oerr == nil {
			page.OtherLangPath = pb.Jurisdiction.TopicPathIn(other, op.Topic.Slug)
			page.OtherLangCode = other
			page.OtherLangLabel = otherLangLabel(other)
		}
	}
	switch {
	case pb.Playbook.Language == "en":
		page.XDefaultPath = page.Canonical
	case page.OtherLangPath != "":
		page.XDefaultPath = tmpl.BaseURL() + page.OtherLangPath
	default:
		page.XDefaultPath = page.Canonical
	}

	render(w, r, http.StatusOK, page)
}

// otherLanguage returns lang's counterpart for the language-toggle link,
// assuming exactly two supported languages (voice.Supported() is "en","es"
// today). Revisit this pairing if a third language is ever added.
func otherLanguage(lang string) string {
	switch lang {
	case "en":
		return "es"
	case "es":
		return "en"
	default:
		return ""
	}
}

// otherLangLabel is the toggle-link text, written IN the target language —
// the standard convention for language switchers, so a reader recognizes
// their own language rather than reading a sentence in the one they're
// trying to leave.
func otherLangLabel(lang string) string {
	switch lang {
	case "es":
		return "Ver esta página en español"
	case "en":
		return "Read this page in English"
	default:
		return "View in " + voice.Label(lang)
	}
}

// topicHub serves /t/{topic} (or /es/t/{topic}, ...): every place with a
// published playbook for this topic in lang. Like servePlaybook, a language
// with zero cities is not a public page (ADR-007 D3) — falls out of the
// existing len(cities)==0 check once cities is scoped to lang.
func topicHub(db browseStore, logger *slog.Logger, lang string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "topic")
		t, err := db.GetTopicBySlug(r.Context(), slug)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				// The 2026-08-01 topic cleanup retired slugs like
				// pittsburgh-discrimination; those URLs land here.
				if moved, aerr := db.ResolveTopicAlias(r.Context(), slug); aerr == nil {
					http.Redirect(w, r, store.LangPrefix(lang)+"/t/"+moved.Slug, http.StatusMovedPermanently)
					return
				}
				render(w, r, http.StatusNotFound, tmpl.NotFoundPage{Language: lang})
				return
			}
			logger.ErrorContext(r.Context(), "get topic", slog.Any("err", err))
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		cities, err := db.ListJurisdictionsByTopic(r.Context(), t.ID, lang)
		if err != nil {
			logger.ErrorContext(r.Context(), "list cities for topic", slog.Any("err", err))
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if len(cities) == 0 {
			// A topic with no published playbooks in this language is not a
			// public page, but the topic is real, so the 404 says "not written
			// yet" rather than "no such page".
			render(w, r, http.StatusNotFound, tmpl.NotFoundPage{Topic: t, Language: lang})
			return
		}

		// ?j= carries the reader's chosen location. Resolving it here rather
		// than in the browser is what lets the homepage link situations
		// straight into a city without shipping a city×topic map to every
		// visitor to avoid 404s. Resolution walks up the ancestor chain — the
		// city's own guide when it has one, else the state's, else the
		// national guide — the same upward-only rule that scopes search. A
		// location with nothing up its chain falls through to the list
		// instead of erroring. 302, not 301: coverage grows, so this mapping
		// must not be cached in browsers forever.
		if jSlug := r.URL.Query().Get("j"); jSlug != "" {
			if loc, jerr := db.GetJurisdictionBySlug(r.Context(), jSlug); jerr == nil {
				if dest, derr := db.GetNearestTopicJurisdiction(r.Context(), loc.ID, t.ID, lang); derr == nil {
					// Not an open redirect: dest comes from the jurisdictions
					// table and t from the topics table.
					http.Redirect(w, r, dest.TopicPathIn(lang, t.Slug), http.StatusFound) // #nosec G710
					return
				}
			}
		}

		// The query returns anywhere with this topic published, which includes
		// state and national guides. Split them: a state grouped by its parent
		// would file under "United States" and read as a city you could pick,
		// and a national guide labelled "Statewide" is simply wrong.
		var inCities, statewide, national []store.Jurisdiction
		for _, c := range cities {
			switch c.Kind {
			case "city":
				inCities = append(inCities, c)
			case "country":
				national = append(national, c)
			default:
				statewide = append(statewide, c)
			}
		}

		render(w, r, http.StatusOK, tmpl.TopicHubPage{
			Topic:     t,
			Groups:    tmpl.GroupByState(inCities),
			Statewide: statewide,
			National:  national,
			CityCount: len(inCities),
			Language:  lang,
		})
	}
}

func author(w http.ResponseWriter, r *http.Request) {
	render(w, r, http.StatusOK, tmpl.AuthorPage{})
}

// NotFound renders the styled 404 for URLs that match no route at all — the
// router's fallback. Browse handlers that know more (an uncovered place, a
// covered place missing one topic) build a richer tmpl.NotFoundPage inline.
func NotFound() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		render(w, r, http.StatusNotFound, tmpl.NotFoundPage{})
	}
}

// BuildPlaybookPage converts a stored playbook into the public page model,
// rendering markdown and enforcing the citation invariant. Shared with the
// authoring tool's draft preview so previews match the live site exactly.
//
// specificsPath is the topic-hub path a national page's tagged statements
// link to (ADR-011 D4, amended); pass "" for non-country pages or when no
// other jurisdiction covers the topic, and no link renders.
func BuildPlaybookPage(ctx context.Context, pb store.PlaybookWithStatements, specificsPath string, logger *slog.Logger) tmpl.PlaybookPage {
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
		stmtSpecifics := ""
		if s.ConceptSlug != "" {
			stmtSpecifics = specificsPath
		}
		statements = append(statements, tmpl.RenderedStatement{
			BodyHTML:      content.RenderMarkdown(s.BodyMD),
			Anchor:        s.ConceptSlug,
			SpecificsPath: stmtSpecifics,
			Citations:     chips,
		})
	}

	canonical := tmpl.BaseURL() + pb.Jurisdiction.TopicPathIn(pb.Playbook.Language, pb.Topic.Slug)

	var reviewedOn string
	switch {
	case pb.Playbook.LastReviewedAt != nil:
		reviewedOn = pb.Playbook.LastReviewedAt.Format("January 2, 2006")
	case !pb.Playbook.UpdatedAt.IsZero():
		reviewedOn = pb.Playbook.UpdatedAt.Format("January 2, 2006")
	}

	// The fallback description reads "...guide for {name}", which works for a
	// place name but not for the bare country ("guide for United States").
	descFor := pb.Jurisdiction.Name
	if pb.Jurisdiction.Kind == "country" {
		descFor = "renters anywhere in the United States"
	}

	return tmpl.PlaybookPage{
		Playbook:       pb.Playbook,
		Jurisdiction:   pb.Jurisdiction,
		Topic:          pb.Topic,
		IntroHTML:      introHTML,
		Statements:     statements,
		Description:    metaDescription(pb.IntroMD, pb.Playbook.Title, descFor),
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
			{Type: "ListItem", Position: 2, Name: pb.Jurisdiction.Name, Item: tmpl.BaseURL() + pb.Jurisdiction.PathIn(pb.Playbook.Language)},
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
