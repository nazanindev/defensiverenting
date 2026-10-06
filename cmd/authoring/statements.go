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
	sitehandlers "github.com/nazanindev/defensiverenting/internal/http/handlers"
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
	// Changes are the pending replacements and drift findings on this
	// statement, decided here rather than on the queue page.
	Changes []queueItem
	// HeldBy is the host of a contact-first org this statement cites that has
	// not said yes (ADR-029 D3). Hiding says whether the live site leaves the
	// statement out now or will once hiding turns on.
	HeldBy string
	Hiding bool
	// Question is the renter's question a rules-page statement answers,
	// with the place added (ADR-028 D4). The live page shows it as the
	// heading; without it here, an answer that opens "Yes." or "It
	// depends" has nothing to answer.
	Question string
	// StageHead is the stage heading (ADR-028 D10) the live page shows
	// above this statement: set on the first card of each run, on a single
	// page's screen only.
	StageHead string
}

func (c stmtCard) CardIndex() int { return c.Position - 1 }

// citeGroup is one quote block on a card: citations of one source and
// locator that sit next to each other. A two-column PDF can only be quoted
// one printed line at a time, so one handbook paragraph arrives as eight
// citations; shown one by one it reads as a broken list.
type citeGroup struct {
	First       store.CitationWithSource
	Quote       string // the fragments joined with " … "
	Unconfirmed bool   // a fragment has a quote nobody confirmed
}

// CiteGroups joins neighbouring citations of the same source and locator,
// leaving site guidance out as the card always has.
func (c stmtCard) CiteGroups() []citeGroup {
	var out []citeGroup
	for _, ct := range c.Stmt.Citations {
		if ct.SourceKind == "editorial" {
			continue
		}
		unconfirmed := ct.Quote != "" && ct.CheckedAt == nil
		if n := len(out); n > 0 && out[n-1].First.SourceID == ct.SourceID && out[n-1].First.Locator == ct.Locator && ct.Quote != "" && out[n-1].Quote != "" {
			out[n-1].Quote += " … " + ct.Quote
			out[n-1].Unconfirmed = out[n-1].Unconfirmed || unconfirmed
			continue
		}
		out = append(out, citeGroup{First: ct, Quote: ct.Quote, Unconfirmed: unconfirmed})
	}
	return out
}

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

var (
	slugRE = regexp.MustCompile(`^[a-z0-9-]{1,80}$`)
	// cardRE matches a statement card's element id, p{playbook}-{position}.
	cardRE = regexp.MustCompile(`^p[0-9]{1,12}-[0-9]{1,6}$`)
)

// filter is the screen's scope, rebuilt from whitelisted fields so a form
// can return to it without carrying a path.
type filter struct {
	Page    int64
	Source  int64
	Concept string
	Notes   bool
	// At is the card the form was posted from, so the screen returns to
	// it instead of the top of the list. Only a card id matching cardRE.
	At string
}

func readFilter(v url.Values) filter {
	var f filter
	f.Page, _ = strconv.ParseInt(v.Get("page"), 10, 64)
	f.Source, _ = strconv.ParseInt(v.Get("source"), 10, 64)
	if c := v.Get("concept"); slugRE.MatchString(c) {
		f.Concept = c
	}
	f.Notes = v.Get("notes") == "1"
	if at := v.Get("at"); cardRE.MatchString(at) {
		f.At = at
	}
	return f
}

// The result is built only from parsed integers, a slug that matched slugRE,
// a card id that matched cardRE, and a message the handler wrote, so it
// cannot point off this site. With no message the path ends at the card
// the form came from; a message is shown at the top, so it wins.
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
	p := "/statements"
	if len(q) > 0 {
		p += "?" + q.Encode()
	}
	if msg == "" && f.At != "" {
		p += "#" + f.At
	}
	return p
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
		flags, err := s.pg.OpenPageFlags(ctx, f.Page)
		if err != nil {
			s.serverError(w, err)
			return
		}
		data["Page"], data["Issues"], data["PageFlags"] = pw, issueDetails(issues), flags
		data["IssuesTip"] = strings.Join(issueDetails(issues), "\n")
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
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.Stmt.ProposalPending {
			keys = append(keys, row.Stmt.Key)
		}
	}
	changes, err := s.pg.PendingChangesByKeys(ctx, keys)
	if err != nil {
		s.serverError(w, err)
		return
	}
	ids := make([]int64, len(rows))
	for i, row := range rows {
		ids[i] = row.Stmt.ID
	}
	heldBy, err := s.pg.StatementsHeldBy(ctx, ids)
	if err != nil {
		s.serverError(w, err)
		return
	}
	hiding, err := s.pg.HelpHiding(ctx)
	if err != nil {
		s.serverError(w, err)
		return
	}
	questions := rulesQuestions(ctx, s.pg, rows)
	cards := make([]stmtCard, 0, len(rows))
	todo := 0
	lastStage := ""
	for _, row := range rows {
		c := cardFromRow(row, f)
		if f.Page != 0 && row.Stmt.Stage != "" && row.Stmt.Stage != lastStage {
			c.StageHead = row.Stmt.Stage
		}
		lastStage = row.Stmt.Stage
		c.HeldBy, c.Hiding = heldBy[row.Stmt.ID], hiding
		if row.PageKind == "rules" {
			if q, ok := questions[row.Stmt.ConceptSlug]; ok {
				c.Question = sitehandlers.QuestionIn(q[0], q[1], row.Jurisdiction)
			}
		}
		for _, pr := range changes[row.Stmt.Key] {
			c.Changes = append(c.Changes, newQueueItem(pr))
		}
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

// pageFlagDone closes a page flag: the reviewer read the doubt about the
// page and dealt with it, or decided it stands as written.
func (s *srv) pageFlagDone(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.FormValue("flag"), 10, 64)
	if err != nil {
		http.Error(w, "invalid flag", http.StatusBadRequest)
		return
	}
	f := readFilter(r.Form)
	switch err := s.pg.ClosePageFlag(r.Context(), id, actor(r), r.FormValue("note")); {
	case errors.Is(err, store.ErrPageFlagNotOpen):
		http.Redirect(w, r, f.path("That page flag is already closed."), http.StatusSeeOther) //nolint:gosec // see filter.path
	case err != nil:
		s.serverError(w, err)
	default:
		http.Redirect(w, r, f.path(""), http.StatusSeeOther) //nolint:gosec // see filter.path
	}
}

