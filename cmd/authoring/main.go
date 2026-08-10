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
	webstatic "github.com/nazanindev/defensiverenting/web/static"
	sitetmpl "github.com/nazanindev/defensiverenting/web/templates"
)

//go:embed templates/*.html
var templateFS embed.FS

type srv struct {
	pg   *store.PG
	log  *slog.Logger
	tmpl *template.Template
	jobs *jobSet
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

	s := &srv{pg: pg, log: log, tmpl: tmpl, jobs: newJobSet()}

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
	return url.Values{"status": {v.Status}, "sort": {v.Sort}, "dir": {v.Dir}}.Encode()
}

// SortLink returns the full href for sorting by col: the same column reversed,
// or a fresh column at its natural direction.
//
// It returns template.URL rather than a string because html/template treats a
// bare value after "/?" as a single query parameter and escapes the "=" and "&"
// inside it, which silently turns every sort link into one meaningless
// parameter. Marking it trusted is safe precisely because normalize() has
// already forced all three fields to values from fixed allowlists.
func (v dashboardView) SortLink(col string) template.URL {
	next := dashboardView{Status: v.Status, Sort: col}
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
	v := dashboardView{Status: q.Get("status"), Sort: q.Get("sort"), Dir: q.Get("dir")}
	if v.Status == "" && v.Sort == "" && v.Dir == "" {
		if c, err := r.Cookie(viewCookie); err == nil {
			if remembered, err := url.ParseQuery(c.Value); err == nil {
				v = dashboardView{
					Status: remembered.Get("status"),
					Sort:   remembered.Get("sort"),
					Dir:    remembered.Get("dir"),
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

	// Cities with no directory at all. Called out on its own because it is the
	// gap that has no other symptom: nothing in the page list can show a page
	// that was never drafted.
	missingDirectory := make([]store.CoverageRow, 0)
	for _, c := range coverage {
		if c.Status["resource-directory"] == "" {
			missingDirectory = append(missingDirectory, c)
		}
	}

	view := readView(r)
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
		"Playbooks":        playbooks,
		"Cities":           cities,
		"ReviewCounts":     counts,
		"Generating":       s.jobs.list(),
		"Flagged":          flagged,
		"View":             view,
		"Status":           view.Status,
		"ShowLanguage":     len(langs) > 1,
		"Coverage":         coverage,
		"CoreTopics":       coreTopics,
		"MissingDirectory": missingDirectory,
		"DraftCount":       draftCount,
		"PublishedCount":   publishedCount,
		"TotalCount":       draftCount + publishedCount + supersededCount,
		"SupersededCount":  supersededCount,
		"Msg":              r.URL.Query().Get("msg"),
		"Err":              r.URL.Query().Get("err"),
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
		err := draftagent.Run(context.Background(), drafting.New(s.pg), draftagent.Options{
			CitySlug:  city,
			TopicSlug: topic,
			TopicName: topicName,
			Log: func(format string, a ...any) {
				s.log.Info("draftgen", slog.String("job", key), slog.String("msg", fmt.Sprintf(format, a...)))
			},
		})
		if err != nil {
			s.log.Error("draftgen failed", slog.String("job", key), slog.Any("err", err))
			return
		}
		s.log.Info("draftgen saved", slog.String("job", key))
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
		"PreloadJSON": template.JS("null"),
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

	type viewSource struct {
		Num       int
		URL       string
		Publisher string
		Kind      string
	}
	type viewCite struct {
		Num     int
		Locator string
		Quote   string
		URL     string
		Domain  string
	}
	type viewStmt struct {
		Num       int
		Body      string
		Cites     []viewCite
		Editorial bool
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
				})
			}
		}
		stmts = append(stmts, vs)
	}

	s.render(w, "view.html", map[string]any{
		"Playbook": pw,
		"Sources":  sources,
		"Stmts":    stmts,
	})
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
			s.formError(w, r, "Unknown city selected")
			return
		}
	}

	topic, msg := s.resolveTopic(ctx, r)
	if msg != "" {
		s.formError(w, r, msg)
		return
	}
	topicSlug := topic.Slug

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

	// Parse statements — language is always "en" at this stage
	stmtIndices := parseIndices(r.FormValue("active_stmts"))
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
				// Quote is carried through the form read-only. Omitting it here
				// is what rewrote every citation's quote to "" on save.
				cites = append(cites, store.IngestCitationParams{
					SourceID: sourceByIdx[i].ID,
					Locator:  loc,
					Quote:    r.FormValue(fmt.Sprintf("quote_%d_%d", ji, i)),
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
			BodyMD: body, Language: "en", Sources: cites,
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
		Language:       "en",
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

// editFormData builds the template data map for the edit form.
func (s *srv) editFormData(ctx context.Context, pw store.PlaybookWithStatements, errMsg string) map[string]any {
	editorial, _ := s.pg.GetEditorialSource(ctx)
	pj, _ := json.Marshal(buildPreload(pw, editorial.ID))

	citySlug := pw.Jurisdiction.Slug
	topicKey := pw.Topic.Slug
	topics, _ := s.pg.ListTopicRegistry(ctx)

	data := map[string]any{
		"EditMode":         true,
		"EditID":           pw.Playbook.ID,
		"Status":           pw.Playbook.Status,
		"CityName":         pw.Jurisdiction.Name,
		"TopicSlug":        pw.Topic.Slug,
		"SelectedCitySlug": citySlug,
		"SelectedTopicKey": topicKey,
		"Topics":           topics,
		"SelectedPageKind": pw.Playbook.PageKind,
		"Title":            pw.Playbook.Title,
		"Intro":            pw.Playbook.IntroMD,
		"Error":            errMsg,
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
		lang = "en"
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
				// See submitForm: the quote round-trips read-only through the
				// form, and dropping it here silently wiped the evidence.
				cites = append(cites, store.IngestCitationParams{
					SourceID: sourceByIdx[i].ID,
					Locator:  loc,
					Quote:    r.FormValue(fmt.Sprintf("quote_%d_%d", ji, i)),
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
		statements = append(statements, store.IngestStatementParams{BodyMD: body, Language: "en", Sources: cites})
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
		ps := preloadStmt{ID: i, Body: stmt.BodyMD, Locators: map[string]string{}, Quotes: map[string]string{}}
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
