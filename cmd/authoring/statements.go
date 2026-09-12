package main

import (
	"context"
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

// The statement is the unit of work (ADR-018, ADR-019). One card renders a
// statement with its standing and every action that clears a reason, and
// every surface that shows statements shows that card: the page view, and
// the grouped list here (by source, by concept, by reviewer note). The
// card's forms carry a return target as two whitelisted fields, so the
// reviewer lands back on the same grouping after each action.

// stmtCard is what the "stmtcard" template renders.
type stmtCard struct {
	PlaybookID   int64
	PageTitle    string
	PageStatus   string
	PageKind     string
	Jurisdiction string
	Topic        string
	Position     int
	Stmt         store.CitedStatement
	Standing     store.Standing
	// PageLevel: a directory entry, reviewed as a page at publish.
	PageLevel bool
	// Focus is the citation the current grouping is about (by source).
	Focus *store.CitationWithSource
	// ShowPage puts the page link in the card header, for cross-page lists.
	ShowPage bool
	// RetBy and RetID say where the card's forms return to: page/<id>,
	// source/<id>, concept/<slug>, note/.
	RetBy string
	RetID string
}

// CardIndex is the editor's zero-based card id for this statement.
func (c stmtCard) CardIndex() int { return c.Position - 1 }

// Item is what a group-level mark-reviewed form posts back.
func (c stmtCard) Item() string { return fmt.Sprintf("%d:%s", c.PlaybookID, c.Stmt.Key) }

// CanReview reports whether the stamp would be accepted now: the statement
// is reviewed per statement and carries no undecided item.
func (c stmtCard) CanReview() bool {
	return !c.PageLevel && c.Stmt.ReviewedAt == nil && !c.Stmt.Undecided
}

// cardFromRow builds a card for a cross-page listing.
func cardFromRow(r store.ReviewRow, by, id string) stmtCard {
	pageLevel := r.PageKind == "directory"
	return stmtCard{
		PlaybookID: r.PlaybookID, PageTitle: r.PageTitle, PageStatus: r.PageStatus, PageKind: r.PageKind,
		Jurisdiction: r.Jurisdiction, Topic: r.Topic, Position: r.Position,
		Stmt: r.Stmt, Standing: r.Stmt.Standing(pageLevel), PageLevel: pageLevel,
		ShowPage: true, RetBy: by, RetID: id,
	}
}

var slugRE = regexp.MustCompile(`^[a-z0-9-]{1,80}$`)

// returnPath rebuilds the grouping URL from the two whitelisted fields. It
// never uses a path the client supplied.
func returnPath(by, id string) string {
	switch by {
	case "page":
		if n, err := strconv.ParseInt(id, 10, 64); err == nil {
			return fmt.Sprintf("/view/%d", n)
		}
	case "source":
		if n, err := strconv.ParseInt(id, 10, 64); err == nil {
			return fmt.Sprintf("/statements?by=source&id=%d", n)
		}
	case "concept":
		if slugRE.MatchString(id) {
			return "/statements?by=concept&slug=" + id
		}
	case "note":
		return "/statements?by=note"
	}
	return "/statements"
}

func redirectBack(w http.ResponseWriter, r *http.Request, msg string) {
	http.Redirect(w, r, returnPath(r.FormValue("by"), r.FormValue("id"))+sep(returnPath(r.FormValue("by"), r.FormValue("id")))+"msg="+url.QueryEscape(msg), http.StatusSeeOther)
}

func sep(path string) string {
	if strings.Contains(path, "?") {
		return "&"
	}
	return "?"
}

// statements is the grouped list: ?by=source[&id=], ?by=concept[&slug=],
// ?by=note. Without a group chosen it shows the picker for that grouping.
func (s *srv) statements(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	by := r.URL.Query().Get("by")
	if by == "" {
		by = "source"
	}
	data := map[string]any{"Actor": actor(r), "By": by, "Msg": r.URL.Query().Get("msg")}

	notes, err := s.pg.ReviewStatementsWithNotes(ctx)
	if err != nil {
		s.serverError(w, err)
		return
	}
	data["NoteCount"] = len(notes)

	switch by {
	case "source":
		all, err := s.pg.ReviewSourcesOverview(ctx)
		if err != nil {
			s.serverError(w, err)
			return
		}
		open := 0
		for _, x := range all {
			if x.Unreviewed > 0 || x.Unconfirmed > 0 {
				open++
			}
		}
		data["Sources"], data["SourcesOpen"] = all, open
		if idStr := r.URL.Query().Get("id"); idStr != "" {
			id, err := strconv.ParseInt(idStr, 10, 64)
			if err != nil {
				http.Error(w, "invalid id", http.StatusBadRequest)
				return
			}
			var chosen *store.SourceReviewSummary
			for i := range all {
				if all[i].SourceID == id {
					chosen = &all[i]
				}
			}
			if chosen == nil {
				http.Error(w, "no page cites this source", http.StatusNotFound)
				return
			}
			rows, err := s.pg.ReviewStatementsBySource(ctx, id)
			if err != nil {
				s.serverError(w, err)
				return
			}
			cards := make([]stmtCard, 0, len(rows))
			for _, row := range rows {
				c := cardFromRow(row, "source", idStr)
				for i := range row.Stmt.Citations {
					if row.Stmt.Citations[i].SourceID == id {
						c.Focus = &row.Stmt.Citations[i]
					}
				}
				cards = append(cards, c)
			}
			data["Source"], data["Cards"], data["Summary"] = chosen, cards, summarize(cards)
		}
	case "concept":
		all, err := s.pg.ReviewConceptsOverview(ctx)
		if err != nil {
			s.serverError(w, err)
			return
		}
		open := 0
		for _, x := range all {
			if x.Unreviewed > 0 {
				open++
			}
		}
		data["Concepts"], data["ConceptsOpen"] = all, open
		if slug := r.URL.Query().Get("slug"); slug != "" {
			if !slugRE.MatchString(slug) {
				http.Error(w, "invalid slug", http.StatusBadRequest)
				return
			}
			rows, err := s.pg.ReviewStatementsByConcept(ctx, slug)
			if err != nil {
				s.serverError(w, err)
				return
			}
			if len(rows) == 0 {
				http.Error(w, "no page carries this concept", http.StatusNotFound)
				return
			}
			name := slug
			for _, c := range all {
				if c.Slug == slug {
					name = c.Name
				}
			}
			cards := make([]stmtCard, 0, len(rows))
			for _, row := range rows {
				cards = append(cards, cardFromRow(row, "concept", slug))
			}
			data["Concept"], data["ConceptName"], data["Cards"], data["Summary"] = slug, name, cards, summarize(cards)
		}
	case "note":
		cards := make([]stmtCard, 0, len(notes))
		for _, row := range notes {
			cards = append(cards, cardFromRow(row, "note", ""))
		}
		data["Cards"], data["Summary"] = cards, summarize(cards)
	default:
		http.Error(w, "unknown grouping", http.StatusBadRequest)
		return
	}
	s.render(w, "statements.html", data)
}

// cardSummary is the group's standing in one line.
type cardSummary struct {
	store.PageStanding
	// Reviewable counts cards whose stamp would be accepted now.
	Reviewable int
}

func summarize(cards []stmtCard) cardSummary {
	var sum cardSummary
	for _, c := range cards {
		sum.Aggregate(c.Standing)
		if c.CanReview() {
			sum.Reviewable++
		}
	}
	return sum
}

// statementReview stamps one statement (the card's "reviewed" action).
func (s *srv) statementReview(w http.ResponseWriter, r *http.Request) {
	pid, key, ok := cardTarget(w, r)
	if !ok {
		return
	}
	res, err := s.pg.MarkStatementsReviewed(r.Context(), pid, []string{key}, actor(r))
	if err != nil {
		s.serverError(w, err)
		return
	}
	if res.Undecided > 0 {
		redirectBack(w, r, "Not stamped: decide the open item on this statement first.")
		return
	}
	redirectBack(w, r, "Marked reviewed.")
}

// statementAttest attests the unconfirmed quotes on one statement whose
// sources the checker cannot read.
func (s *srv) statementAttest(w http.ResponseWriter, r *http.Request) {
	pid, key, ok := cardTarget(w, r)
	if !ok {
		return
	}
	n, err := s.pg.AttestStatementQuotes(r.Context(), pid, key, actor(r))
	if err != nil {
		s.serverError(w, err)
		return
	}
	redirectBack(w, r, fmt.Sprintf("Attested %d quote(s) under your name.", n))
}

// statementDecide answers a reviewer note from the card: "fine" records that
// the statement stands as written, "fixed" that the edit was made.
func (s *srv) statementDecide(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.FormValue("proposal"), 10, 64)
	if err != nil {
		http.Error(w, "invalid proposal", http.StatusBadRequest)
		return
	}
	var status, note string
	switch r.FormValue("verdict") {
	case "fine":
		status, note = "rejected", "Read; the statement stands as written"
	case "fixed":
		status = "approved"
	default:
		http.Error(w, "unknown verdict", http.StatusBadRequest)
		return
	}
	if err := s.pg.DecideProposal(r.Context(), id, status, actor(r), note, nil); err != nil {
		s.serverError(w, err)
		return
	}
	redirectBack(w, r, "Decided.")
}