// statementFlag is the reader's doubt: they read the statement as a renter
// would and something looked wrong. It files a reviewer note under their
// name for the triage agent to answer (ADR-024), and the open item stops the
// page publishing until it is decided.
func (s *srv) statementFlag(w http.ResponseWriter, r *http.Request) {
	pid, err := strconv.ParseInt(r.FormValue("playbook"), 10, 64)
	if err != nil {
		http.Error(w, "invalid playbook", http.StatusBadRequest)
		return
	}
	f := readFilter(r.Form)
	err = s.pg.FlagStatement(r.Context(), pid, r.FormValue("key"), r.FormValue("note"), actor(r))
	switch {
	case errors.Is(err, store.ErrNotFound):
		http.Redirect(w, r, f.path("That statement is no longer on the page."), http.StatusSeeOther) //nolint:gosec // see filter.path
	case err != nil:
		http.Redirect(w, r, f.path("Not flagged: "+err.Error()), http.StatusSeeOther) //nolint:gosec // see filter.path
	default:
		http.Redirect(w, r, f.path(""), http.StatusSeeOther) //nolint:gosec // see filter.path
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
		http.Redirect(w, r, f.path("Not saved: this page is live and the change adds a problem it did not have: "+strings.Join(issueDetails(npe.Issues), "; ")), http.StatusSeeOther) //nolint:gosec // see filter.path
	case err != nil:
		s.serverError(w, err)
	default:
		http.Redirect(w, r, f.path(""), http.StatusSeeOther) //nolint:gosec // see filter.path
	}
}

// statementChange decides a replacement or drift finding from the card:
// apply it (the reviewer may have edited the proposed text first) or keep
// the statement as it is.
func (s *srv) statementChange(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.FormValue("proposal"), 10, 64)
	if err != nil {
		http.Error(w, "invalid proposal", http.StatusBadRequest)
		return
	}
	f := readFilter(r.Form)
	ctx := r.Context()
	p, err := s.pg.GetProposal(ctx, id)
	if err != nil {
		s.serverError(w, err)
		return
	}
	switch r.FormValue("verdict") {
	case "apply":
		if p.Proposed == nil {
			err = s.pg.DecideProposal(ctx, id, "approved", actor(r), "Resolved on the statement", nil)
		} else {
			err = s.applyApproval(ctx, p, strings.TrimSpace(r.FormValue("body")), actor(r))
		}
	case "keep":
		note := strings.TrimSpace(r.FormValue("note"))
		if note == "" {
			note = "Kept as written"
		}
		err = s.pg.DecideProposal(ctx, id, "rejected", actor(r), note, nil)
	default:
		http.Error(w, "unknown verdict", http.StatusBadRequest)
		return
	}
	var npe *store.NotPublishableError
	switch {
	case errors.As(err, &npe):
		http.Redirect(w, r, f.path("Not applied: the page is live and this change adds a problem it did not have: "+strings.Join(issueDetails(npe.Issues), "; ")), http.StatusSeeOther) //nolint:gosec // see filter.path
	case err != nil:
		http.Redirect(w, r, f.path("Not applied: "+err.Error()), http.StatusSeeOther) //nolint:gosec // see filter.path
	default:
		http.Redirect(w, r, f.path(""), http.StatusSeeOther) //nolint:gosec // see filter.path
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

// rulesQuestions maps concept slug to its question and name when any row is
// on a rules page; nil otherwise, so other screens skip the query.
func rulesQuestions(ctx context.Context, pg store.Store, rows []store.ReviewRow) map[string][2]string {
	need := false
	for _, r := range rows {
		if r.PageKind == "rules" {
			need = true
			break
		}
	}
	if !need {
		return nil
	}
	concepts, err := pg.ListConcepts(ctx)
	if err != nil {
		return nil
	}
	out := make(map[string][2]string, len(concepts))
	for _, c := range concepts {
		out[c.Slug] = [2]string{c.Question, c.Name}
	}
	return out
}
