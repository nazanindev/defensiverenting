package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nazanindev/defensiverenting/internal/drafting"
	"github.com/nazanindev/defensiverenting/internal/sourcecheck"
	"github.com/nazanindev/defensiverenting/internal/store"
)

// One screen for review work (ADR-019, amended 2026-09-12): the statements
// of one page, one source, or one concept, each with its quotes, and one
// button per statement: Done. Done records that the reviewer read the
// statement with its evidence: it attests quotes nobody confirmed, records
// the drafting agent's notes as read, and stamps the statement. Publishing
// appears when a page has nothing left to do.

// stmtCard is what the "stmtcard" template renders.
type stmtCard struct {
	PlaybookID   int64
	PageStatus   string
	PageKind     string
	Jurisdiction string
	Topic        string
	Position     int
	Stmt         store.CitedStatement
	Standing     store.Standing
	// Done: nothing left on this statement.
	Done bool
	// Broken: the words or evidence need an edit before Done means anything.
	Broken bool
	// ShowPage puts the page name in the card, for cross-page lists.
	ShowPage bool
	// F is the screen's filter, so the Done form returns to it.
	F filter
}

func (c stmtCard) CardIndex() int { return c.Position - 1 }

func cardFromRow(r store.ReviewRow, f filter) stmtCard {
	st := r.Stmt.Standing()
	return stmtCard{
		PlaybookID: r.PlaybookID, PageStatus: r.PageStatus, PageKind: r.PageKind,
		Jurisdiction: r.Jurisdiction, Topic: r.Topic, Position: r.Position,
		Stmt: r.Stmt, Standing: st, Done: st.Status == store.Ready,
		Broken:   st.Has(store.ReasonEmpty) || st.Has(store.ReasonUncited) || st.Has(store.ReasonQuoteMissing) || st.Has(store.ReasonStatuteLocator),
		ShowPage: f.Page == 0, F: f,
	}
}

var slugRE = regexp.MustCompile(`^[a-z0-9-]{1,80}$`)

// filter is the screen's scope, rebuilt from whitelisted fields so a form
// can return to it without carrying a path.
type filter struct {
	Page    int64
	Source  int64
	Concept string
	Notes   bool
}

func readFilter(v url.Values) filter {
	var f filter
	f.Page, _ = strconv.ParseInt(v.Get("page"), 10, 64)
	f.Source, _ = strconv.ParseInt(v.Get("source"), 10, 64)
	if c := v.Get("concept"); slugRE.MatchString(c) {
		f.Concept = c
	}
	f.Notes = v.Get("notes") == "1"
	return f
}

// The result is built only from parsed integers, a slug that matched slugRE,
// and a message the handler wrote, so it cannot point off this site.
func (f filter) path(msg string) string {
	q := url.Values{}
	if f.Page > 0 {
		q.Set("page", strconv.FormatInt(f.Page, 10))
	}
	if f.Source > 0 {
		q.Set("source", strconv.FormatInt(f.Source, 10))
	}
	if f.Concept != "" {
		q.Set("concept", f.Concept)
	}
	if f.Notes {
		q.Set("notes", "1")
	}
	if msg != "" {
		q.Set("msg", msg)
	}
	if len(q) == 0 {
		return "/statements"
	}
	return "/statements?" + q.Encode()
}

func (f filter) Empty() bool { return f.Page == 0 && f.Source == 0 && f.Concept == "" && !f.Notes }

