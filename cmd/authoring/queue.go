package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nazanindev/defensiverenting/internal/drafting"
	"github.com/nazanindev/defensiverenting/internal/store"
)

// The review queue (ADR-014 D5): every pending statement proposal in one
// list, proposed edits first and then the checker's drift findings, each in
// reading order. One item is open at a time and a decision opens the next.
// Approving one is the reviewer's save of that page with the statement
// swapped (D3), so it is subject to everything a save is — the reference-only
// rule, live quote checks, the publish gate.

// sourceItem is one unused-source proposal (ADR-014 D7). Approving it
// deletes the source row; there is no page to save.
type sourceItem struct {
	store.SourceProposal
	Age string
}

type queueItem struct {
	store.ProposalRow
	EvidenceText string
	Age          string
	// ReasonLabel splits "agent-pass:rerun-sources" into a chip and a name.
	ReasonLabel string
	ReasonName  string
	// Drift is the checker's evidence read into shape for a source-drift
	// item, so the page can show what changed rather than a JSON blob.
	Drift *driftEvidence
	// Note is the drafting agent's doubt on a reviewer-flag item (ADR-018
	// D1), shown as the finding in its own words.
	Note string
	// Answers are the reviewer notes this edit resolves (evidence
	// "resolves"), shown above the replacement so the question sits next to
	// its answer. Approving the edit closes them.
	Answers []string
	// EditorHref opens the target page's editor scrolled to this statement;
	// "" when the statement is no longer on the page.
	EditorHref string
	// Why is the proposer's own account of the change (evidence "note"):
	// what a triage pass answered and did, or what the checker concluded.
	// It is the first line the reviewer reads.
	Why string
	// Next is the id of the item that follows this one in the queue, so a
	// decision can open it; 0 on the last.
	Next int64
	// NewCitations and GoneCitations are the citations the proposal adds
	// and drops against the statement as it reads now, so the page shows
	// only what changed in the evidence. Nil when the citations are the same.
	NewCitations  []store.ProposedCitation
	GoneCitations []store.ProposedCitation
}

// newQueueItem reads a proposal row into the shape the queue and the
// statement card both render.
func newQueueItem(row store.ProposalRow) queueItem {
	item := queueItem{ProposalRow: row, Age: ago(row.CreatedAt)}
	item.ReasonLabel, item.ReasonName, _ = strings.Cut(row.Reason, ":")
	if ev := strings.TrimSpace(string(row.Evidence)); ev != "" && ev != "{}" {
		item.EvidenceText = prettyJSON(row.Evidence)
		if item.ReasonLabel == "source-drift" {
			var d driftEvidence
			if json.Unmarshal(row.Evidence, &d) == nil && d.OldQuote != "" {
				item.Drift = &d
			}
		}
		if row.Reason == store.ReasonReviewerFlag {
			var f store.ReviewerFlagEvidence
			if json.Unmarshal(row.Evidence, &f) == nil {
				item.Note = f.Note
			}
		} else {
			var w struct {
				Note string `json:"note"`
			}
			if json.Unmarshal(row.Evidence, &w) == nil {
				item.Why = strings.TrimSpace(w.Note)
			}
		}
	}
	if row.Proposed != nil {
		item.NewCitations = citationsNotIn(row.Proposed.Citations, row.CurrentCitations)
		item.GoneCitations = citationsNotIn(row.CurrentCitations, row.Proposed.Citations)
	}
	if row.OnPage() {
		// The editor numbers its cards from zero in page order.
		item.EditorHref = fmt.Sprintf("/edit/%d#card_%d", row.TargetPlaybookID, row.Position-1)
	}
	return item
}

// citationsNotIn returns the citations of a that b lacks, compared on what
// the reader sees: source, locator, and quote.
func citationsNotIn(a, b []store.ProposedCitation) []store.ProposedCitation {
	same := func(x, y store.ProposedCitation) bool {
		return x.Editorial == y.Editorial && x.URL == y.URL && x.Locator == y.Locator && strings.TrimSpace(x.Quote) == strings.TrimSpace(y.Quote)
	}
	var out []store.ProposedCitation
	for _, x := range a {
		found := false
		for _, y := range b {
			if same(x, y) {
				found = true
				break
			}
		}
		if !found {
			out = append(out, x)
		}
	}
	return out
}

