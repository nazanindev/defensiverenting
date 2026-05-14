// cmd/authoring is a small internal service for authoring new city playbooks.
// It writes directly to the shared Postgres DB. No auth — keep the URL private.
// Run: authoring -addr :8081 -db $DATABASE_URL
package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"

	dbpkg "github.com/nazanin212/bostontenantsrights/db"
	"github.com/nazanin212/bostontenantsrights/internal/store"
)

//go:embed templates/*.html
var templateFS embed.FS

type srv struct {
	pg   *store.PG
	log  *slog.Logger
	tmpl *template.Template
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

	tmpl, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		log.Error("parse templates", slog.Any("err", err))
		os.Exit(1)
	}

	s := &srv{pg: pg, log: log, tmpl: tmpl}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.dashboard)
	mux.HandleFunc("GET /new", s.showForm)
	mux.HandleFunc("POST /new", s.submitForm)
	mux.HandleFunc("GET /edit/{id}", s.showEditForm)
	mux.HandleFunc("POST /edit/{id}", s.submitEditForm)
	mux.HandleFunc("GET /view/{id}", s.viewPlaybook)
	mux.HandleFunc("POST /publish/{id}", s.publish)
	mux.HandleFunc("POST /delete/{id}", s.delete)

	httpSrv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}

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

func (s *srv) dashboard(w http.ResponseWriter, r *http.Request) {
	playbooks, err := s.pg.AuthorListPlaybooks(r.Context())
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.render(w, "dashboard.html", map[string]any{"Playbooks": playbooks})
}

func (s *srv) showForm(w http.ResponseWriter, r *http.Request) {
	cities, err := s.pg.ListCityJurisdictions(r.Context())
	if err != nil {
		s.serverError(w, err)
		return
	}
	data := map[string]any{"Cities": cities, "Error": "", "PreloadJSON": template.JS("null")}

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
				vs.Cites = append(vs.Cites, viewCite{Num: m.Idx + 1, Locator: c.Locator})
			}
		}
		stmts = append(stmts, vs)
	}

	s.render(w, "view.html", map[string]any{
		"Playbook":  pw,
		"Sources":   sources,
		"Stmts":     stmts,
	})
}

