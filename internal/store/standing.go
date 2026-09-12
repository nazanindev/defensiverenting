package store

import "strings"

// Standing is the one answer to "is this statement ready to publish?",
// computed from the statement alone so every surface that shows it (a
// card on any grouping, the dashboard worklist) reads the same thing. The
// gate in issues.go enforces the same conditions per page in SQL; a test
// holds the two together.

// StandingStatus is the three-way summary a reviewer scans for.
type StandingStatus string

const (
	// Ready: nothing left to do on this statement.
	Ready StandingStatus = "ready"
	// NeedsYou: a person has to read, decide, or confirm something.
	NeedsYou StandingStatus = "needs-you"
	// Blocked: a quote needs confirming but the checker cannot read its
	// source; only a person who opens it can attest.
	Blocked StandingStatus = "blocked"
)

// Reason codes, stable for templates and tests. Each names the verb that
// clears it: decide, confirm, review, or fix in the editor.
const (
	ReasonEmpty            = "empty"             // no words
	ReasonUncited          = "uncited"           // no citation at all
	ReasonQuoteMissing     = "quote-missing"     // a citation with no verbatim quote
	ReasonQuoteUnconfirmed = "quote-unconfirmed" // a quote nobody confirmed
	ReasonSourceBlocked    = "source-blocked"    // an unconfirmed quote on a source the checker cannot read
	ReasonStatuteLocator   = "statute-locator"   // a statute cited without its provision
	ReasonNote             = "note"              // a reviewer note to decide
	ReasonProposal         = "proposal"          // a replacement or drift finding in the queue
	ReasonUnread           = "unread"            // no valid review stamp
)

// Standing is the computed status with the reasons behind it, in the order
// a reviewer should clear them: fix the words and evidence, confirm quotes,
// decide what is open, then read and stamp.
type Standing struct {
	Status  StandingStatus
	Reasons []string
	// Unconfirmed counts quotes on this statement nobody has confirmed.
	Unconfirmed int
}

func (s Standing) Has(reason string) bool {
	for _, r := range s.Reasons {
		if r == reason {
			return true
		}
	}
	return false
}

// Standing computes the statement's standing. pageLevel is true on a
// directory page, whose statements are reviewed as a page at publish and so
// never carry the unread reason themselves.
func (st CitedStatement) Standing(pageLevel bool) Standing {
	var out Standing
	add := func(r string) {
		if !out.Has(r) {
			out.Reasons = append(out.Reasons, r)
		}
	}
	if strings.TrimSpace(st.BodyMD) == "" {
		add(ReasonEmpty)
	}
	if len(st.Citations) == 0 {
		add(ReasonUncited)
	}
	for _, c := range st.Citations {
		if c.SourceKind == "editorial" {
			continue
		}
		switch {
		case strings.TrimSpace(c.Quote) == "":
			add(ReasonQuoteMissing)
		case c.CheckedAt == nil && !c.ManuallyVerified:
			out.Unconfirmed++
			if c.SourceUnreadable {
				add(ReasonSourceBlocked)
			} else {
				add(ReasonQuoteUnconfirmed)
			}
		}
		if c.SourceKind == "statute" && !looksLikeSection(c.Locator) {
			add(ReasonStatuteLocator)
		}
	}
	if len(st.Notes) > 0 || (st.Undecided && !st.ProposalPending) {
		add(ReasonNote)
	}
	if st.ProposalPending {
		add(ReasonProposal)
	}
	if !pageLevel && st.ReviewedAt == nil {
		add(ReasonUnread)
	}
	switch {
	case len(out.Reasons) == 0:
		out.Status = Ready
	case out.Has(ReasonSourceBlocked):
		out.Status = Blocked
	default:
		out.Status = NeedsYou
	}
	return out
}

// PageStanding aggregates statements' standings for a worklist row.
type PageStanding struct {
	Total, Ready, Blocked int
	// Counts of statements carrying each reason; a statement may count in
	// several.
	Unread, Notes, Proposals, Unconfirmed, Broken int
}

// Aggregate folds one statement's standing into the page's.
func (p *PageStanding) Aggregate(s Standing) {
	p.Total++
	switch s.Status {
	case Ready:
		p.Ready++
	case Blocked:
		p.Blocked++
	}
	if s.Has(ReasonUnread) {
		p.Unread++
	}
	if s.Has(ReasonNote) {
		p.Notes++
	}
	if s.Has(ReasonProposal) {
		p.Proposals++
	}
	if s.Has(ReasonQuoteUnconfirmed) || s.Has(ReasonSourceBlocked) {
		p.Unconfirmed++
	}
	if s.Has(ReasonEmpty) || s.Has(ReasonUncited) || s.Has(ReasonQuoteMissing) || s.Has(ReasonStatuteLocator) {
		p.Broken++
	}
}

// Publishable reports whether every statement is ready: the worklist's
// version of the gate's verdict.
func (p PageStanding) Publishable() bool { return p.Total > 0 && p.Ready == p.Total }