// statementsMarkGroup stamps every statement a group page posted back.
func (s *srv) statementsMarkGroup(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	byPage := map[int64][]string{}
	var order []int64
	for _, item := range r.Form["item"] {
		pid, key, ok := strings.Cut(item, ":")
		id, err := strconv.ParseInt(pid, 10, 64)
		if !ok || err != nil {
			http.Error(w, "bad item", http.StatusBadRequest)
			return
		}
		if _, seen := byPage[id]; !seen {
			order = append(order, id)
		}
		byPage[id] = append(byPage[id], key)
	}
	var total store.ReviewStamp
	for _, id := range order {
		res, err := s.pg.MarkStatementsReviewed(r.Context(), id, byPage[id], actor(r))
		if err != nil {
			s.serverError(w, err)
			return
		}
		total.Stamped += res.Stamped
		total.Undecided += res.Undecided
	}
	msg := fmt.Sprintf("Marked %d statement(s) reviewed.", total.Stamped)
	if total.Undecided > 0 {
		msg += fmt.Sprintf(" %d skipped: an item is still undecided.", total.Undecided)
	}
	redirectBack(w, r, msg)
}

// sourceAttest attests every unconfirmed quote from one unreadable source.
func (s *srv) sourceAttest(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	n, err := s.pg.AttestSourceQuotes(r.Context(), id, actor(r))
	if err != nil {
		s.serverError(w, err)
		return
	}
	redirectBack(w, r, fmt.Sprintf("Attested %d quote(s) from this source under your name.", n))
}

