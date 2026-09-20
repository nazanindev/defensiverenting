package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/nazanindev/defensiverenting/internal/drafting"
	"github.com/nazanindev/defensiverenting/internal/store"
)

// decide is the review agent's seat in the queue (ADR-021): it decides the
// proposals a stated rule can decide, under its own name, through the same
// approval path a person's click takes. Anything the rule cannot decide it
// leaves pending and says why. The first rule is widen: a widen-quote
// proposal whose only change is each quote growing to contain the quote a
// person already read, confirmed verbatim at the live source.
//
//	triage decide widen [-apply] [-limit n]   decide pending widen-quote proposals
//	triage decide audit                       every proposal the agent decided
func decide(ctx context.Context, pg *store.PG, args []string) {
	if len(args) < 1 {
		usage()
	}
	switch args[0] {
	case "widen":
		decideWiden(ctx, pg, args[1:])
	case "audit":
		audit(ctx, pg)
	default:
		usage()
	}
}

// ReasonWidenQuote is the reason triage widen files under.
const ReasonWidenQuote = "agent-pass:widen-quote"

func decideWiden(ctx context.Context, pg *store.PG, args []string) {
	fs := flag.NewFlagSet("decide widen", flag.ExitOnError)
	apply := fs.Bool("apply", false, "write the decisions; default prints them")
	limit := fs.Int("limit", 0, "stop after this many approvals (0 = all)")
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	pending, err := pg.ListProposalsByReason(ctx, "pending", "edit")
	if err != nil {
		fatal(err)
	}
	check := drafting.LiveQuoteCheck()
	pages := map[int64]store.PlaybookWithStatements{}
	current := func(p store.ProposalRow) (store.CitedStatement, bool) {
		pw, ok := pages[p.TargetPlaybookID]
		if !ok {
			var err error
			if pw, err = pg.AuthorGetPlaybook(ctx, p.TargetPlaybookID); err != nil {
				fatal(err)
			}
			pages[p.TargetPlaybookID] = pw
		}
		for _, st := range pw.Statements {
			if st.Key == p.StatementKey {
				return st, true
			}
		}
		return store.CitedStatement{}, false
	}
	seen, approved, left := 0, 0, 0
	for _, p := range pending {
		if p.Reason != ReasonWidenQuote {
			continue
		}
		seen++
		cur, ok := current(p)
		if !ok {
			left++
			fmt.Printf("#%d %s · %s\n    left pending: the statement is no longer on the page\n", p.ID, p.JurisdictionName, p.TopicName)
			continue
		}
		changed, why := widenVerdict(p, cur)
		if why == "" {
			for _, c := range changed {
				v := check(ctx, p.Position, c.URL, c.Quote)
				switch {
				case v.Verified:
				case v.Overridable:
					why = "the source could not be read from here; a person confirms this one"
				default:
					why = "the widened quote is not at the live source"
				}
				if why != "" {
					break
				}
			}
		}
		if why != "" {
			left++
			fmt.Printf("#%d %s · %s\n    left pending: %s\n", p.ID, p.JurisdictionName, p.TopicName, why)
			continue
		}
		fmt.Printf("#%d %s · %s (%s)\n    approve: %d quote(s) widened, each containing the reviewed quote, confirmed at the source\n",
			p.ID, p.JurisdictionName, p.TopicName, p.TargetStatus, len(changed))
		if !*apply {
			approved++
			continue
		}
		note := fmt.Sprintf("Review agent, rule widen: %d quote(s) widened to the subsection; each contains the quote a person reviewed; confirmed verbatim at the live source.", len(changed))
		pages = map[int64]store.PlaybookWithStatements{} // the page changes under us
		err := drafting.ApplyProposal(ctx, pg, check, p, "", store.ActorReviewAgent, note)
		var npe *store.NotPublishableError
		switch {
		case errors.As(err, &npe):
			left++
			fmt.Printf("    left pending: the page is live and the gate refused it: %v\n", npe)
		case err != nil:
			left++
			fmt.Printf("    left pending: %v\n", err)
		default:
			approved++
		}
		if *limit > 0 && approved >= *limit {
			break
		}
	}
	if *apply {
		fmt.Printf("%d widen-quote proposals seen; %d approved by %s; %d left for a person\n", seen, approved, store.ActorReviewAgent, left)
	} else {
		fmt.Printf("%d widen-quote proposals seen; %d would be approved; %d left for a person; nothing written (add -apply)\n", seen, approved, left)
	}
}

// widenVerdict applies the widen rule to one proposal against the statement
// as it reads on the target page today. It returns the citations whose
// quote changes, for the live check, or a reason the rule does not decide
// this proposal. The rule: body and tags identical; the same sources at the
// same locators; every changed quote contains the current quote and stays
// under the widen cap; at least one quote changes.
func widenVerdict(p store.ProposalRow, cur store.CitedStatement) ([]store.ProposedCitation, string) {
	if p.Proposed == nil {
		return nil, "no replacement to apply"
	}
	if p.Proposed.BodyMD != cur.BodyMD {
		return nil, "the body changes; only a quote may"
	}
	if p.Proposed.Concept != cur.ConceptSlug || p.Proposed.TopicRef != cur.TopicRefSlug {
		return nil, "the tags change; only a quote may"
	}
	type at struct{ URL, Locator string }
	have := map[at]store.CitationWithSource{}
	editorial := 0
	for _, c := range cur.Citations {
		if c.SourceKind == "editorial" {
			editorial++
			continue
		}
		have[at{c.SourceURL, c.Locator}] = c
	}
	var changed []store.ProposedCitation
	used := map[at]bool{}
	for _, c := range p.Proposed.Citations {
		if c.Editorial || c.Kind == "editorial" {
			editorial--
			continue
		}
		k := at{strings.TrimSpace(c.URL), c.Locator}
		old, ok := have[k]
		if !ok || used[k] {
			return nil, fmt.Sprintf("citation %s %s is not on the statement as it reads; a source or locator changes", k.URL, k.Locator)
		}
		used[k] = true
		if collapse(c.Quote) == collapse(old.Quote) {
			continue
		}
		if !drafting.QuoteAppearsIn(c.Quote, old.Quote) {
			return nil, "a new quote does not contain the quote a person reviewed"
		}
		if w := len(strings.Fields(c.Quote)); w > widenCap {
			return nil, fmt.Sprintf("a quote runs %d words, over the %d-word cap", w, widenCap)
		}
		changed = append(changed, c)
	}
	if editorial != 0 || len(used) != len(have) {
		return nil, "a citation is added or dropped; only a quote may change"
	}
	if len(changed) == 0 {
		return nil, "nothing changes"
	}
	return changed, ""
}

func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }

// audit lists every proposal the review agent decided, newest first, so a
// person can read what it did in one place.
func audit(ctx context.Context, pg *store.PG) {
	n := 0
	for _, status := range []string{"approved", "rejected"} {
		rows, err := pg.ListProposals(ctx, status)
		if err != nil {
			fatal(err)
		}
		for _, r := range rows {
			if r.DecidedBy != store.ActorReviewAgent {
				continue
			}
			n++
			when := ""
			if r.DecidedAt != nil {
				when = r.DecidedAt.Format("2006-01-02 15:04")
			}
			fmt.Printf("#%d %s %s · %s · %s (%s)\n    %s\n    %s\n", r.ID, status, when, r.JurisdictionName, r.TopicName, r.TargetStatus, truncate(r.CurrentBody, 120), r.DecisionNote)
		}
	}
	fmt.Printf("%d decisions by %s\n", n, store.ActorReviewAgent)
}