// statements renders the screen for the filter in the query.
func (s *srv) statements(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	f := readFilter(r.URL.Query())
	data := map[string]any{"Actor": actor(r), "F": f, "Msg": r.URL.Query().Get("msg")}

	// The pickers: sources and concepts with work left, for the filter bar.
	sources, err := s.pg.ReviewSourcesOverview(ctx)
	if err != nil {
		s.serverError(w, err)
		return
	}
	concepts, err := s.pg.ReviewConceptsOverview(ctx)
	if err != nil {
		s.serverError(w, err)
		return
	}
	data["Sources"], data["Concepts"] = sources, concepts

	var rows []store.ReviewRow
	switch {
	case f.Page > 0:
		pw, err := s.pg.AuthorGetPlaybook(ctx, f.Page)
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			s.serverError(w, err)
			return
		}
		issues, err := s.pg.AuthorPlaybookIssues(ctx, f.Page)
		if err != nil {
			s.serverError(w, err)
			return
		}
		data["Page"], data["Issues"] = pw, issueDetails(issues)
		for i, st := range pw.Statements {
			rows = append(rows, store.ReviewRow{
				PlaybookID: pw.Playbook.ID, PageTitle: pw.Playbook.Title, PageStatus: pw.Playbook.Status, PageKind: pw.Playbook.PageKind,
				Jurisdiction: pw.Jurisdiction.Name, Topic: pw.Topic.Slug, Position: i + 1, Stmt: st,
			})
		}
	case f.Source > 0:
		for i := range sources {
			if sources[i].SourceID == f.Source {
				data["Source"] = &sources[i]
			}
		}
		if data["Source"] == nil {
			http.Error(w, "no page cites this source", http.StatusNotFound)
			return
		}
		if rows, err = s.pg.ReviewStatementsBySource(ctx, f.Source); err != nil {
			s.serverError(w, err)
			return
		}
	case f.Concept != "":
		if rows, err = s.pg.ReviewStatementsByConcept(ctx, f.Concept); err != nil {
			s.serverError(w, err)
			return
		}
	case f.Notes:
		if rows, err = s.pg.ReviewStatementsWithNotes(ctx); err != nil {
			s.serverError(w, err)
			return
		}
	}
	cards := make([]stmtCard, 0, len(rows))
	todo := 0
	for _, row := range rows {
		c := cardFromRow(row, f)
		if !c.Done {
			todo++
		}
		cards = append(cards, c)
	}
	data["Cards"], data["Todo"] = cards, todo
	s.render(w, "statements.html", data)
}

// statementDone is the one action: the reviewer read this statement with
// its quotes.
func (s *srv) statementDone(w http.ResponseWriter, r *http.Request) {
	pid, err := strconv.ParseInt(r.FormValue("playbook"), 10, 64)
	if err != nil {
		http.Error(w, "invalid playbook", http.StatusBadRequest)
		return
	}
	f := readFilter(r.Form)
	err = s.pg.MarkStatementDone(r.Context(), pid, r.FormValue("key"), actor(r))
	switch {
	case errors.Is(err, store.ErrChangeProposed):
		http.Redirect(w, r, f.path("Not done: a change is proposed for that statement. Decide it in the queue first."), http.StatusSeeOther) //nolint:gosec // see filter.path: typed fields, never a client path
	case errors.Is(err, store.ErrNotFound):
		http.Redirect(w, r, f.path("That statement is no longer on the page."), http.StatusSeeOther) //nolint:gosec // see filter.path: typed fields, never a client path
	case err != nil:
		s.serverError(w, err)
	default:
		http.Redirect(w, r, f.path(""), http.StatusSeeOther) //nolint:gosec // see filter.path: typed fields, never a client path
	}
}

