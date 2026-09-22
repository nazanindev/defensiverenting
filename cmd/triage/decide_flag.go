package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/nazanindev/defensiverenting/internal/drafting"
	"github.com/nazanindev/defensiverenting/internal/store"
)

// Rule flag (ADR-021 D3, second rule): a reviewer flag on a draft page is a
// doubt the drafting agent had about a claim. A second reader, the agent
// working this command in a local session, reads the claim, the doubt, and
// the cited sources as fetched now, and says whether the cited text itself
// answers the doubt. It may only answer "stands" by quoting the passage that
// answers it, and the command confirms that passage verbatim at the source
// before anything is written; a passage that is not there is thrown away.
// What the cited sections do not settle stays pending for a person.
//
//	triage decide flag [-published]          the pending flags, with their citations
//	triage decide flag <decisions.json> [-apply] [-published]
//	                                         close the listed flags as stands, each with its passage
//
// -published extends the rule to flags on live pages: closing a flag as
// stands writes nothing to the page, so nothing a reader sees can change.
//
// The reader fetches sources with triage fetch, like the triage pass. It has
// nothing the statement does not already cite.

// flagPassageCap bounds the passage a "stands" answer quotes: enough for a
// subsection, not enough to paste the document back.
const flagPassageCap = 150

// flagItem is one pending flag as the reader sees it.
type flagItem struct {
	ID           int64         `json:"id"`
	StatementKey string        `json:"statement_key"`
	PlaybookID   int64         `json:"playbook_id"`
	Page         string        `json:"page"`
	Position     int           `json:"position"`
	BodyMD       string        `json:"body_md"`
	Doubt        string        `json:"doubt"`
	Citations    []citationOut `json:"citations"`
}

// flagDecision is one entry of the decisions file.
type flagDecision struct {
	ID      int64  `json:"id"`
	Verdict string `json:"verdict"` // stands | leave
	Passage string `json:"passage"` // verbatim from a cited source, for stands
	Reason  string `json:"reason"`  // one or two plain sentences for the auditor
}

// flagSource is one cited source's fetched text, or why there is none.
type flagSource struct {
	URL  string
	Text string
	Err  string
}

func decideFlag(ctx context.Context, pg *store.PG, args []string) {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		decideFlagFile(ctx, pg, args[0], args[1:])
		return
	}
	fs := flag.NewFlagSet("decide flag", flag.ExitOnError)
	published := fs.Bool("published", false, "include flags on published pages (closing a flag changes nothing a reader sees)")
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	items := pendingFlags(ctx, pg, *published)
	emit(items)
	fmt.Fprintf(os.Stderr, "%d flags on draft pages. Read each statement, its doubt, and its sources (triage fetch <url>); write decisions.json as [{id, verdict, passage, reason}]; then triage decide flag decisions.json [-apply].\n", len(items))
}

// pendingFlags lists every reviewer flag on a draft page whose statement is
// still on it, with the statement and citations as they read now.
func pendingFlags(ctx context.Context, pg *store.PG, published bool) []flagItem {
	pending, err := pg.ListProposalsByReason(ctx, "pending", "note")
	if err != nil {
		fatal(err)
	}
	pages := map[int64]store.PlaybookWithStatements{}
	var items []flagItem
	for _, p := range pending {
		if (p.TargetStatus != "draft" && !published) || !p.OnPage() {
			continue
		}
		if p.ProposedBy == store.ActorReviewAgent {
			// A note the review agent itself left (a refused PASS or a
			// held edit) is the triage agent's to fix, not a doubt for
			// rule flag to answer; answering it would only bounce.
			continue
		}
		var ev store.ReviewerFlagEvidence
		if json.Unmarshal(p.Evidence, &ev) != nil || strings.TrimSpace(ev.Note) == "" {
			continue
		}
		pw, ok := pages[p.TargetPlaybookID]
		if !ok {
			if pw, err = pg.AuthorGetPlaybook(ctx, p.TargetPlaybookID); err != nil {
				fatal(err)
			}
			pages[p.TargetPlaybookID] = pw
		}
		for _, st := range pw.Statements {
			if st.Key != p.StatementKey {
				continue
			}
			it := flagItem{ID: p.ID, StatementKey: st.Key, PlaybookID: pw.ID, Page: pw.Jurisdiction.Name + " · " + pw.Topic.Name,
				Position: p.Position, BodyMD: st.BodyMD, Doubt: ev.Note}
			for _, c := range st.Citations {
				if c.SourceKind == "editorial" {
					continue
				}
				it.Citations = append(it.Citations, citationOut{URL: c.SourceURL, Publisher: c.Publisher, Kind: c.SourceKind, Locator: c.Locator, Quote: c.Quote})
			}
			items = append(items, it)
		}
	}
	return items
}

