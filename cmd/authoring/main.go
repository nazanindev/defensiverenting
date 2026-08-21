// cmd/authoring is a small internal service for authoring new city playbooks.
// It writes directly to the shared Postgres DB.
// Auth: HTTP Basic Auth. Set AUTHORING_USER and AUTHORING_PASSWORD env vars.
// Run: authoring -addr :8081 -db $DATABASE_URL
package main

import (
	"context"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	dbpkg "github.com/nazanindev/defensiverenting/db"
	"github.com/nazanindev/defensiverenting/internal/discover"
	"github.com/nazanindev/defensiverenting/internal/draftagent"
	"github.com/nazanindev/defensiverenting/internal/drafting"
	sitehandlers "github.com/nazanindev/defensiverenting/internal/http/handlers"
	"github.com/nazanindev/defensiverenting/internal/sourcecheck"
	"github.com/nazanindev/defensiverenting/internal/store"
	"github.com/nazanindev/defensiverenting/internal/voice"
	webstatic "github.com/nazanindev/defensiverenting/web/static"
	sitetmpl "github.com/nazanindev/defensiverenting/web/templates"
)

//go:embed templates/*.html
var templateFS embed.FS

type srv struct {
	pg          *store.PG
	log         *slog.Logger
	tmpl        *template.Template
	jobs        *jobSet
	sourceCache *sourceFetchCache
}

// jobSet tracks in-flight AI draft generations by "city/topic" key, so the
// dashboard can show what's running and duplicate triggers are ignored.
type jobSet struct {
	mu sync.Mutex
	m  map[string]bool
}

func newJobSet() *jobSet { return &jobSet{m: map[string]bool{}} }

// start marks a job running; it returns false if one is already in flight.
func (j *jobSet) start(key string) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.m[key] {
		return false
	}
	j.m[key] = true
	return true
}

func (j *jobSet) done(key string) {
	j.mu.Lock()
	delete(j.m, key)
	j.mu.Unlock()
}

func (j *jobSet) list() []string {
	j.mu.Lock()
	defer j.mu.Unlock()
	out := make([]string, 0, len(j.m))
	for k := range j.m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func noindex(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Robots-Tag", "noindex, nofollow")
		next.ServeHTTP(w, r)
	})
}

func basicAuth(user, pass string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok ||
			subtle.ConstantTimeCompare([]byte(u), []byte(user)) != 1 ||
			subtle.ConstantTimeCompare([]byte(p), []byte(pass)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Defensive Renting Authoring"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	addr := flag.String("addr", ":8081", "listen address")
	dsn := flag.String("db", os.Getenv("DATABASE_URL"), "Postgres DSN")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if *dsn == "" {
		log.Error("DATABASE_URL or -db flag required")
		os.Exit(1)
	}

	authUser := os.Getenv("AUTHORING_USER")
	authPass := os.Getenv("AUTHORING_PASSWORD")
	if authUser == "" || authPass == "" {
		log.Error("AUTHORING_USER and AUTHORING_PASSWORD env vars are required")
		os.Exit(1)
	}

	ctx := context.Background()
	pg, err := store.New(ctx, *dsn)
	if err != nil {
		log.Error("connect db", slog.Any("err", err))
		os.Exit(2)
	}
	defer pg.Close()

	if err := dbpkg.Migrate(ctx, pg.Pool()); err != nil {
		log.Error("migrate", slog.Any("err", err))
		os.Exit(2)
	}

	tmpl, err := template.New("").Funcs(template.FuncMap{"date": fmtDate}).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		log.Error("parse templates", slog.Any("err", err))
		os.Exit(1)
	}

	s := &srv{pg: pg, log: log, tmpl: tmpl, jobs: newJobSet(), sourceCache: newSourceFetchCache(2 * time.Minute)}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.dashboard)
	mux.HandleFunc("GET /guidelines", s.guidelines)
	mux.HandleFunc("GET /new", s.showForm)
	mux.HandleFunc("POST /new", s.submitForm)
	mux.HandleFunc("GET /edit/{id}", s.showEditForm)
	mux.HandleFunc("POST /edit/{id}", s.submitEditForm)
	mux.HandleFunc("GET /view/{id}", s.viewPlaybook)
	mux.HandleFunc("GET /preview/{id}", s.previewPlaybook)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(webstatic.Files))))
	mux.HandleFunc("POST /generate", s.generateDraft)
	mux.HandleFunc("POST /check-sources", s.checkSources)
	mux.HandleFunc("POST /sources/{id}/dismiss-flag", s.dismissSourceFlag)
	mux.HandleFunc("POST /publish/{id}", s.publish)
	mux.HandleFunc("POST /unpublish/{id}", s.unpublish)
	mux.HandleFunc("GET /api/sources/{id}", s.sourcesJSON)
	mux.HandleFunc("POST /api/check-quote", s.checkQuoteLive)
	mux.HandleFunc("GET /api/source-text", s.sourceText)
	mux.HandleFunc("POST /delete/{id}", s.delete)
	mux.HandleFunc("POST /discover", s.discover)
	mux.HandleFunc("GET /candidates", s.candidates)
	mux.HandleFunc("POST /candidates/{id}/approve", s.approveCandidate)
	mux.HandleFunc("POST /candidates/{id}/reject", s.rejectCandidate)
	mux.HandleFunc("POST /candidates/{id}/snooze", s.snoozeCandidate)

	// /healthz is outside the auth wrapper so Fly's health check can reach it.
	outer := http.NewServeMux()
	outer.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// This is an internal tool on a public hostname: refuse indexing outright.
	outer.HandleFunc("GET /robots.txt", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("User-agent: *\nDisallow: /\n"))
	})
	outer.Handle("/", noindex(basicAuth(authUser, authPass, mux)))

	httpSrv := &http.Server{Addr: *addr, Handler: outer, ReadHeaderTimeout: 5 * time.Second}

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
		<-sig
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutCtx)
	}()

	log.Info("authoring started", slog.String("addr", *addr))
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("serve", slog.Any("err", err))
		os.Exit(1)
	}
}

// ---- handlers ---------------------------------------------------------------

func (s *srv) guidelines(w http.ResponseWriter, r *http.Request) {
	s.render(w, "guidelines.html", nil)
}

// dashboardView is the sort and filter state of the page list.
//
// It is remembered in a cookie because the review loop constantly leaves and
// comes back: publishing, deleting and saving all redirect to "/", and losing
// the sort on every action makes working steadily through one slice of the
// queue impossible. An explicit query parameter always beats the remembered
// value, so a link still means exactly what it says.
type dashboardView struct {
	Status string
	Sort   string
	Dir    string
	// Place is a jurisdiction slug at any level. Selecting a state shows the
	// cities under it, not just pages scoped to the state itself, because
	// "show me Massachusetts" means the work for Massachusetts.
	Place string
}

const viewCookie = "authoring_view"

// sortableColumns is the allowlist. The sort key reaches a switch rather than
// any query, but it also round-trips through a cookie, so it is bounded here
// instead of trusted.
var sortableColumns = map[string]bool{
	"city": true, "title": true, "topic": true, "kind": true,
	"status": true, "updated": true, "created": true, "size": true,
}

func (v dashboardView) normalize() dashboardView {
	if v.Status != "draft" && v.Status != "published" && v.Status != "superseded" {
		v.Status = "all"
	}
	if !sortableColumns[v.Sort] {
		v.Sort = "updated"
	}
	if v.Dir != "asc" && v.Dir != "desc" {
		// Dates default newest-first, which is what "what moved lately" means.
		// Everything else defaults A to Z.
		v.Dir = "asc"
		if v.Sort == "updated" || v.Sort == "created" {
			v.Dir = "desc"
		}
	}
	return v
}

func (v dashboardView) query() string {
	q := url.Values{"status": {v.Status}, "sort": {v.Sort}, "dir": {v.Dir}}
	if v.Place != "" {
		q.Set("place", v.Place)
	}
	return q.Encode()
}

// PlaceLink returns the href for filtering to one location, keeping the current
// status and sort. An empty slug clears the filter.
func (v dashboardView) PlaceLink(slug string) template.URL {
	next := v
	next.Place = slug
	return template.URL("/?" + next.normalize().query()) // #nosec G203
}

// SortLink returns the full href for sorting by col: the same column reversed,
// or a fresh column at its natural direction.
//
// It returns template.URL rather than a string because html/template treats a
// bare value after "/?" as a single query parameter and escapes the "=" and "&"
// inside it, which silently turns every sort link into one meaningless
// parameter. Marking it trusted is safe because status, sort and dir come from
// fixed allowlists in normalize(), and place — which is a slug, so no allowlist
// can be written here — is percent-encoded by url.Values.Encode before it
// reaches the URL. The dashboard also drops a place it does not recognise.
//
// Place is carried through: sorting a filtered list must not silently widen it
// back to every city.
func (v dashboardView) SortLink(col string) template.URL {
	next := dashboardView{Status: v.Status, Sort: col, Place: v.Place}
	if v.Sort == col {
		next.Dir = "asc"
		if v.Dir == "asc" {
			next.Dir = "desc"
		}
	}
	return template.URL("/?" + next.normalize().query()) // #nosec G203
}