// sourceRecheck re-fetches one source and confirms its quotes now.
func (s *srv) sourceRecheck(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	var last string
	res, err := sourcecheck.RunSource(ctx, s.pg, drafting.FetchExtract, id, func(format string, a ...any) {
		last = fmt.Sprintf(format, a...)
		s.log.Info("sourcecheck", slog.String("msg", last))
	})
	if err != nil {
		s.serverError(w, err)
		return
	}
	msg := "Rechecked."
	switch {
	case res.Failed > 0 || res.Unreadable > 0:
		msg = "The checker could not read this source. Open it yourself and attest the quotes you find."
	case res.Drifted > 0:
		msg = fmt.Sprintf("Rechecked: %d quote(s) no longer found; drift items were filed in the queue.", res.Proposed)
	case res.Sources > 0:
		msg = "Rechecked: every quote still appears at the source."
	}
	if last != "" {
		msg += " " + last
	}
	redirectBack(w, r, msg)
}

// cardTarget reads the statement a card form names.
func cardTarget(w http.ResponseWriter, r *http.Request) (int64, string, bool) {
	pid, err := strconv.ParseInt(r.FormValue("playbook"), 10, 64)
	if err != nil {
		http.Error(w, "invalid playbook", http.StatusBadRequest)
		return 0, "", false
	}
	return pid, r.FormValue("key"), true
}

// publishReady publishes every draft with no critical issue, through the
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
		msg += fmt.Sprintf(" %d draft(s) still held by the gate: %s.", len(held), strings.Join(held, "; "))
	}
	http.Redirect(w, r, "/?status=draft&msg="+url.QueryEscape(msg), http.StatusSeeOther)
}