// decideFlagFile closes the flags a decisions file marks stands, after
// confirming each passage at the source.
func decideFlagFile(ctx context.Context, pg *store.PG, path string, args []string) {
	fs := flag.NewFlagSet("decide flag", flag.ExitOnError)
	apply := fs.Bool("apply", false, "write the decisions; default prints them")
	published := fs.Bool("published", false, "allow closing flags on published pages")
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	raw, err := os.ReadFile(path) // #nosec G703 -- the file named on the command line is the job
	if err != nil {
		fatal(err)
	}
	var decisions []flagDecision
	if err := json.Unmarshal(raw, &decisions); err != nil {
		fatal(fmt.Errorf("decode %s: %w", path, err))
	}
	byID := map[int64]flagItem{}
	for _, it := range pendingFlags(ctx, pg, *published) {
		byID[it.ID] = it
	}
	fetched := map[string]flagSource{}
	fetch := func(url string) flagSource {
		if s, ok := fetched[url]; ok {
			return s
		}
		s := flagSource{URL: url}
		rc, err := drafting.FetchExtract(url)
		switch {
		case err != nil:
			s.Err = err.Error()
		case strings.TrimSpace(rc.Text) == "":
			s.Err = "no text (" + rc.Describe() + ")"
		default:
			// Thin text is kept: the verbatim check below is the real test,
			// and a short statute section renders short. A passage that is
			// not in it fails on its own.
			s.Text = rc.Text
		}
		fetched[url] = s
		return s
	}
	seen, stood, left := 0, 0, 0
	for _, d := range decisions {
		it, ok := byID[d.ID]
		if !ok {
			fmt.Printf("#%d is not a pending flag on a draft page; skipped\n", d.ID)
			continue
		}
		seen++
		fmt.Printf("#%d %s · statement %d\n    doubt: %s\n", it.ID, it.Page, it.Position, truncate(it.Doubt, 160))
		var srcs []flagSource
		if strings.EqualFold(strings.TrimSpace(d.Verdict), "stands") {
			for _, c := range it.Citations {
				srcs = append(srcs, fetch(c.URL))
			}
		}
		stands, why := flagVerdict(d, srcs)
		if !stands {
			left++
			fmt.Printf("    left pending: %s\n", why)
			continue
		}
		fmt.Printf("    stands: %s\n    passage: %s\n", d.Reason, truncate(strings.TrimSpace(d.Passage), 200))
		if !*apply {
			stood++
			continue
		}
		note := fmt.Sprintf("Review agent, rule flag: stands as written. %s Passage: “%s”", strings.TrimSpace(d.Reason), strings.TrimSpace(d.Passage))
		if err := pg.DecideProposal(ctx, it.ID, "rejected", store.ActorReviewAgent, note, nil); err != nil {
			left++
			fmt.Printf("    left pending: %v\n", err)
			continue
		}
		stood++
	}
	if *apply {
		fmt.Printf("%d decisions; %d flags closed as stands by %s; %d left for a person\n", seen, stood, store.ActorReviewAgent, left)
	} else {
		fmt.Printf("%d decisions; %d would close as stands; %d left for a person; nothing written (add -apply)\n", seen, stood, left)
	}
}

// flagVerdict applies the invariants to a decision. A "stands" survives only
// when its passage is verbatim in one of the fetched texts and short enough
// to be a passage. Returns whether the flag closes, or why it stays pending.
func flagVerdict(d flagDecision, srcs []flagSource) (stands bool, why string) {
	d.Verdict = strings.ToLower(strings.TrimSpace(d.Verdict))
	d.Reason = strings.TrimSpace(d.Reason)
	if d.Verdict != "stands" {
		if d.Reason == "" {
			return false, "left by the reader"
		}
		return false, "left by the reader: " + d.Reason
	}
	if d.Reason == "" {
		return false, "stands without a reason; left"
	}
	passage := strings.TrimSpace(d.Passage)
	if passage == "" {
		return false, "stands without the passage that settles it; left"
	}
	if n := len(strings.Fields(passage)); n > flagPassageCap {
		return false, fmt.Sprintf("the passage runs %d words, over the %d-word cap; left", n, flagPassageCap)
	}
	for _, s := range srcs {
		if s.Text != "" && drafting.QuoteAppearsIn(s.Text, passage) {
			return true, ""
		}
	}
	for _, s := range srcs {
		if s.Err != "" {
			return false, fmt.Sprintf("the passage is not verbatim in any readable cited source (%s: %s); left", s.URL, s.Err)
		}
	}
	return false, "the passage is not verbatim in any cited source; left"
}