// Arrow marks the sorted column in the header.
func (v dashboardView) Arrow(col string) string {
	if v.Sort != col {
		return ""
	}
	if v.Dir == "asc" {
		return "▲"
	}
	return "▼"
}

func readView(r *http.Request) dashboardView {
	q := r.URL.Query()
	v := dashboardView{Status: q.Get("status"), Sort: q.Get("sort"), Dir: q.Get("dir"), Place: q.Get("place")}
	if v.Status == "" && v.Sort == "" && v.Dir == "" && q.Get("place") == "" {
		if c, err := r.Cookie(viewCookie); err == nil {
			if remembered, err := url.ParseQuery(c.Value); err == nil {
				v = dashboardView{
					Status: remembered.Get("status"),
					Sort:   remembered.Get("sort"),
					Dir:    remembered.Get("dir"),
					Place:  remembered.Get("place"),
				}
			}
		}
	}
	return v.normalize()
}

// sortPlaybooks orders rows in place. SliceStable so that re-sorting on a
// column with many equal values (status, kind) keeps the previous order inside
// each group rather than reshuffling rows the author was reading.
func sortPlaybooks(rows []store.AuthorPlaybookRow, key, dir string) {
	// Drafts first: the list is a review queue, so "status ascending" should
	// mean most-needing-attention rather than alphabetical.
	rank := map[string]int{"draft": 0, "published": 1, "superseded": 2}
	less := func(a, b store.AuthorPlaybookRow) bool {
		switch key {
		case "city":
			return strings.ToLower(a.JurisdictionName) < strings.ToLower(b.JurisdictionName)
		case "title":
			return strings.ToLower(a.Title) < strings.ToLower(b.Title)
		case "topic":
			return a.TopicSlug < b.TopicSlug
		case "kind":
			return a.PageKind < b.PageKind
		case "status":
			return rank[a.Status] < rank[b.Status]
		case "created":
			return a.CreatedAt.Before(b.CreatedAt)
		case "size":
			return a.StatementCount < b.StatementCount
		default:
			return a.UpdatedAt.Before(b.UpdatedAt)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if dir == "desc" {
			return less(rows[j], rows[i])
		}
		return less(rows[i], rows[j])
	})
}

func (s *srv) dashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	playbooks, err := s.pg.AuthorListPlaybooks(ctx)
	if err != nil {
		s.serverError(w, err)
		return
	}
	cities, err := s.pg.ListCityJurisdictions(ctx)
	if err != nil {
		s.serverError(w, err)
		return
	}
	counts, err := s.pg.CandidateCounts(ctx)
	if err != nil {
		s.serverError(w, err)
		return
	}
	flagged, err := s.pg.ListFlaggedSources(ctx)
	if err != nil {
		s.serverError(w, err)
		return
	}
	coverage, err := s.pg.AuthorCoverage(ctx)
	if err != nil {
		s.serverError(w, err)
		return
	}
	unused, err := s.pg.ListUnusedSources(ctx)
	if err != nil {
		s.serverError(w, err)
		return
	}
	coreTopics, err := s.pg.ListCoreTopics(ctx)
	if err != nil {
		s.serverError(w, err)
		return
	}

	// Drafts and published pages share one table, so a review pass through a
	// batch of drafts otherwise means scrolling past every live page.
	var draftCount, publishedCount, supersededCount int
	langs := map[string]bool{}
	for _, p := range playbooks {
		langs[p.Language] = true
		switch p.Status {
		case "draft":
			draftCount++
		case "superseded":
			supersededCount++
		default:
			publishedCount++
		}
	}

	view := readView(r)

	// Location filter. A page belongs to a place if that place is anywhere in
	// its jurisdiction's ancestry, so Massachusetts shows Boston's pages and
	// the United States shows everything under it. Filtering on the slug alone
	// would make a state select only pages scoped to the state itself, which is
	// almost never what someone means by "show me Massachusetts".
	places, parentOf := s.placeOptions(ctx, playbooks)
	if view.Place != "" && !places.known[view.Place] {
		view.Place = "" // a stale cookie or a hand-edited URL must not hide everything
	}
	if view.Place != "" {
		kept := make([]store.AuthorPlaybookRow, 0, len(playbooks))
		for _, p := range playbooks {
			for slug := p.JurisdictionSlug; slug != ""; slug = parentOf[slug] {
				if slug == view.Place {
					kept = append(kept, p)
					break
				}
			}
		}
		playbooks = kept
	}

	if view.Status != "all" {
		kept := make([]store.AuthorPlaybookRow, 0, len(playbooks))
		for _, p := range playbooks {
			if p.Status == view.Status {
				kept = append(kept, p)
			}
		}
		playbooks = kept
	}
	sortPlaybooks(playbooks, view.Sort, view.Dir)

	// Secure is set unconditionally rather than sniffed from the request: Fly
	// terminates TLS upstream, so r.TLS is nil in production and any check
	// based on it would disable the flag exactly where it matters. Browsers
	// treat http://localhost as a secure context, so local development still
	// stores it.
	http.SetCookie(w, &http.Cookie{
		Name: viewCookie, Value: view.query(), Path: "/",
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode, MaxAge: 30 * 24 * 3600,
	})

	s.render(w, "dashboard.html", map[string]any{
		"Playbooks":       playbooks,
		"Cities":          cities,
		"ReviewCounts":    counts,
		"Generating":      s.jobs.list(),
		"Flagged":         flagged,
		"Unused":          unused,
		"View":            view,
		"Places":          places.Opts(),
		"Status":          view.Status,
		"ShowLanguage":    len(langs) > 1,
		"Coverage":        coverage,
		"CoreTopics":      coreTopics,
		"DraftCount":      draftCount,
		"PublishedCount":  publishedCount,
		"TotalCount":      draftCount + publishedCount + supersededCount,
		"SupersededCount": supersededCount,
		"Msg":             r.URL.Query().Get("msg"),
		"Err":             r.URL.Query().Get("err"),
	})
}

// generateDraft kicks off an AI drafting run for a city+topic in a background
// goroutine (it takes minutes) and returns immediately. The draft appears in the
// dashboard when the run finishes; progress is logged server-side.
func (s *srv) generateDraft(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}
	topic := toSlug(strings.TrimSpace(r.FormValue("topic_slug")))

	redirect := func(param, msg string) {
		http.Redirect(w, r, "/?"+param+"="+url.QueryEscape(msg), http.StatusSeeOther)
	}
	if topic == "" {
		redirect("err", "enter a topic slug")
		return
	}
	// Topics are a closed registry; drafting may not invent one.
	registryTopic, err := s.pg.GetTopicBySlug(r.Context(), topic)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			redirect("err", "unknown topic "+topic+" — pick one that already exists")
			return
		}
		s.serverError(w, err)
		return
	}
	topicName := registryTopic.Name
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		redirect("err", "ANTHROPIC_API_KEY is not set on the authoring server — cannot generate")
		return
	}

	jur, userErr, err := s.resolveCity(r.Context(), r)
	if err != nil {
		s.serverError(w, err)
		return
	}
	if userErr != "" {
		redirect("err", userErr)
		return
	}
	city := jur.Slug

	key := city + "/" + topic
	if !s.jobs.start(key) {
		redirect("msg", "already generating "+key)
		return
	}
	// A drafting run outlives the request that started it, so binding it to the
	// request context would cancel the job the moment the browser got its
	// redirect. context.Background is deliberate here.
	go func() { // #nosec G118
		defer s.jobs.done(key)
		report, err := draftagent.Run(context.Background(), drafting.New(s.pg), draftagent.Options{
			JurisdictionSlug: city,
			TopicSlug:        topic,
			TopicName:        topicName,
			Log: func(format string, a ...any) {
				s.log.Info("draftgen", slog.String("job", key), slog.String("msg", fmt.Sprintf(format, a...)))
			},
		})
		if err != nil {
			s.log.Error("draftgen failed", slog.String("job", key), slog.Any("err", err), slog.Int("steps", report.Steps))
			return
		}
		s.log.Info("draftgen saved", slog.String("job", key),
			slog.Int("steps", report.Steps),
			slog.Int64("input_tokens", report.Usage.InputTokens),
			slog.Int64("output_tokens", report.Usage.OutputTokens))
	}()
	redirect("msg", "generating draft for "+key+" — refresh in a minute to see it")
}

