package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/nazanindev/defensiverenting/internal/discover"
	"github.com/nazanindev/defensiverenting/internal/store"
)

// The review queue (ADR-014 D5): every pending statement proposal, grouped
// by page in reading order. Approving one is the reviewer's save of that
// page with the statement swapped (D3), so it is subject to everything a
// save is — the reference-only rule, live quote checks, the publish gate.

type queueGroup struct {
	Title            string
	JurisdictionName string
	TopicName        string
	Language         string
	TargetPlaybookID int64
	TargetStatus     string
	Items            []queueItem
}

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
	rows, err := s.pg.ListProposals(ctx, status)
	if err != nil {
		s.serverError(w, err)
		return
	}
	sources, err := s.pg.ListSourceProposals(ctx, status)
	if err != nil {
		s.serverError(w, err)
		return
	}
	sourceItems := make([]sourceItem, 0, len(sources))
	for _, sp := range sources {
		sourceItems = append(sourceItems, sourceItem{SourceProposal: sp, Age: ago(sp.CreatedAt)})
	}
	var groups []queueGroup
	for _, row := range rows {
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
		}
		if n := len(groups); n == 0 || groups[n-1].TargetPlaybookID != row.TargetPlaybookID {
			groups = append(groups, queueGroup{
				Title: row.Title, JurisdictionName: row.JurisdictionName, TopicName: row.TopicName,
				Language: row.Language, TargetPlaybookID: row.TargetPlaybookID, TargetStatus: row.TargetStatus,
			})
		}
		groups[len(groups)-1].Items = append(groups[len(groups)-1].Items, item)
	}
	s.render(w, "queue.html", map[string]any{
		"Actor":    actor(r),
		"Status":   status,
		"Groups":   groups,
		"Sources":  sourceItems,
		"Count":    len(rows) + len(sourceItems),
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
	body := strings.TrimSpace(r.FormValue("body"))
	if body == "" {
		body = p.Proposed.BodyMD
	}
	stmt := store.IngestStatementParams{BodyMD: body, ConceptSlug: p.Proposed.Concept, TopicRefSlug: p.Proposed.TopicRef}
	editorial, err := s.pg.GetEditorialSource(ctx)
	if err != nil {
		s.serverError(w, err)
		return
	}
	pw, err := s.pg.AuthorGetPlaybook(ctx, p.TargetPlaybookID)
	if err != nil {
		s.serverError(w, err)
		return
	}
	qv := newQuoteVerifier(s.pg, s.sourceCache)
	for i, c := range p.Proposed.Citations {
		if c.Editorial || c.Kind == "editorial" {
			stmt.Sources = append(stmt.Sources, store.IngestCitationParams{SourceID: editorial.ID})
			continue
		}
		u := strings.TrimSpace(c.URL)
		if u == "" {
			s.queueRedirect(w, r, "", fmt.Sprintf("citation %d has no URL; sources are stored by URL", i+1))
			return
		}
		if discover.ReferenceOnly(u) {
			s.queueRedirect(w, r, "", fmt.Sprintf("citation %d (%s) is reference-only and can never become a source; reject this proposal or edit the page by hand", i+1, u))
			return
		}
		src, err := s.pg.UpsertSource(ctx, store.UpsertSourceParams{
			URL: u, Publisher: strings.TrimSpace(c.Publisher), Kind: sourceKindOrDefault(c.Kind), JurisdictionID: &pw.JurisdictionID,
		})
		if err != nil {
			s.queueRedirect(w, r, "", fmt.Sprintf("citation %d: %v", i+1, err))
			return
		}
		cite := store.IngestCitationParams{SourceID: src.ID, Locator: c.Locator, Quote: c.Quote}
		if strings.TrimSpace(c.Quote) != "" {
			res := qv.check(ctx, p.Position, u, c.Quote)
			switch {
			case res.Verified:
				cite.CheckedNow, cite.CheckedBy, cite.Checked = true, actor(r), res.Receipt
			case c.Checked && res.Overridable:
				// This server could not read the page; the proposer did.
				cite.CheckedNow, cite.CheckedBy = true, p.ProposedBy
				cite.Checked = store.CheckReceipt{Via: c.CheckedVia}
			}
		}
		stmt.Sources = append(stmt.Sources, cite)
	}

	err = s.pg.ApproveProposal(ctx, store.ApproveProposalParams{ID: id, By: actor(r), Statement: stmt})
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

func (s *srv) queueRedirect(w http.ResponseWriter, r *http.Request, msg, errMsg string) {
	q := url.Values{}
	if st := r.FormValue("status"); st != "" {
		q.Set("status", st)
	}
	if msg != "" {
		q.Set("msg", msg)
	}
	if errMsg != "" {
		q.Set("err", errMsg)
	}
	http.Redirect(w, r, "/queue?"+q.Encode(), http.StatusSeeOther)
}

func sourceKindOrDefault(k string) string {
	switch k {
	case "statute", "regulation", "gov_guidance", "nonprofit", "court_ruling":
		return k
	}
	return "gov_guidance"
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