// driftEvidence is the source-drift evidence the checker files (see
// sourcecheck.fileDrift). Every field is optional: older proposals carry
// only the quotes and the similarity.
type driftEvidence struct {
	SourceURL   string  `json:"source_url"`
	Publisher   string  `json:"publisher"`
	Locator     string  `json:"locator"`
	OldQuote    string  `json:"old_quote"`
	NewQuote    string  `json:"new_quote"`
	Similarity  float64 `json:"similarity"`
	OldContext  string  `json:"old_context"`
	NewContext  string  `json:"new_context"`
	FetchedVia  string  `json:"fetched_via"`
	TextChanged string  `json:"text_changed"` // "yes" | "unknown"; "" on older proposals
	Note        string  `json:"note"`
}

// Verdict is the one line the reviewer reads first: what the checker
// actually established about the page.
func (d driftEvidence) Verdict() string {
	switch d.TextChanged {
	case "yes":
		return "The page text is different from the text this quote was confirmed in."
	case "no":
		return "The page text is unchanged; the quote could not be matched (report this)."
	default:
		return "No baseline was recorded for this quote, so whether the page changed is not known; only that the quote is not on it now."
	}
}

// SimilarityPct renders the word-bag similarity for the page.
func (d driftEvidence) SimilarityPct() int { return int(d.Similarity*100 + 0.5) }

func (s *srv) queue(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "pending"
	}
	rows, err := s.pg.ListProposalsByReason(ctx, status, r.URL.Query().Get("kind"))
	if err != nil {
		s.serverError(w, err)
		return
	}
	sources, err := s.pg.ListSourceProposals(ctx, status)
	if err != nil {
		s.serverError(w, err)
		return
	}
	var resolved []int64
	for _, row := range rows {
		resolved = append(resolved, store.ResolvedFlagIDs(row.Evidence)...)
	}
	notes, err := s.pg.FlagNotes(ctx, resolved)
	if err != nil {
		s.serverError(w, err)
		return
	}
	// One list, worked top to bottom: proposed edits first, then the
	// checker's drift findings, each family in reading order. Sources no
	// page cites come last on the page.
	items := make([]queueItem, 0, len(rows))
	for _, row := range rows {
		item := newQueueItem(row)
		for _, id := range store.ResolvedFlagIDs(row.Evidence) {
			if n, ok := notes[id]; ok {
				item.Answers = append(item.Answers, n)
			}
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return (items[i].ReasonLabel == "source-drift") != (items[j].ReasonLabel == "source-drift") && items[j].ReasonLabel == "source-drift"
	})
	for i := range items[:max(len(items)-1, 0)] {
		items[i].Next = items[i+1].ID
	}
	sourceItems := make([]sourceItem, 0, len(sources))
	for _, sp := range sources {
		sourceItems = append(sourceItems, sourceItem{SourceProposal: sp, Age: ago(sp.CreatedAt)})
	}
	// The item shown open: the one a decision handed on, else the first.
	open, _ := strconv.ParseInt(r.URL.Query().Get("open"), 10, 64)
	if open == 0 && len(items) > 0 {
		open = items[0].ID
	}
	s.render(w, "queue.html", map[string]any{
		"Actor":    actor(r),
		"Status":   status,
		"Items":    items,
		"Open":     open,
		"Sources":  sourceItems,
		"Count":    len(items) + len(sourceItems),
		"Checking": s.jobs.has("sources-check"),
		"Msg":      r.URL.Query().Get("msg"),
		"Err":      r.URL.Query().Get("err"),
	})
}

// approveProposal turns the proposed statement into a save. The body may
// have been edited on the queue page; the citations are the proposal's,
// resolved to source rows here, with each quote fetched live the way a
// manual save does. Only when the source cannot be read from this server
// does a quote the proposer confirmed in the live page (Checked) go through
// on the proposer's word; a readable page that lacks the quote leaves it
// unverified, and the publish gate says so.
func (s *srv) approveProposal(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	p, err := s.pg.GetProposal(ctx, id)
	if err != nil {
		s.serverError(w, err)
		return
	}
	if p.Proposed == nil {
		// A work item carries no replacement: "approved" means the reviewer
		// dealt with it in the editor.
		if err := s.pg.DecideProposal(ctx, id, "approved", actor(r), strings.TrimSpace(r.FormValue("note")), nil); err != nil {
			s.queueRedirect(w, r, "", err.Error())
			return
		}
		s.queueRedirect(w, r, "Marked resolved.", "")
		return
	}
	err = s.applyApproval(ctx, p, strings.TrimSpace(r.FormValue("body")), actor(r))
	var npe *store.NotPublishableError
	switch {
	case errors.As(err, &npe):
		s.queueRedirect(w, r, "", "Not applied: the page is live and this change would leave it unpublishable. "+npe.Error())
	case errors.Is(err, store.ErrProposalTargetGone):
		s.queueRedirect(w, r, "", "Not applied: "+err.Error()+". Reject it, or edit the page by hand.")
	case err != nil:
		s.queueRedirect(w, r, "", "Not applied: "+err.Error())
	default:
		s.queueRedirect(w, r, fmt.Sprintf("Applied to %s · %s.", p.JurisdictionName, p.TopicName), "")
	}
}