// checkSources re-fetches every cited source in the background and flags any
// whose content changed since last review. No AI, no cost.
func (s *srv) checkSources(w http.ResponseWriter, r *http.Request) {
	key := "sources-check"
	if !s.jobs.start(key) {
		http.Redirect(w, r, "/?msg="+url.QueryEscape("a source check is already running"), http.StatusSeeOther)
		return
	}
	go func() {
		defer s.jobs.done(key)
		res, err := sourcecheck.Run(context.Background(), s.pg, drafting.FetchExtract, func(format string, a ...any) {
			s.log.Info("sourcecheck", slog.String("msg", fmt.Sprintf(format, a...)))
		})
		if err != nil {
			s.log.Error("sourcecheck failed", slog.Any("err", err))
			return
		}
		s.log.Info("sourcecheck done",
			slog.Int("sources", res.Sources), slog.Int("flagged", res.Flagged), slog.Int("failed", res.Failed))
	}()
	http.Redirect(w, r, "/?msg="+url.QueryEscape("re-checking sources for changes — refresh in a moment"), http.StatusSeeOther)
}

// dismissSourceFlag clears a source's change flag after the author reviews it.
func (s *srv) dismissSourceFlag(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := s.pg.DismissSourceFlag(r.Context(), id); err != nil {
		s.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/?msg="+url.QueryEscape("flag dismissed"), http.StatusSeeOther)
}

// ---- source discovery -------------------------------------------------------

// discover resolves the target jurisdiction (an existing city, or a new
// city+state typed into the form), runs the registry providers, and stores the
// resulting candidates for the author to triage. Discovery only seeds the
// research shelf — nothing here writes statements or citations.
func (s *srv) discover(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}
	j, userErr, err := s.resolveCity(ctx, r)
	if err != nil {
		s.serverError(w, err)
		return
	}
	if userErr != "" {
		http.Error(w, userErr, http.StatusBadRequest)
		return
	}
	cands := discover.Run(j.Slug, discover.DefaultProviders()...)
	if _, err := s.pg.InsertCandidates(ctx, j.ID, cands); err != nil {
		s.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/candidates?j="+url.QueryEscape(j.Slug), http.StatusSeeOther)
}

// resolveCity returns the target city jurisdiction from the form: an existing
// city (jurisdiction_select), or a new city+state (new_city_name/new_state_name)
// that it creates. A non-empty userErr is a bad-input message for the caller to
// surface; err is a real failure.
func (s *srv) resolveCity(ctx context.Context, r *http.Request) (store.Jurisdiction, string, error) {
	newCity := strings.TrimSpace(r.FormValue("new_city_name"))
	newState := strings.TrimSpace(r.FormValue("new_state_name"))
	if newCity != "" {
		if newState == "" {
			return store.Jurisdiction{}, "State name is required for a new city.", nil
		}
		state, err := s.pg.UpsertJurisdiction(ctx, store.UpsertJurisdictionParams{
			Kind: "state", Name: newState, Slug: toSlug(newState),
		})
		if err != nil {
			return store.Jurisdiction{}, "", err
		}
		city, err := s.pg.UpsertJurisdiction(ctx, store.UpsertJurisdictionParams{
			ParentID: &state.ID, Kind: "city", Name: newCity, Slug: toSlug(newCity),
		})
		return city, "", err
	}
	slug := strings.TrimSpace(r.FormValue("jurisdiction_select"))
	if slug == "" || slug == "new" {
		return store.Jurisdiction{}, "Select a city or enter a new one.", nil
	}
	city, err := s.pg.GetJurisdictionBySlug(ctx, slug)
	return city, "", err
}

// candidateView decorates a stored candidate with display-only fields.
type candidateView struct {
	store.SourceCandidate
	ConfidencePct int
}

func (s *srv) candidates(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	slug := r.URL.Query().Get("j")
	if slug == "" {
		http.Error(w, "missing ?j=<city-slug>", http.StatusBadRequest)
		return
	}
	j, err := s.pg.GetJurisdictionBySlug(ctx, slug)
	if err != nil {
		s.serverError(w, err)
		return
	}
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "pending"
	}
	cands, err := s.pg.ListCandidates(ctx, j.ID, status)
	if err != nil {
		s.serverError(w, err)
		return
	}
	views := make([]candidateView, len(cands))
	for i, c := range cands {
		views[i] = candidateView{SourceCandidate: c, ConfidencePct: int(c.Confidence*100 + 0.5)}
	}
	s.render(w, "candidates.html", map[string]any{
		"City":       j,
		"Status":     status,
		"Candidates": views,
	})
}

// approveCandidate promotes a candidate into a real (uncited) sources row, ready
// to attach when the author writes statements, then marks it approved.
func (s *srv) approveCandidate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	c, err := s.pg.GetCandidate(ctx, id)
	if err != nil {
		s.serverError(w, err)
		return
	}
	src, err := s.pg.UpsertSource(ctx, store.UpsertSourceParams{
		URL:            c.URL,
		Publisher:      c.Publisher,
		JurisdictionID: c.JurisdictionID,
		Kind:           c.KindGuess,
	})
	if err != nil {
		s.serverError(w, err)
		return
	}
	if err := s.pg.SetCandidateStatus(ctx, id, "approved", &src.ID); err != nil {
		s.serverError(w, err)
		return
	}
	s.redirectToCandidates(w, r)
}

func (s *srv) rejectCandidate(w http.ResponseWriter, r *http.Request) {
	s.triageCandidate(w, r, "rejected")
}

func (s *srv) snoozeCandidate(w http.ResponseWriter, r *http.Request) {
	s.triageCandidate(w, r, "snoozed")
}

func (s *srv) triageCandidate(w http.ResponseWriter, r *http.Request, status string) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := s.pg.SetCandidateStatus(r.Context(), id, status, nil); err != nil {
		s.serverError(w, err)
		return
	}
	s.redirectToCandidates(w, r)
}

// redirectToCandidates returns to the candidate queue for the city carried in
// the form's hidden "j" field (falls back to the dashboard).
func (s *srv) redirectToCandidates(w http.ResponseWriter, r *http.Request) {
	slug := r.FormValue("j")
	if slug == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/candidates?j="+url.QueryEscape(slug), http.StatusSeeOther)
}

func (s *srv) showForm(w http.ResponseWriter, r *http.Request) {
	jurisdictions, err := s.pg.ListAuthorableJurisdictions(r.Context())
	if err != nil {
		s.serverError(w, err)
		return
	}
	topics, err := s.pg.ListTopicRegistry(r.Context())
	if err != nil {
		s.serverError(w, err)
		return
	}
	data := map[string]any{
		"Jurisdictions": jurisdictions, "Topics": topics, "Error": "",
		"PreloadJSON":      template.JS("null"),
		"ImportGroups":     s.importablePages(r.Context(), 0, ""),
		"Languages":        languageOptions(),
		"SelectedLanguage": "en",
	}

	// ?from=<id> pre-fills the form from an existing playbook as a reference
	if fromID := r.URL.Query().Get("from"); fromID != "" {
		id, err := strconv.ParseInt(fromID, 10, 64)
		if err == nil {
			if pw, err := s.pg.AuthorGetPlaybook(r.Context(), id); err == nil {
				editorial, _ := s.pg.GetEditorialSource(r.Context())
				pj, _ := json.Marshal(buildPreload(pw, editorial.ID))
				//nolint:gosec // pj is json.Marshal output of an internal struct, not user input
				data["PreloadJSON"] = template.JS(pj)
			}
		}
	}

	s.render(w, "form.html", data)
}

// fmtDate renders a timestamp (time.Time or *time.Time) for the authoring UI;
// nil/zero shows an em dash.
func fmtDate(v any) string {
	var t time.Time
	switch x := v.(type) {
	case time.Time:
		t = x
	case *time.Time:
		if x == nil {
			return "—"
		}
		t = *x
	default:
		return "—"
	}
	if t.IsZero() {
		return "—"
	}
	return t.Format("Jan 2, 2006")
}

// hostOf returns the bare host (minus a leading www.) of a URL, for compact
// display next to a citation quote.
func hostOf(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return strings.TrimPrefix(u.Host, "www.")
	}
	return raw
}

