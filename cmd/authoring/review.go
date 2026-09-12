package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/nazanindev/defensiverenting/internal/drafting"
	"github.com/nazanindev/defensiverenting/internal/sourcecheck"
	"github.com/nazanindev/defensiverenting/internal/store"
)

// Group review (ADR-018 D4, D5). The page view stamps statements one page
// at a time; these pages stamp them one source or one concept at a time,
// across every page, since a stamp lives on the claim and not on the page.
// Every list renders each statement in full with its quotes before offering
// to stamp the set: the stamp is over what the reviewer had in front of them.

// reviewIndex lists the three groupings: sources and concepts with work
// left, and the count of reviewer notes waiting in the queue.
func (s *srv) reviewIndex(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
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
	notes, err := s.pg.ListProposalsByReason(ctx, "pending", "note")
	if err != nil {
		s.serverError(w, err)
		return
	}
	var srcOpen, conOpen int
	for _, x := range sources {
		if x.Unreviewed > 0 || x.Unconfirmed > 0 {
			srcOpen++
		}
	}
	for _, x := range concepts {
		if x.Unreviewed > 0 {
			conOpen++
		}
	}
	s.render(w, "review.html", map[string]any{
		"Actor":        actor(r),
		"Sources":      sources,
		"Concepts":     concepts,
		"SourcesOpen":  srcOpen,
		"ConceptsOpen": conOpen,
		"Notes":        len(notes),
		"Msg":          r.URL.Query().Get("msg"),
	})
}

// reviewGroupItem is one statement as the group pages render it.
type reviewGroupItem struct {
	store.ReviewRow
	// Focus is the citation this group is about (by source), so the template
	// can show its quote first; nil on a concept group.
	Focus *store.CitationWithSource
	// Item is "playbookID:key", what the mark-reviewed form posts back.
	Item string
}

// CardIndex is the editor's zero-based card id for this statement.
func (it reviewGroupItem) CardIndex() int { return it.Position - 1 }

func (s *srv) reviewSource(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	rows, err := s.pg.ReviewStatementsBySource(ctx, id)
	if err != nil {
		s.serverError(w, err)
		return
	}
	var summary store.SourceReviewSummary
	all, err := s.pg.ReviewSourcesOverview(ctx)
	if err != nil {
		s.serverError(w, err)
		return
	}
	for _, x := range all {
		if x.SourceID == id {
			summary = x
		}
	}
	if summary.SourceID == 0 {
		http.Error(w, "no page cites this source", http.StatusNotFound)
		return
	}
	items := make([]reviewGroupItem, 0, len(rows))
	for _, row := range rows {
		it := reviewGroupItem{ReviewRow: row, Item: fmt.Sprintf("%d:%s", row.PlaybookID, row.Stmt.Key)}
		for i := range row.Stmt.Citations {
			if row.Stmt.Citations[i].SourceID == id {
				it.Focus = &row.Stmt.Citations[i]
			}
		}
		items = append(items, it)
	}
	s.render(w, "review_group.html", map[string]any{
		"Actor":    actor(r),
		"Kind":     "source",
		"Source":   summary,
		"Title":    firstNonEmpty(summary.Publisher, summary.URL),
		"Items":    items,
		"Counts":   countReview(rows),
		"MarkPath": fmt.Sprintf("/review/source/%d/reviewed", id),
		"Msg":      r.URL.Query().Get("msg"),
	})
}

func (s *srv) reviewConcept(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	ctx := r.Context()
	rows, err := s.pg.ReviewStatementsByConcept(ctx, slug)
	if err != nil {
		s.serverError(w, err)
		return
	}
	if len(rows) == 0 {
		http.Error(w, "no page carries this concept", http.StatusNotFound)
		return
	}
	var name string
	if all, err := s.pg.ReviewConceptsOverview(ctx); err == nil {
		for _, c := range all {
			if c.Slug == slug {
				name = c.Name
			}
		}
	}
	items := make([]reviewGroupItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, reviewGroupItem{ReviewRow: row, Item: fmt.Sprintf("%d:%s", row.PlaybookID, row.Stmt.Key)})
	}
	s.render(w, "review_group.html", map[string]any{
		"Actor":    actor(r),
		"Kind":     "concept",
		"Concept":  slug,
		"Title":    firstNonEmpty(name, slug),
		"Items":    items,
		"Counts":   countReview(rows),
		"MarkPath": "/review/concept/" + url.PathEscape(slug) + "/reviewed",
		"Msg":      r.URL.Query().Get("msg"),
	})
}

type reviewCounts struct{ Total, Reviewed, Undecided, Directory int }

func countReview(rows []store.ReviewRow) reviewCounts {
	var c reviewCounts
	for _, r := range rows {
		c.Total++
		if r.Stmt.ReviewedAt != nil {
			c.Reviewed++
		}
		if r.Stmt.Undecided {
			c.Undecided++
		}
		if r.PageKind == "directory" {
			c.Directory++
		}
	}
	return c
}

// reviewMarkGroup stamps the statements a group page posted back, each on
// its own page. The form lists "playbookID:key" per statement it rendered.
func (s *srv) reviewMarkGroup(w http.ResponseWriter, r *http.Request) {
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
		msg += fmt.Sprintf(" %d skipped: a queue item is still undecided.", total.Undecided)
	}
	http.Redirect(w, r, backTo(r)+"?msg="+url.QueryEscape(msg), http.StatusSeeOther)
}

// reviewAttestSource records the reviewer's attestation for every
// unconfirmed quote from one source, after they opened it themselves.
func (s *srv) reviewAttestSource(w http.ResponseWriter, r *http.Request) {
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
	msg := fmt.Sprintf("Attested %d quote(s) from this source under your name.", n)
	http.Redirect(w, r, fmt.Sprintf("/review/source/%d?msg=%s", id, url.QueryEscape(msg)), http.StatusSeeOther)
}

// reviewRecheckSource re-fetches one source and confirms its quotes now,
// the whole-site check scoped to one URL. It runs inline: one fetch.
func (s *srv) reviewRecheckSource(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	var lines []string
	res, err := sourcecheck.RunSource(ctx, s.pg, drafting.FetchExtract, id, func(format string, a ...any) {
		line := fmt.Sprintf(format, a...)
		lines = append(lines, line)
		s.log.Info("sourcecheck", slog.String("msg", line))
	})
	if err != nil {
		s.serverError(w, err)
		return
	}
	msg := "Rechecked."
	switch {
	case res.Failed > 0 || res.Unreadable > 0:
		msg = "The checker could not read this source; attest the quotes by hand after opening it yourself."
	case res.Drifted > 0:
		msg = fmt.Sprintf("Rechecked: %d quote(s) no longer found; drift items were filed in the queue.", res.Proposed)
	case res.Sources > 0:
		msg = "Rechecked: every quote still appears at the source."
	}
	if len(lines) > 0 {
		msg += " " + lines[len(lines)-1]
	}
	http.Redirect(w, r, fmt.Sprintf("/review/source/%d?msg=%s", id, url.QueryEscape(msg)), http.StatusSeeOther)
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

func backTo(r *http.Request) string {
	if b := r.FormValue("back"); strings.HasPrefix(b, "/") && !strings.HasPrefix(b, "//") {
		return b
	}
	return "/review"
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