// applyApproval resolves a replacement's sources, checks its quotes, and
// approves it under a person's name. body overrides the proposed text when
// the reviewer edited it first. Shared by the queue and the statement card.
func (s *srv) applyApproval(ctx context.Context, p store.ProposalRow, body, by string) error {
	qv := newQuoteVerifier(s.pg, s.sourceCache)
	check := func(ctx context.Context, stmtNo int, url, quote string) drafting.QuoteVerdict {
		res := qv.check(ctx, stmtNo, url, quote)
		return drafting.QuoteVerdict{Verified: res.Verified, Overridable: res.Overridable, Receipt: res.Receipt}
	}
	return drafting.ApplyProposal(ctx, s.pg, check, p, body, by, "")
}

func (s *srv) rejectProposal(w http.ResponseWriter, r *http.Request) {
	s.decideProposal(w, r, "rejected", nil)
}

func (s *srv) snoozeProposal(w http.ResponseWriter, r *http.Request) {
	days, _ := strconv.Atoi(r.FormValue("days"))
	if days <= 0 {
		days = 7
	}
	until := time.Now().Add(time.Duration(days) * 24 * time.Hour)
	s.decideProposal(w, r, "snoozed", &until)
}

func (s *srv) decideProposal(w http.ResponseWriter, r *http.Request, status string, until *time.Time) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := s.pg.DecideProposal(r.Context(), id, status, actor(r), strings.TrimSpace(r.FormValue("note")), until); err != nil {
		s.queueRedirect(w, r, "", err.Error())
		return
	}
	s.queueRedirect(w, r, strings.ToUpper(status[:1])+status[1:]+".", "")
}

// approveSourceProposal deletes the source. The store refuses if a page has
// come to cite it while the proposal waited.
func (s *srv) approveSourceProposal(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	p, err := s.pg.GetSourceProposal(r.Context(), id)
	if err != nil {
		s.serverError(w, err)
		return
	}
	err = s.pg.ApproveSourceProposal(r.Context(), id, actor(r))
	switch {
	case errors.Is(err, store.ErrSourceInUse):
		s.queueRedirect(w, r, "", "Not deleted: "+err.Error()+" Reject the proposal instead.")
	case err != nil:
		s.queueRedirect(w, r, "", "Not deleted: "+err.Error())
	default:
		s.queueRedirect(w, r, fmt.Sprintf("Deleted source %s.", p.URL), "")
	}
}

func (s *srv) rejectSourceProposal(w http.ResponseWriter, r *http.Request) {
	s.decideSourceProposal(w, r, "rejected", nil)
}

func (s *srv) snoozeSourceProposal(w http.ResponseWriter, r *http.Request) {
	days, _ := strconv.Atoi(r.FormValue("days"))
	if days <= 0 {
		days = 7
	}
	until := time.Now().Add(time.Duration(days) * 24 * time.Hour)
	s.decideSourceProposal(w, r, "snoozed", &until)
}

func (s *srv) decideSourceProposal(w http.ResponseWriter, r *http.Request, status string, until *time.Time) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if err := s.pg.DecideSourceProposal(r.Context(), id, status, actor(r), strings.TrimSpace(r.FormValue("note")), until); err != nil {
		s.queueRedirect(w, r, "", err.Error())
		return
	}
	s.queueRedirect(w, r, strings.ToUpper(status[:1])+status[1:]+".", "")
}

// queueRedirect returns to the queue with the next item open, so a decision
// hands the reviewer the following item instead of the top of the page.
func (s *srv) queueRedirect(w http.ResponseWriter, r *http.Request, msg, errMsg string) {
	q := url.Values{}
	if st := r.FormValue("status"); st != "" {
		q.Set("status", st)
	}
	if next := r.FormValue("next"); next != "" && errMsg == "" {
		q.Set("open", next)
	} else if self := r.FormValue("self"); self != "" && errMsg != "" {
		// The item that failed stays open, with the error above it.
		q.Set("open", self)
	}
	if msg != "" {
		q.Set("msg", msg)
	}
	if errMsg != "" {
		q.Set("err", errMsg)
	}
	http.Redirect(w, r, "/queue?"+q.Encode(), http.StatusSeeOther)
}

func ago(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func prettyJSON(raw []byte) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return string(raw)
	}
	return buf.String()
}