func (s *srv) viewPlaybook(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	pw, err := s.pg.AuthorGetPlaybook(r.Context(), id)
	if err != nil {
		s.serverError(w, err)
		return
	}
	editorial, _ := s.pg.GetEditorialSource(r.Context())

	// Deduplicate sources across all statements for the reference display
	type srcMeta struct {
		Idx       int
		URL       string
		Publisher string
		Kind      string
	}
	srcByID := map[int64]srcMeta{}
	var srcOrder []int64
	for _, stmt := range pw.Statements {
		for _, c := range stmt.Citations {
			if c.SourceID == editorial.ID {
				continue
			}
			if _, ok := srcByID[c.SourceID]; !ok {
				srcByID[c.SourceID] = srcMeta{
					Idx:       len(srcOrder),
					URL:       c.SourceURL,
					Publisher: c.Publisher,
					Kind:      c.SourceKind,
				}
				srcOrder = append(srcOrder, c.SourceID)
			}
		}
	}

	var sources []viewSource
	for _, id := range srcOrder {
		m := srcByID[id]
		sources = append(sources, viewSource{Num: m.Idx + 1, URL: m.URL, Publisher: m.Publisher, Kind: m.Kind})
	}

	var stmts []viewStmt
	for i, stmt := range pw.Statements {
		vs := viewStmt{Num: i + 1, Body: stmt.BodyMD}
		for _, c := range stmt.Citations {
			if c.SourceID == editorial.ID {
				vs.Editorial = true
			} else if m, ok := srcByID[c.SourceID]; ok {
				vs.Cites = append(vs.Cites, viewCite{
					Num: m.Idx + 1, Locator: c.Locator,
					Quote: c.Quote, URL: c.SourceURL, Domain: hostOf(c.SourceURL),
					CheckedAt: c.CheckedAt,
				})
			}
		}
		vs.CheckedAt, vs.Unchecked = stmtCheckedAt(vs.Cites)
		stmts = append(stmts, vs)
	}

	// A directory lists organisations, and one organisation takes several
	// statements: the number, the hours, who qualifies, what it will not do.
	// Numbered one per row they read as fragments — "Ask early, the wait can
	// exceed 21 days" means nothing without the organisation above it — so the
	// verify view groups them the way the live page does. Grouping is by
	// consecutive shared source because on a directory the source is the
	// organisation. See web/templates.groupByOrg.
	publisherOf := make(map[string]string, len(sources))
	for _, src := range sources {
		publisherOf[src.URL] = src.Publisher
	}
	groups := groupViewStmts(stmts, pw.Playbook.PageKind == "directory", publisherOf)

	s.render(w, "view.html", map[string]any{
		"Playbook": pw,
		"Sources":  sources,
		"Stmts":    stmts,
		"Groups":   groups,
		"Grouped":  pw.Playbook.PageKind == "directory",
	})
}

// The verify view's shapes. Package level rather than inside the handler so
// grouping can be a plain function over them.
type viewSource struct {
	Num       int
	URL       string
	Publisher string
	Kind      string
}

type viewCite struct {
	Num       int
	Locator   string
	Quote     string
	URL       string
	Domain    string
	CheckedAt *time.Time // when the quote was last confirmed at the source; nil = never
}

type viewStmt struct {
	Num       int
	Body      string
	Cites     []viewCite
	Editorial bool
	// CheckedAt is when the statement was last fully checked: the oldest
	// confirmation among its citations, since a statement is only as current
	// as its least-recently-confirmed evidence. Unchecked is set instead when
	// any citation has never been confirmed. Editorial-only statements carry
	// neither: they cite no external text to check.
	CheckedAt *time.Time
	Unchecked bool
}

// stmtCheckedAt derives a statement's last-checked stamp from its citations.
func stmtCheckedAt(cites []viewCite) (*time.Time, bool) {
	var oldest *time.Time
	for _, c := range cites {
		if c.CheckedAt == nil {
			return nil, true
		}
		if oldest == nil || c.CheckedAt.Before(*oldest) {
			oldest = c.CheckedAt
		}
	}
	return oldest, false
}

// viewGroup is a run of statements shown together under one heading.
type viewGroup struct {
	Heading string // the organisation, empty when not grouping
	Stmts   []viewStmt
}

// groupViewStmts collapses consecutive statements that cite the same source,
// heading each run with that source's publisher. When grouping is off every
// statement is its own group, so the template renders one shape either way.
func groupViewStmts(stmts []viewStmt, grouped bool, publisherOf map[string]string) []viewGroup {
	out := make([]viewGroup, 0, len(stmts))
	lastKey := ""
	for _, st := range stmts {
		key := ""
		if len(st.Cites) > 0 {
			key = st.Cites[0].URL
		}
		if grouped && key != "" && key == lastKey && len(out) > 0 {
			out[len(out)-1].Stmts = append(out[len(out)-1].Stmts, st)
			continue
		}
		lastKey = key
		g := viewGroup{Stmts: []viewStmt{st}}
		if grouped {
			if p := publisherOf[key]; p != "" {
				g.Heading = p
			} else {
				g.Heading = hostOf(key)
			}
		}
		out = append(out, g)
	}
	return out
}

// previewPlaybook renders a playbook (draft or published) through the live
// site's own template, so the author sees exactly what publishing would produce.
func (s *srv) previewPlaybook(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	pw, err := s.pg.AuthorGetPlaybook(r.Context(), id)
	if err != nil {
		s.serverError(w, err)
		return
	}
	page := sitehandlers.BuildPlaybookPage(r.Context(), pw, s.log)
	page.Preview = true
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := sitetmpl.Render(w, page); err != nil {
		s.log.Error("render preview", slog.Any("err", err))
	}
}