// statementSave is edit in place: the card's form posts the statement's
// text and, per citation, its quote and locator. A quote that changed is
// checked against the fetched source text the way the editor does; found,
// it is saved confirmed under the reviewer's name, otherwise unconfirmed
// and the gate holds until it is. Everything else about the statement
// (its tags, which sources it cites) is kept as it was.
func (s *srv) statementSave(w http.ResponseWriter, r *http.Request) {
	pid, err := strconv.ParseInt(r.FormValue("playbook"), 10, 64)
	if err != nil {
		http.Error(w, "invalid playbook", http.StatusBadRequest)
		return
	}
	key := r.FormValue("key")
	f := readFilter(r.Form)
	ctx := r.Context()
	pw, err := s.pg.AuthorGetPlaybook(ctx, pid)
	if err != nil {
		s.serverError(w, err)
		return
	}
	var cur *store.CitedStatement
	for i := range pw.Statements {
		if pw.Statements[i].Key == key {
			cur = &pw.Statements[i]
		}
	}
	if cur == nil {
		http.Redirect(w, r, f.path("That statement is no longer on the page."), http.StatusSeeOther) //nolint:gosec // see filter.path
		return
	}
	st := store.IngestStatementParams{
		BodyMD: strings.TrimSpace(r.FormValue("body")), ConceptSlug: cur.ConceptSlug, TopicRefSlug: cur.TopicRefSlug,
	}
	qv := newQuoteVerifier(s.pg, s.sourceCache)
	for _, c := range cur.Citations {
		cite := store.IngestCitationParams{SourceID: c.SourceID, Locator: c.Locator, Quote: c.Quote, ManuallyVerified: c.ManuallyVerified}
		if c.SourceKind != "editorial" {
			sid := strconv.FormatInt(c.SourceID, 10)
			if v, ok := r.Form["locator_"+sid]; ok {
				cite.Locator = strings.TrimSpace(v[0])
			}
			if v, ok := r.Form["quote_"+sid]; ok && strings.TrimSpace(v[0]) != c.Quote {
				cite.Quote = strings.TrimSpace(v[0])
				cite.ManuallyVerified = false
				if cite.Quote != "" {
					if res := qv.checkQuote(ctx, c.SourceURL, cite.Quote); res.Verified {
						cite.CheckedNow, cite.CheckedBy, cite.Checked = true, actor(r), res.Receipt
					}
				}
			}
		}
		st.Sources = append(st.Sources, cite)
	}
	err = s.pg.ReplaceStatement(ctx, pid, key, st, actor(r))
	var npe *store.NotPublishableError
	switch {
	case errors.As(err, &npe):
		http.Redirect(w, r, f.path("Not saved: this page is live and the change would leave it unpublishable. "+strings.Join(issueDetails(npe.Issues), "; ")), http.StatusSeeOther) //nolint:gosec // see filter.path
	case err != nil:
		s.serverError(w, err)
	default:
		http.Redirect(w, r, f.path("Saved."), http.StatusSeeOther) //nolint:gosec // see filter.path
	}
}

// sourceRecheck re-fetches one source and confirms its quotes now.
func (s *srv) sourceRecheck(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	f := readFilter(r.Form)
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	res, err := sourcecheck.RunSource(ctx, s.pg, drafting.FetchExtract, id, func(format string, a ...any) {
		s.log.Info("sourcecheck", slog.String("msg", fmt.Sprintf(format, a...)))
	})
	if err != nil {
		s.serverError(w, err)
		return
	}
	msg := "Rechecked."
	switch {
	case res.Failed > 0 || res.Unreadable > 0:
		msg = "The checker could not read this source. Open it yourself; Done records what you found."
	case res.Drifted > 0:
		msg = fmt.Sprintf("Rechecked: %d quote(s) no longer found; drift items were filed in the queue.", res.Proposed)
	case res.Sources > 0:
		msg = "Rechecked: every quote still appears at the source."
	}
	http.Redirect(w, r, f.path(msg), http.StatusSeeOther) //nolint:gosec // see filter.path: typed fields, never a client path
}

// publishReady publishes every draft with nothing left to do, through the
// ordinary gate, and reports each outcome on the dashboard.
func (s *srv) publishReady(w http.ResponseWriter, r *http.Request) {
	out, err := s.pg.PublishReadyDrafts(r.Context(), actor(r))
	if err != nil {
		s.serverError(w, err)
		return
	}
	var published, held []string
	for _, o := range out {
		name := o.Jurisdiction + " · " + o.Topic
		switch {
		case o.Published:
			published = append(published, name)
		case o.Err != nil:
			held = append(held, name+" (error: "+o.Err.Error()+")")
		default:
			held = append(held, fmt.Sprintf("%s (%d issue(s))", name, len(o.Issues)))
		}
	}
	msg := fmt.Sprintf("Published %d page(s)", len(published))
	if len(published) > 0 {
		msg += ": " + strings.Join(published, "; ")
	}
	msg += "."
	if len(held) > 0 {
		msg += fmt.Sprintf(" %d draft(s) still held: %s.", len(held), strings.Join(held, "; "))
	}
	http.Redirect(w, r, "/?status=draft&msg="+url.QueryEscape(msg), http.StatusSeeOther)
}