func (s *srv) submitForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		s.formError(w, "Invalid form data: "+err.Error())
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
			s.formError(w, "City name and state name are required for a new city")
			return
		}
		citySlug = toSlug(cityName)
		stateSlug := toSlug(stateName)
		state, err := s.pg.UpsertJurisdiction(ctx, store.UpsertJurisdictionParams{
			Kind: "state", Name: stateName, Slug: stateSlug,
		})
		if err != nil {
			s.formError(w, "Failed to create state: "+err.Error())
			return
		}
		j, err = s.pg.UpsertJurisdiction(ctx, store.UpsertJurisdictionParams{
			ParentID: &state.ID, Kind: "city", Name: cityName, Slug: citySlug,
		})
		if err != nil {
			s.formError(w, "Failed to create city: "+err.Error())
			return
		}
	} else {
		j, err = s.pg.GetJurisdictionBySlug(ctx, jSlug)
		if err != nil {
			s.formError(w, "Unknown city selected")
			return
		}
		citySlug = jSlug
	}

	// Derive topic slug: {city-slug}-{topic-key}
	topicKey := r.FormValue("topic_key")
	if topicKey == "custom" {
		topicKey = toSlug(strings.TrimSpace(r.FormValue("custom_topic_name")))
	}
	if topicKey == "" {
		s.formError(w, "Please select a topic")
		return
	}
	topicSlug := citySlug + "-" + topicKey

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
			s.formError(w, fmt.Sprintf("Source %d is missing a URL or publisher", i+1))
			return
		}
		src, err := s.pg.UpsertSource(ctx, store.UpsertSourceParams{
			URL: u, Publisher: pub, Kind: knd,
		})
		if err != nil {
			s.formError(w, "Failed to save source: "+err.Error())
			return
		}
		sourceByIdx[i] = src
		srcLocatorByIdx[i] = loc
	}

	editorialSrc, err := s.pg.GetEditorialSource(ctx)
	if err != nil {
		s.formError(w, "Editorial source not found — run migrations first")
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
				cites = append(cites, store.IngestCitationParams{
					SourceID: sourceByIdx[i].ID,
					Locator:  loc,
				})
			}
		}
		if r.FormValue(fmt.Sprintf("edit_%d", ji)) == "on" {
			cites = append(cites, store.IngestCitationParams{SourceID: editorialSrc.ID})
		}
		if len(cites) == 0 {
			s.formError(w, fmt.Sprintf("Statement %d needs at least one citation", ji+1))
			return
		}
		statements = append(statements, store.IngestStatementParams{
			BodyMD: body, Language: "en", Sources: cites,
		})
	}
	if len(statements) == 0 {
		s.formError(w, "At least one statement is required")
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		s.formError(w, "Title is required")
		return
	}
	topic, err := s.pg.UpsertTopic(ctx, store.UpsertTopicParams{
		Slug: topicSlug, Name: slugToTitle(topicSlug),
	})
	if err != nil {
		s.formError(w, "Failed to save topic: "+err.Error())
		return
	}

	if err := s.pg.IngestPlaybook(ctx, store.IngestPlaybookParams{
		JurisdictionID: j.ID,
		TopicID:        topic.ID,
		Language:       "en",
		Slug:           topicSlug,
		Title:          title,
		IntroMD:        strings.TrimSpace(r.FormValue("intro")),
		Statements:     statements,
		Status:         "draft",
	}); err != nil {
		s.formError(w, "Failed to save playbook: "+err.Error())
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
	editorial, _ := s.pg.GetEditorialSource(r.Context())
	pj, _ := json.Marshal(buildPreload(pw, editorial.ID))
	//nolint:gosec // pj is json.Marshal output of an internal struct, not user input
	s.render(w, "form.html", map[string]any{
		"EditMode":    true,
		"EditID":      pw.Playbook.ID,
		"CityName":    pw.Jurisdiction.Name,
		"TopicSlug":   pw.Topic.Slug,
		"Title":       pw.Playbook.Title,
		"Intro":       pw.Playbook.IntroMD,
		"Error":       "",
		"PreloadJSON": template.JS(pj),
	})
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

	editErr := func(msg string) {
		editorial, _ := s.pg.GetEditorialSource(context.Background())
		pj, _ := json.Marshal(buildPreload(existing, editorial.ID))
		title := r.FormValue("title")
		if title == "" {
			title = existing.Playbook.Title
		}
		intro := r.FormValue("intro")
		if intro == "" {
			intro = existing.Playbook.IntroMD
		}
		//nolint:gosec // pj is json.Marshal output of an internal struct, not user input
		s.render(w, "form.html", map[string]any{
			"EditMode":    true,
			"EditID":      existing.Playbook.ID,
			"CityName":    existing.Jurisdiction.Name,
			"TopicSlug":   existing.Topic.Slug,
			"Title":       title,
			"Intro":       intro,
			"Error":       msg,
			"PreloadJSON": template.JS(pj),
		})
	}

	if err := r.ParseForm(); err != nil {
		editErr("Invalid form data: " + err.Error())
		return
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
				cites = append(cites, store.IngestCitationParams{SourceID: sourceByIdx[i].ID, Locator: loc})
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

	if err := s.pg.IngestPlaybook(ctx, store.IngestPlaybookParams{
		JurisdictionID: existing.Playbook.JurisdictionID,
		TopicID:        existing.Playbook.TopicID,
		Language:       existing.Playbook.Language,
		Slug:           existing.Playbook.Slug,
		Title:          title,
		IntroMD:        strings.TrimSpace(r.FormValue("intro")),
		Statements:     statements,
		Status:         existing.Playbook.Status,
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

func (s *srv) formError(w http.ResponseWriter, msg string) {
	cities, _ := s.pg.ListCityJurisdictions(context.Background())
	s.render(w, "form.html", map[string]any{
		"Cities":      cities,
		"Error":       msg,
		"PreloadJSON": template.JS("null"),
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
	Cites     []int             `json:"cites"`   // indices into sources slice
	Locators  map[string]string `json:"locators"` // "srcIdx" -> locator override
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
		ps := preloadStmt{ID: i, Body: stmt.BodyMD, Locators: map[string]string{}}
		for _, c := range stmt.Citations {
			if c.SourceID == editorialSourceID {
				ps.Editorial = true
			} else if m, ok := srcByID[c.SourceID]; ok {
				ps.Cites = append(ps.Cites, m.idx)
				if c.Locator != "" {
					ps.Locators[strconv.Itoa(m.idx)] = c.Locator
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

func slugToTitle(slug string) string {
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if len(p) == 0 {
			continue
		}
		r := []rune(p)
		r[0] = unicode.ToUpper(r[0])
		parts[i] = string(r)
	}
	return strings.Join(parts, " ")
}