func (s *srv) submitForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		s.formError(w, r, "Invalid form data: "+err.Error())
		return
	}

	// Resolve jurisdiction
	jSlug := r.FormValue("jurisdiction_select")
	var j store.Jurisdiction
	var citySlug string
	var err error
	if jSlug == "new" {
		cityName := strings.TrimSpace(r.FormValue("new_city_name"))
		stateName := strings.TrimSpace(r.FormValue("new_state_name"))
		if cityName == "" || stateName == "" {
			s.formError(w, r, "City name and state name are required for a new city")
			return
		}
		citySlug = toSlug(cityName)
		stateSlug := toSlug(stateName)
		state, err := s.pg.UpsertJurisdiction(ctx, store.UpsertJurisdictionParams{
			Kind: "state", Name: stateName, Slug: stateSlug,
		})
		if err != nil {
			s.formError(w, r, "Failed to create state: "+err.Error())
			return
		}
		j, err = s.pg.UpsertJurisdiction(ctx, store.UpsertJurisdictionParams{
			ParentID: &state.ID, Kind: "city", Name: cityName, Slug: citySlug,
		})
		if err != nil {
			s.formError(w, r, "Failed to create city: "+err.Error())
			return
		}
	} else {
		j, err = s.pg.GetJurisdictionBySlug(ctx, jSlug)
		if err != nil {
			s.formError(w, r, "Unknown jurisdiction selected")
			return
		}
	}

	topic, msg := s.resolveTopic(ctx, r)
	if msg != "" {
		s.formError(w, r, msg)
		return
	}
	topicSlug := topic.Slug

	lang, err := drafting.ResolveLanguage(r.FormValue("language"))
	if err != nil {
		s.formError(w, r, err.Error())
		return
	}

	// Parse sources
	srcIndices := parseIndices(r.FormValue("active_sources"))
	sourceByIdx := make(map[int]store.Source, len(srcIndices))
	srcLocatorByIdx := make(map[int]string, len(srcIndices))
	for _, i := range srcIndices {
		u := strings.TrimSpace(r.FormValue(fmt.Sprintf("src_url_%d", i)))
		pub := strings.TrimSpace(r.FormValue(fmt.Sprintf("src_pub_%d", i)))
		knd := r.FormValue(fmt.Sprintf("src_kind_%d", i))
		loc := r.FormValue(fmt.Sprintf("src_loc_%d", i))
		if u == "" || pub == "" {
			s.formError(w, r, fmt.Sprintf("Source %d is missing a URL or publisher", i+1))
			return
		}
		src, err := s.pg.UpsertSource(ctx, store.UpsertSourceParams{
			URL: u, Publisher: pub, Kind: knd,
		})
		if err != nil {
			s.formError(w, r, "Failed to save source: "+err.Error())
			return
		}
		sourceByIdx[i] = src
		srcLocatorByIdx[i] = loc
	}

	editorialSrc, err := s.pg.GetEditorialSource(ctx)
	if err != nil {
		s.formError(w, r, "Editorial source not found — run migrations first")
		return
	}

	// Parse statements
	stmtIndices := parseIndices(r.FormValue("active_stmts"))
	qv := newQuoteVerifier(s.pg, s.sourceCache)
	var statements []store.IngestStatementParams
	for _, ji := range stmtIndices {
		body := strings.TrimSpace(r.FormValue(fmt.Sprintf("stmt_%d", ji)))
		if body == "" {
			continue
		}
		var cites []store.IngestCitationParams
		for _, i := range srcIndices {
			if r.FormValue(fmt.Sprintf("cite_%d_%d", ji, i)) == "on" {
				loc := r.FormValue(fmt.Sprintf("loc_%d_%d", ji, i))
				if loc == "" {
					loc = srcLocatorByIdx[i]
				}
				quote := r.FormValue(fmt.Sprintf("quote_%d_%d", ji, i))
				overridden := r.FormValue(fmt.Sprintf("verify_override_%d_%d", ji, i)) == "on"
				res := qv.check(ctx, ji+1, sourceByIdx[i].URL, quote)
				if res.Msg != "" && (!res.Overridable || !overridden) {
					s.formError(w, r, res.Msg)
					return
				}
				// Quote is carried through the form read-only. Omitting it here
				// is what rewrote every citation's quote to "" on save.
				cites = append(cites, store.IngestCitationParams{
					SourceID:         sourceByIdx[i].ID,
					Locator:          loc,
					Quote:            quote,
					ManuallyVerified: overridden,
					// A live fetch confirmed the quote just now, or the
					// reviewer attested to it by hand just now. A known-quote
					// skip is neither, and inherits its earlier stamp.
					CheckedNow: res.Verified || overridden,
				})
			}
		}
		if r.FormValue(fmt.Sprintf("edit_%d", ji)) == "on" {
			cites = append(cites, store.IngestCitationParams{SourceID: editorialSrc.ID})
		}
		if len(cites) == 0 {
			s.formError(w, r, fmt.Sprintf("Statement %d needs at least one citation", ji+1))
			return
		}
		statements = append(statements, store.IngestStatementParams{
			BodyMD: body, Language: lang, Sources: cites,
		})
	}
	if len(statements) == 0 {
		s.formError(w, r, "At least one statement is required")
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		s.formError(w, r, "Title is required")
		return
	}

	if err := s.pg.IngestPlaybook(ctx, store.IngestPlaybookParams{
		JurisdictionID: j.ID,
		TopicID:        topic.ID,
		Language:       lang,
		Slug:           topicSlug,
		Title:          title,
		IntroMD:        strings.TrimSpace(r.FormValue("intro")),
		PageKind:       r.FormValue("page_kind"),
		Statements:     statements,
		Status:         "draft",
	}); err != nil {
		s.formError(w, r, "Failed to save playbook: "+err.Error())
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *srv) publish(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := s.pg.AuthorPublishPlaybook(r.Context(), id); err != nil {
		s.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// placeOption is one entry in the location filter.
type placeOption struct {
	Slug  string
	Name  string
	Kind  string // country|state|city, used to indent the list
	Count int    // pages at or under this place
}

type placeList struct {
	Options []placeOption
	known   map[string]bool
}

// Opts exposes the options to the template; known is for validation only.
func (p placeList) Opts() []placeOption { return p.Options }

// placeOptions builds the location filter from the jurisdictions that actually
// have pages, plus their ancestors, ordered country → state → city.
//
// Only places with pages are offered. The jurisdictions table carries rows
// created for hierarchy repair and for cities seeded but never written, and a
// filter listing options that select nothing is worse than no filter.
func (s *srv) placeOptions(ctx context.Context, rows []store.AuthorPlaybookRow) (placeList, map[string]string) {
	all, err := s.pg.ListAuthorableJurisdictions(ctx)
	if err != nil {
		return placeList{known: map[string]bool{}}, map[string]string{}
	}
	parentOf := make(map[string]string, len(all))
	meta := make(map[string]store.Jurisdiction, len(all))
	for _, j := range all {
		parentOf[j.Slug] = j.ParentSlug
		meta[j.Slug] = j
	}

	// Count each page against its own jurisdiction and every ancestor, so a
	// state's number is the work in that state rather than the handful of pages
	// scoped to the state itself.
	count := map[string]int{}
	for _, p := range rows {
		seen := map[string]bool{}
		for slug := p.JurisdictionSlug; slug != "" && !seen[slug]; slug = parentOf[slug] {
			seen[slug] = true
			count[slug]++
		}
	}

	out := placeList{known: map[string]bool{}}
	for slug, n := range count {
		j, ok := meta[slug]
		if !ok {
			continue
		}
		out.known[slug] = true
		out.Options = append(out.Options, placeOption{Slug: slug, Name: j.Name, Kind: j.Kind, Count: n})
	}
	rank := map[string]int{"country": 0, "state": 1, "city": 2}
	sort.SliceStable(out.Options, func(i, j int) bool {
		if rank[out.Options[i].Kind] != rank[out.Options[j].Kind] {
			return rank[out.Options[i].Kind] < rank[out.Options[j].Kind]
		}
		return out.Options[i].Name < out.Options[j].Name
	})
	return out, parentOf
}

// importPage is one option in the "copy sources from" picker.
type importPage struct {
	ID      int64
	Topic   string
	Sources int
}

// importGroup is the pages for one jurisdiction. The picker is grouped because
// the sources worth copying are nearly always another page for the same city:
// a Pittsburgh page needs PALawHelp and Allegheny County, not Seattle's.
type importGroup struct {
	City  string
	Pages []importPage
}

// importablePages lists pages worth copying sources from, grouped by
// jurisdiction and with the current page's own jurisdiction first.
//
// A page with no sources is left out; offering it would only waste a click.
func (s *srv) importablePages(ctx context.Context, excludeID int64, nearCity string) []importGroup {
	rows, err := s.pg.AuthorListPlaybooks(ctx)
	if err != nil {
		return nil
	}
	byCity := map[string][]importPage{}
	for _, r := range rows {
		if r.ID == excludeID || r.SourceCount == 0 {
			continue
		}
		byCity[r.JurisdictionName] = append(byCity[r.JurisdictionName],
			importPage{ID: r.ID, Topic: r.TopicSlug, Sources: r.SourceCount})
	}
	out := make([]importGroup, 0, len(byCity))
	for city, pages := range byCity {
		sort.SliceStable(pages, func(i, j int) bool { return pages[i].Topic < pages[j].Topic })
		out = append(out, importGroup{City: city, Pages: pages})
	}
	// The city being edited leads: copying from a sibling page is the common
	// case, and scrolling past every other city to reach it is the friction
	// this picker exists to remove.
	sort.SliceStable(out, func(i, j int) bool {
		if (out[i].City == nearCity) != (out[j].City == nearCity) {
			return out[i].City == nearCity
		}
		return out[i].City < out[j].City
	})
	return out
}

// sourcesJSON returns one page's sources so the form can copy them into
// another page without the reviewer retyping a URL, publisher and kind that
// already exist.
//
// Cities reuse the same handful of authorities across every topic — one state
// code, one legal aid site, one city agency — so a new page for a city that
// already has pages is mostly a re-entry exercise. Getting a URL subtly wrong
// creates a second source row for the same document, which splits the source
// checker's view of it and shows a reader two chips for one authority.
//
// Sources only. Statements are the part that differs between topics, and
// copying those is what ?from= already does when starting a page from another.
func (s *srv) sourcesJSON(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	pw, err := s.pg.AuthorGetPlaybook(r.Context(), id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	editorial, _ := s.pg.GetEditorialSource(r.Context())

	type srcOut struct {
		URL       string `json:"url"`
		Publisher string `json:"publisher"`
		Kind      string `json:"kind"`
		Locator   string `json:"locator"`
	}
	out := []srcOut{}
	seen := map[string]bool{}
	for _, st := range pw.Statements {
		for _, c := range st.Citations {
			// The editorial source is a site-wide singleton the form adds with
			// its own checkbox, so copying it would produce a duplicate entry
			// for something that is not really a source.
			if c.SourceID == editorial.ID || seen[c.SourceURL] {
				continue
			}
			seen[c.SourceURL] = true
			out = append(out, srcOut{URL: c.SourceURL, Publisher: c.Publisher, Kind: c.SourceKind, Locator: c.Locator})
		}
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(out); err != nil {
		s.log.Error("encode sources", slog.Any("err", err))
	}
}

// checkQuoteLive answers the form's live, per-citation check: a reviewer
// leaves a quote field, the browser posts the source URL and what they typed,
// and this runs the exact same check the save path would — so a citation that
// will be refused later shows that now, while it is one field to fix instead
// of a full-page error after writing the rest of the page.
// sourceText returns the readable text of a source URL, fetched through the
// shared source cache, so an author can read the source in the form instead of
// tabbing out to a site that may fight the browser (PDFs, WAF walls). Fetching
// arbitrary URLs server-side is nothing new for this tool — /api/check-quote
// has always done it — and the portal sits behind basic auth.
func (s *srv) sourceText(w http.ResponseWriter, r *http.Request) {
	u := strings.TrimSpace(r.URL.Query().Get("url"))
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		http.Error(w, "url query parameter must be an http(s) URL", http.StatusBadRequest)
		return
	}
	qv := newQuoteVerifier(s.pg, s.sourceCache)
	text, err := qv.fetchCached(u)
	// Fetched page text is untrusted; text/plain plus nosniff means no browser
	// will ever interpret it as HTML, which is what gosec's XSS taint warning
	// is about.
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, "Could not fetch %s: %v", u, err) //nolint:gosec // plain text + nosniff, see above
		return
	}
	_, _ = w.Write([]byte(text)) //nolint:gosec // plain text + nosniff, see above
}

func (s *srv) checkQuoteLive(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL   string `json:"url"`
		Quote string `json:"quote"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.URL) == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}
	qv := newQuoteVerifier(s.pg, s.sourceCache)
	res := qv.checkQuote(r.Context(), req.URL, req.Quote)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":          res.Msg == "",
		"message":     res.Msg,
		"overridable": res.Overridable,
	})
}

// unpublish takes a live page down and leaves its content as the slot's draft.
//
// The collision — a draft already waiting in the same slot — is reported to the
// reviewer rather than resolved here, because both ways of resolving it lose
// someone's work and only a person can weigh that.
func (s *srv) unpublish(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	retire := r.FormValue("retire_draft") == "1"
	switch err := s.pg.AuthorUnpublishPlaybook(r.Context(), id, retire); {
	case err == nil:
		http.Redirect(w, r, fmt.Sprintf("/view/%d", id), http.StatusSeeOther)
	case errors.Is(err, store.ErrDraftExists):
		// Not a dead end: show what will happen and offer to go ahead. A page
		// can only hold one draft, so the waiting one has to move aside — but
		// it is retired, not deleted, and stays readable under Replaced.
		s.actionError(w, id, actionChoice{
			Message: "This page already has a draft revision waiting, and a page can only hold one draft. " +
				"Taking this page down will move that draft to Replaced, where it keeps its statements, " +
				"citations and quotes and can still be read. Nothing is deleted.",
			ConfirmLabel: "Retire that draft and take this page down",
			ConfirmPath:  fmt.Sprintf("/unpublish/%d", id),
			ConfirmField: "retire_draft",
		})
	case errors.Is(err, store.ErrNotPublished):
		s.actionError(w, id, actionChoice{Message: "This page is not live, so there is nothing to take down."})
	case errors.Is(err, store.ErrNotFound):
		http.Error(w, "not found", http.StatusNotFound)
	default:
		s.serverError(w, err)
	}
}

// actionChoice is a blocked action explained to the reviewer, optionally with
// the button that goes ahead anyway.
type actionChoice struct {
	Message      string
	ConfirmLabel string // empty when there is nothing to confirm
	ConfirmPath  string
	ConfirmField string // hidden field set to "1" when confirming
}

// actionError shows a blocked action as a page the reviewer can read and act
// on, rather than a 500 that loses which page they were working with.
func (s *srv) actionError(w http.ResponseWriter, id int64, c actionChoice) {
	w.WriteHeader(http.StatusConflict)
	s.render(w, "actionerror.html", map[string]any{"C": c, "ID": id})
}

func (s *srv) delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := s.pg.AuthorDeletePlaybook(r.Context(), id); err != nil {
		s.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// resolveTopic reads the topic selection off the form and resolves it against
// the topic registry, returning the row or a message to show the author.
//
// Topics are a closed set: the form picks one, it never creates one. Adding a
// topic is an editorial decision made in a migration. Both form paths used to
// build the slug as citySlug + "-" + topicKey and upsert it, which fragmented
// topic hubs and cross-city links and produced /j/{city}/{city}-{topic} URLs —
// the incident cleaned up on 2026-08-01. Resolving against the registry makes
// that unrepresentable rather than merely discouraged. See ADR-005 D5.
func (s *srv) resolveTopic(ctx context.Context, r *http.Request) (store.Topic, string) {
	slug := strings.TrimSpace(r.FormValue("topic_key"))
	if slug == "" {
		return store.Topic{}, "Please select a topic"
	}
	t, err := s.pg.GetTopicBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.Topic{}, fmt.Sprintf("Topic %q is not in the topic list. Pick an existing topic.", slug)
		}
		return store.Topic{}, "Could not read the topic list: " + err.Error()
	}
	return t, ""
}

// languageOption is one choice in the form's language select.
type languageOption struct{ Code, Label string }

// languageOptions lists the languages the form can save a page in, sourced
// from voice.Supported() so the dropdown can't offer a code the editorial
// lint (and drafting.ResolveLanguage) would reject.
func languageOptions() []languageOption {
	supported := voice.Supported()
	opts := make([]languageOption, 0, len(supported))
	for _, c := range supported {
		opts = append(opts, languageOption{Code: c, Label: voice.Label(c)})
	}
	return opts
}

// editFormData builds the template data map for the edit form.
func (s *srv) editFormData(ctx context.Context, pw store.PlaybookWithStatements, errMsg string) map[string]any {
	editorial, _ := s.pg.GetEditorialSource(ctx)
	pj, _ := json.Marshal(buildPreload(pw, editorial.ID))

	citySlug := pw.Jurisdiction.Slug
	topicKey := pw.Topic.Slug
	topics, _ := s.pg.ListTopicRegistry(ctx)

	data := map[string]any{
		"EditMode":              true,
		"EditID":                pw.Playbook.ID,
		"Status":                pw.Playbook.Status,
		"CityName":              pw.Jurisdiction.Name,
		"TopicSlug":             pw.Topic.Slug,
		"SelectedCitySlug":      citySlug,
		"SelectedTopicKey":      topicKey,
		"Topics":                topics,
		"SelectedPageKind":      pw.Playbook.PageKind,
		"Languages":             languageOptions(),
		"SelectedLanguage":      pw.Playbook.Language,
		"SelectedLanguageLabel": voice.Label(pw.Playbook.Language),
		"Title":                 pw.Playbook.Title,
		"Intro":                 pw.Playbook.IntroMD,
		"AuthorNotes":           pw.Playbook.AuthorNotes,
		"Error":                 errMsg,
		"ImportGroups":          s.importablePages(ctx, pw.Playbook.ID, pw.Jurisdiction.Name),
		//nolint:gosec // pj is json.Marshal output of an internal struct, not user input
		"PreloadJSON": template.JS(pj),
	}
	if pw.Playbook.Status == "draft" {
		jurisdictions, _ := s.pg.ListAuthorableJurisdictions(ctx)
		data["Jurisdictions"] = jurisdictions
	}
	return data
}

func (s *srv) showEditForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	pw, err := s.pg.AuthorGetPlaybook(r.Context(), id)
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.render(w, "form.html", s.editFormData(r.Context(), pw, ""))
}

func (s *srv) submitEditForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	existing, err := s.pg.AuthorGetPlaybook(ctx, id)
	if err != nil {
		s.serverError(w, err)
		return
	}

	// editErr re-renders the edit form with a validation message, showing what
	// was submitted rather than what is stored. Re-reading the playbook here
	// silently reverted the whole session: on a page with a dozen statements,
	// one missing citation discarded every other edit the author had made.
	//
	// The stored copy is still the fallback. A request that failed to parse has
	// no usable form values, and handing back an empty form would be a second
	// way to lose the same work.
	editErr := func(msg string) {
		pw := existing
		if v := r.FormValue("title"); v != "" {
			pw.Playbook.Title = v
		}
		if v := r.FormValue("intro"); v != "" {
			pw.Playbook.IntroMD = v
		}
		data := s.editFormData(context.Background(), pw, msg)
		if submitted := preloadFromForm(r); len(submitted.Stmts) > 0 || len(submitted.Sources) > 0 {
			pj, _ := json.Marshal(submitted)
			//nolint:gosec // pj is json.Marshal output of an internal struct, not user input
			data["PreloadJSON"] = template.JS(pj)
			if existing.Playbook.Status == "draft" {
				data["SelectedCitySlug"] = r.FormValue("jurisdiction_select")
				data["SelectedTopicKey"] = r.FormValue("topic_key")
				data["NewCityName"] = r.FormValue("new_city_name")
				data["NewStateName"] = r.FormValue("new_state_name")
			}
			data["SelectedPageKind"] = r.FormValue("page_kind")
			if existing.Playbook.Status == "draft" {
				data["SelectedLanguage"] = r.FormValue("language")
			}
		}
		s.render(w, "form.html", data)
	}

	if err := r.ParseForm(); err != nil {
		editErr("Invalid form data: " + err.Error())
		return
	}

	// Resolve jurisdiction and topic — editable for drafts, locked for published.
	var jurisdictionID, topicID int64
	var lang, slug string
	if existing.Playbook.Status == "draft" {
		jSlug := r.FormValue("jurisdiction_select")
		var j store.Jurisdiction
		var citySlug string
		if jSlug == "new" {
			cityName := strings.TrimSpace(r.FormValue("new_city_name"))
			stateName := strings.TrimSpace(r.FormValue("new_state_name"))
			if cityName == "" || stateName == "" {
				editErr("City name and state name are required for a new city")
				return
			}
			citySlug = toSlug(cityName)
			state, err := s.pg.UpsertJurisdiction(ctx, store.UpsertJurisdictionParams{
				Kind: "state", Name: stateName, Slug: toSlug(stateName),
			})
			if err != nil {
				editErr("Failed to create state: " + err.Error())
				return
			}
			j, err = s.pg.UpsertJurisdiction(ctx, store.UpsertJurisdictionParams{
				ParentID: &state.ID, Kind: "city", Name: cityName, Slug: citySlug,
			})
			if err != nil {
				editErr("Failed to create city: " + err.Error())
				return
			}
		} else {
			j, err = s.pg.GetJurisdictionBySlug(ctx, jSlug)
			if err != nil {
				editErr("Unknown city selected")
				return
			}
		}
		topic, msg := s.resolveTopic(ctx, r)
		if msg != "" {
			editErr(msg)
			return
		}
		jurisdictionID = j.ID
		topicID = topic.ID
		lang, err = drafting.ResolveLanguage(r.FormValue("language"))
		if err != nil {
			editErr(err.Error())
			return
		}
		slug = topic.Slug
	} else {
		jurisdictionID = existing.Playbook.JurisdictionID
		topicID = existing.Playbook.TopicID
		lang = existing.Playbook.Language
		slug = existing.Playbook.Slug
	}

	srcIndices := parseIndices(r.FormValue("active_sources"))
	sourceByIdx := make(map[int]store.Source, len(srcIndices))
	srcLocatorByIdx := make(map[int]string, len(srcIndices))
	for _, i := range srcIndices {
		u := strings.TrimSpace(r.FormValue(fmt.Sprintf("src_url_%d", i)))
		pub := strings.TrimSpace(r.FormValue(fmt.Sprintf("src_pub_%d", i)))
		knd := r.FormValue(fmt.Sprintf("src_kind_%d", i))
		loc := r.FormValue(fmt.Sprintf("src_loc_%d", i))
		if u == "" || pub == "" {
			editErr(fmt.Sprintf("Source %d is missing a URL or publisher", i+1))
			return
		}
		src, err := s.pg.UpsertSource(ctx, store.UpsertSourceParams{URL: u, Publisher: pub, Kind: knd})
		if err != nil {
			editErr("Failed to save source: " + err.Error())
			return
		}
		sourceByIdx[i] = src
		srcLocatorByIdx[i] = loc
	}

	editorialSrc, err := s.pg.GetEditorialSource(ctx)
	if err != nil {
		editErr("Editorial source not found — run migrations first")
		return
	}

	stmtIndices := parseIndices(r.FormValue("active_stmts"))
	qv := newQuoteVerifier(s.pg, s.sourceCache)
	var statements []store.IngestStatementParams
	for _, ji := range stmtIndices {
		body := strings.TrimSpace(r.FormValue(fmt.Sprintf("stmt_%d", ji)))
		if body == "" {
			continue
		}
		var cites []store.IngestCitationParams
		for _, i := range srcIndices {
			if r.FormValue(fmt.Sprintf("cite_%d_%d", ji, i)) == "on" {
				loc := r.FormValue(fmt.Sprintf("loc_%d_%d", ji, i))
				if loc == "" {
					loc = srcLocatorByIdx[i]
				}
				quote := r.FormValue(fmt.Sprintf("quote_%d_%d", ji, i))
				overridden := r.FormValue(fmt.Sprintf("verify_override_%d_%d", ji, i)) == "on"
				res := qv.check(ctx, ji+1, sourceByIdx[i].URL, quote)
				if res.Msg != "" && (!res.Overridable || !overridden) {
					editErr(res.Msg)
					return
				}
				// See submitForm: the quote round-trips through the form, and
				// dropping it here silently wiped the evidence.
				cites = append(cites, store.IngestCitationParams{
					SourceID:         sourceByIdx[i].ID,
					Locator:          loc,
					Quote:            quote,
					ManuallyVerified: overridden,
					CheckedNow:       res.Verified || overridden, // see submitForm
				})
			}
		}
		if r.FormValue(fmt.Sprintf("edit_%d", ji)) == "on" {
			cites = append(cites, store.IngestCitationParams{SourceID: editorialSrc.ID})
		}
		if len(cites) == 0 {
			editErr(fmt.Sprintf("Statement %d needs at least one citation", ji+1))
			return
		}
		statements = append(statements, store.IngestStatementParams{BodyMD: body, Language: lang, Sources: cites})
	}
	if len(statements) == 0 {
		editErr("At least one statement is required")
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		editErr("Title is required")
		return
	}

	if err := s.pg.AuthorUpdatePlaybook(ctx, store.AuthorUpdatePlaybookParams{
		ID:             id,
		JurisdictionID: jurisdictionID,
		TopicID:        topicID,
		Language:       lang,
		Slug:           slug,
		Title:          title,
		IntroMD:        strings.TrimSpace(r.FormValue("intro")),
		PageKind:       r.FormValue("page_kind"),
		AuthorNotes:    strings.TrimSpace(r.FormValue("author_notes")),
		Statements:     statements,
	}); err != nil {
		editErr("Failed to save playbook: " + err.Error())
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/view/%d", id), http.StatusSeeOther)
}

// ---- helpers ----------------------------------------------------------------

func (s *srv) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		s.log.Error("render template", slog.String("tmpl", name), slog.Any("err", err))
	}
}

func (s *srv) serverError(w http.ResponseWriter, err error) {
	s.log.Error("server error", slog.Any("err", err))
	http.Error(w, "Internal server error", http.StatusInternalServerError)
}

// quoteVerifier checks pasted quotes against the source they claim to come
// from.
//
// The drafting tools verify a quote when the agent writes it. A reviewer typing
// one into the form had no such check, and until the publish guard landed there
// was nothing downstream either: a hand-added citation reached renters carrying
// whatever text someone typed. This closes that path without making the form
// slower to use than it has to be.
// quoteLookup is the one thing the verifier needs from the database, kept
// narrow so the check can be tested without a Postgres.
type quoteLookup interface {
	CitationQuoteExists(ctx context.Context, url, quote string) (bool, error)
}

type quoteVerifier struct {
	quotes quoteLookup
	fetch  func(string) (string, error)
	cache  *sourceFetchCache // shared across requests; nil disables caching (tests)
}

func newQuoteVerifier(pg quoteLookup, cache *sourceFetchCache) *quoteVerifier {
	return &quoteVerifier{quotes: pg, fetch: drafting.FetchExtract, cache: cache}
}

// quoteCheckResult is check's verdict: Msg is a reviewer-facing message when
// the quote could not be confirmed ("" when it could), and Overridable says
// whether that failure was the source being unreachable — blocked outright,
// e.g. a WAF 403 — rather than a confirmed mismatch. Only an unreachable
// source can be overridden by a reviewer who read it by hand; a mismatch
// means the text was read and it is simply wrong, which no override may pass.
type quoteCheckResult struct {
	Msg         string
	Overridable bool
	// Verified says this call fetched the live source and found the quote in
	// it. False on the known-quote skip: that pass rests on a verification
	// recorded earlier, so the citation's checked_at must inherit that older
	// stamp rather than claim one from a fetch that never happened.
	Verified bool
}

// check returns a reviewer-facing message when the quote cannot be confirmed,
// and a zero result when it can. It is checkQuote with the statement number
// woven into the message so a save error names the row that failed.
func (qv *quoteVerifier) check(ctx context.Context, stmtNo int, url, quote string) quoteCheckResult {
	res := qv.checkQuote(ctx, url, quote)
	if res.Msg != "" {
		res.Msg = fmt.Sprintf("Statement %d: %s", stmtNo, res.Msg)
	}
	return res
}

// checkQuote is check without a statement number, for callers that have no
// statement to name — the live per-citation check the form fires on blur.
//
// An unchanged quote is skipped: it is already stored, so it was verified when
// it was written. That keeps a save fast, and keeps a source that has since
// gone unreachable from blocking edits to text that does not depend on it.
func (qv *quoteVerifier) checkQuote(ctx context.Context, url, quote string) quoteCheckResult {
	quote = strings.TrimSpace(quote)
	if quote == "" {
		return quoteCheckResult{} // absence is caught at publish, not here: a draft may be part-finished
	}
	if known, err := qv.quotes.CitationQuoteExists(ctx, url, quote); err == nil && known {
		return quoteCheckResult{}
	}
	text, err := qv.fetchCached(url)
	if err != nil {
		return quoteCheckResult{Overridable: true, Msg: fmt.Sprintf(
			"could not open %s to check the quote (%v). "+
				"Check \"I verified this quote myself\" below to save it anyway.", url, err)}
	}
	if !drafting.QuoteAppearsIn(text, quote) {
		return quoteCheckResult{Msg: fmt.Sprintf("that quote does not appear in %s. "+
			"Copy the wording from the source exactly, without editing it.", url)}
	}
	return quoteCheckResult{Verified: true}
}

// fetchCached routes a fetch through the shared source cache when one is set,
// so the live check fired on every blur and the check repeated at save don't
// each pay a slow or blocked source's fetch cost on their own.
func (qv *quoteVerifier) fetchCached(url string) (string, error) {
	if qv.cache == nil {
		return qv.fetch(url)
	}
	if text, err, ok := qv.cache.get(url); ok {
		return text, err
	}
	text, err := qv.fetch(url)
	qv.cache.put(url, text, err)
	return text, err
}

// sourceFetchCache remembers a source fetch's outcome for a short time so
// repeated checks of the same URL — several citations to one source, or a
// live check followed moments later by save — don't each re-fetch it. A
// blocked source's fetch alone can cost tens of seconds (direct timeout plus
// the headless-render fallback), so without this a page citing one such
// source several times would pay that cost once per citation instead of once.
type sourceFetchCache struct {
	mu  sync.Mutex
	ttl time.Duration
	m   map[string]cacheEntry
}

type cacheEntry struct {
	text string
	err  error
	at   time.Time
}

func newSourceFetchCache(ttl time.Duration) *sourceFetchCache {
	return &sourceFetchCache{ttl: ttl, m: map[string]cacheEntry{}}
}

func (c *sourceFetchCache) get(url string) (text string, err error, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, found := c.m[url]
	if !found || time.Since(e.at) > c.ttl {
		return "", nil, false
	}
	return e.text, e.err, true
}

func (c *sourceFetchCache) put(url, text string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[url] = cacheEntry{text: text, err: err, at: time.Now()}
}

// formError re-renders the new-playbook form with a validation message, handing
// back everything that was submitted.
//
// Both selects have to be repopulated: without Topics the author lands on a
// form whose topic dropdown is empty and cannot resubmit at all. The statements
// and sources come from the POST, not the database — there is no stored copy of
// a page that has never saved.
func (s *srv) formError(w http.ResponseWriter, r *http.Request, msg string) {
	ctx := context.Background()
	jurisdictions, _ := s.pg.ListAuthorableJurisdictions(ctx)
	topics, _ := s.pg.ListTopicRegistry(ctx)
	pj, _ := json.Marshal(preloadFromForm(r))
	s.render(w, "form.html", map[string]any{
		"Jurisdictions":    jurisdictions,
		"Topics":           topics,
		"SelectedCitySlug": r.FormValue("jurisdiction_select"),
		"SelectedTopicKey": r.FormValue("topic_key"),
		"SelectedPageKind": r.FormValue("page_kind"),
		"Languages":        languageOptions(),
		"SelectedLanguage": r.FormValue("language"),
		"NewCityName":      r.FormValue("new_city_name"),
		"NewStateName":     r.FormValue("new_state_name"),
		"Title":            r.FormValue("title"),
		"Intro":            r.FormValue("intro"),
		"Error":            msg,
		//nolint:gosec // pj is json.Marshal output of an internal struct, not user input
		"PreloadJSON": template.JS(pj),
	})
}

// ---- preload ----------------------------------------------------------------

type preloadData struct {
	Sources []preloadSrc  `json:"sources"`
	Stmts   []preloadStmt `json:"stmts"`
}

type preloadSrc struct {
	ID        int    `json:"id"`
	URL       string `json:"url"`
	Publisher string `json:"publisher"`
	Kind      string `json:"kind"`
	Locator   string `json:"locator"`
}

type preloadStmt struct {
	ID        int               `json:"id"`
	Body      string            `json:"body"`
	Editorial bool              `json:"editorial"`
	Cites     []int             `json:"cites"`    // indices into sources slice
	Locators  map[string]string `json:"locators"` // "srcIdx" -> locator override
	// Quotes carries the verbatim source text backing each citation, keyed the
	// same way as Locators. The form never captured it, so every save rewrote
	// citations.quote to "" — a human review pass silently destroyed the
	// evidence the drafting agent had verified. It round-trips read-only.
	Quotes map[string]string `json:"quotes"`
	// Verified marks a citation, keyed the same way as Quotes, whose quote a
	// reviewer attested to by hand because the automated fetch could not reach
	// the source at all. Round-trips so a manual override survives a
	// validation-error re-render and an edit of an already-saved draft.
	Verified map[string]bool `json:"verified"`
}

// preloadFromForm rebuilds the preload structure from a submitted form rather
// than from the database.
//
// A validation error used to re-render from stored state, which threw away
// every statement and source the author had in the browser — on a page with a
// dozen statements, one missing citation cost the whole session. The form's own
// ids are arbitrary (the JS hands them out as cards are added and removed), so
// they are renumbered to the sequential positions the re-rendered form expects.
//
// Empty statements are deliberately kept: an empty body is often exactly what
// the author has to come back and fill in.
func preloadFromForm(r *http.Request) preloadData {
	srcIDs := parseIndices(r.FormValue("active_sources"))
	position := make(map[int]int, len(srcIDs))
	out := preloadData{Sources: make([]preloadSrc, 0, len(srcIDs))}

	for i, id := range srcIDs {
		position[id] = i
		out.Sources = append(out.Sources, preloadSrc{
			ID:        i,
			URL:       strings.TrimSpace(r.FormValue(fmt.Sprintf("src_url_%d", id))),
			Publisher: strings.TrimSpace(r.FormValue(fmt.Sprintf("src_pub_%d", id))),
			Kind:      r.FormValue(fmt.Sprintf("src_kind_%d", id)),
			Locator:   r.FormValue(fmt.Sprintf("src_loc_%d", id)),
		})
	}

	stmtIDs := parseIndices(r.FormValue("active_stmts"))
	out.Stmts = make([]preloadStmt, 0, len(stmtIDs))
	for i, id := range stmtIDs {
		ps := preloadStmt{
			ID:        i,
			Body:      r.FormValue(fmt.Sprintf("stmt_%d", id)),
			Editorial: r.FormValue(fmt.Sprintf("edit_%d", id)) == "on",
			Locators:  map[string]string{},
			Quotes:    map[string]string{},
			Verified:  map[string]bool{},
		}
		for _, srcID := range srcIDs {
			if r.FormValue(fmt.Sprintf("cite_%d_%d", id, srcID)) == "on" {
				ps.Cites = append(ps.Cites, position[srcID])
			}
			if loc := r.FormValue(fmt.Sprintf("loc_%d_%d", id, srcID)); loc != "" {
				ps.Locators[strconv.Itoa(position[srcID])] = loc
			}
			if q := r.FormValue(fmt.Sprintf("quote_%d_%d", id, srcID)); q != "" {
				ps.Quotes[strconv.Itoa(position[srcID])] = q
			}
			if r.FormValue(fmt.Sprintf("verify_override_%d_%d", id, srcID)) == "on" {
				ps.Verified[strconv.Itoa(position[srcID])] = true
			}
		}
		out.Stmts = append(out.Stmts, ps)
	}
	return out
}

// buildPreload converts a stored playbook into the JS-ready preload structure
// so the authoring form can render it as a starting point for a new playbook.
func buildPreload(pw store.PlaybookWithStatements, editorialSourceID int64) preloadData {
	type meta struct {
		idx       int
		url       string
		publisher string
		kind      string
	}
	srcByID := map[int64]meta{}
	var srcOrder []int64

	for _, stmt := range pw.Statements {
		for _, c := range stmt.Citations {
			if c.SourceID == editorialSourceID {
				continue
			}
			if _, ok := srcByID[c.SourceID]; !ok {
				srcByID[c.SourceID] = meta{
					idx:       len(srcOrder),
					url:       c.SourceURL,
					publisher: c.Publisher,
					kind:      c.SourceKind,
				}
				srcOrder = append(srcOrder, c.SourceID)
			}
		}
	}

	sources := make([]preloadSrc, 0, len(srcOrder))
	for i, id := range srcOrder {
		m := srcByID[id]
		sources = append(sources, preloadSrc{ID: i, URL: m.url, Publisher: m.publisher, Kind: m.kind})
	}

	stmts := make([]preloadStmt, 0, len(pw.Statements))
	for i, stmt := range pw.Statements {
		ps := preloadStmt{ID: i, Body: stmt.BodyMD, Locators: map[string]string{}, Quotes: map[string]string{}, Verified: map[string]bool{}}
		for _, c := range stmt.Citations {
			if c.SourceID == editorialSourceID {
				ps.Editorial = true
			} else if m, ok := srcByID[c.SourceID]; ok {
				ps.Cites = append(ps.Cites, m.idx)
				if c.Locator != "" {
					ps.Locators[strconv.Itoa(m.idx)] = c.Locator
				}
				if c.Quote != "" {
					ps.Quotes[strconv.Itoa(m.idx)] = c.Quote
				}
				if c.ManuallyVerified {
					ps.Verified[strconv.Itoa(m.idx)] = true
				}
			}
		}
		stmts = append(stmts, ps)
	}

	return preloadData{Sources: sources, Stmts: stmts}
}

func parseIndices(s string) []int {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if n, err := strconv.Atoi(p); err == nil {
			out = append(out, n)
		}
	}
	return out
}

// toSlug converts a human-readable name to a lowercase hyphenated slug.
func toSlug(s string) string {
	s = strings.ToLower(s)
	var sb strings.Builder
	prev := rune('-')
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
			prev = r
		} else if r != '\'' && r != '’' && prev != '-' {
			sb.WriteRune('-')
			prev = '-'
		}
	}
	return strings.Trim(sb.String(), "-")
}
