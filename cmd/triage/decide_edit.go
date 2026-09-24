package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/nazanindev/defensiverenting/internal/discover"
	"github.com/nazanindev/defensiverenting/internal/drafting"
	"github.com/nazanindev/defensiverenting/internal/store"
	"github.com/nazanindev/defensiverenting/internal/voice"
)

// Rule edit (ADR-021 D3, third rule): a triage edit on a draft page is a
// replacement statement an agent filed with its evidence. A reader in a
// local session reads the statement as it is, the replacement, the note
// that says why, and the sources, and says whether the replacement is what
// the note claims and is backed by its quotes. The command then holds the
// replacement to the same bar a person's Apply would, and more: the body
// passes the voice lint, no citation is reference-only, and every quote is
// found verbatim at the live source, or the item stays pending. An applied
// edit leaves the statement unreviewed for the person's page read.
//
//	triage decide edit                       pending edits on draft pages, current and proposed
//	triage decide edit <decisions.json> [-apply]
//	                                         apply the listed edits

type editItem struct {
	ID               int64                    `json:"id"`
	StatementKey     string                   `json:"statement_key"`
	PlaybookID       int64                    `json:"playbook_id"`
	Page             string                   `json:"page"`
	Position         int                      `json:"position"`
	Reason           string                   `json:"reason"`
	Note             string                   `json:"note"`
	Resolves         []int64                  `json:"resolves,omitempty"`
	CurrentBody      string                   `json:"current_body"`
	CurrentCitations []store.ProposedCitation `json:"current_citations"`
	Proposed         *store.ProposedStatement `json:"proposed"`
}

type editDecision struct {
	ID      int64  `json:"id"`
	Verdict string `json:"verdict"` // apply | leave
	Reason  string `json:"reason"`
}

func decideEdit(ctx context.Context, pg *store.PG, args []string) {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		decideEditFile(ctx, pg, args[0], args[1:])
		return
	}
	items, _ := pendingEdits(ctx, pg)
	emit(items)
	fmt.Fprintf(os.Stderr, "%d edits on draft pages. Read each current statement, the replacement, its note, and its sources (triage fetch <url>); write decisions.json as [{id, verdict: apply|leave, reason}]; then triage decide edit decisions.json [-apply].\n", len(items))
}

// pendingEdits lists every pending proposal with a replacement on a draft
// page whose statement is still there: triage edits and drift suggestions
// alike, since both are a replacement with a note.
func pendingEdits(ctx context.Context, pg *store.PG) ([]editItem, map[int64]store.ProposalRow) {
	pending, err := pg.ListProposals(ctx, "pending")
	if err != nil {
		fatal(err)
	}
	var items []editItem
	rows := map[int64]store.ProposalRow{}
	for _, p := range pending {
		if p.Proposed == nil || p.TargetStatus != "draft" || !p.OnPage() || p.Reason == ReasonWidenQuote {
			continue
		}
		var ev struct {
			Note     string  `json:"note"`
			Resolves []int64 `json:"resolves"`
		}
		_ = json.Unmarshal(p.Evidence, &ev)
		items = append(items, editItem{ID: p.ID, StatementKey: p.StatementKey, PlaybookID: p.TargetPlaybookID,
			Page: p.JurisdictionName + " · " + p.TopicName, Position: p.Position, Reason: p.Reason, Note: ev.Note, Resolves: ev.Resolves,
			CurrentBody: p.CurrentBody, CurrentCitations: p.CurrentCitations, Proposed: p.Proposed})
		rows[p.ID] = p
	}
	return items, rows
}

func decideEditFile(ctx context.Context, pg *store.PG, path string, args []string) {
	fs := flag.NewFlagSet("decide edit", flag.ExitOnError)
	apply := fs.Bool("apply", false, "write the decisions; default prints them")
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	raw, err := os.ReadFile(path) // #nosec G703 -- the file named on the command line is the job
	if err != nil {
		fatal(err)
	}
	var decisions []editDecision
	if err := json.Unmarshal(raw, &decisions); err != nil {
		fatal(fmt.Errorf("decode %s: %w", path, err))
	}
	_, rows := pendingEdits(ctx, pg)
	check := drafting.LiveQuoteCheck()
	seen, applied, left := 0, 0, 0
	for _, d := range decisions {
		p, ok := rows[d.ID]
		if !ok {
			fmt.Printf("#%d is not a pending edit on a draft page; skipped\n", d.ID)
			continue
		}
		seen++
		fmt.Printf("#%d %s · statement %d (%s)\n", p.ID, p.JurisdictionName+" · "+p.TopicName, p.Position, p.Reason)
		why := editVerdict(ctx, d, p, check)
		if why != "" {
			left++
			fmt.Printf("    left pending: %s\n", why)
			if *apply && strings.HasPrefix(why, "left by the reader: ") {
				// The hold becomes a note on the statement for the triage
				// agent, which re-proposes with the reason in hand (ADR-022).
				note := fmt.Sprintf("Proposal #%d held: %s", p.ID, strings.TrimPrefix(why, "left by the reader: "))
				if err := pg.FileReaderNote(ctx, p.TargetPlaybookID, p.StatementKey, note); err != nil {
					fmt.Printf("    could not file the note: %v\n", err)
				}
			}
			continue
		}
		fmt.Printf("    apply: %s\n", strings.TrimSpace(d.Reason))
		if !*apply {
			applied++
			continue
		}
		note := "Review agent, rule edit: " + strings.TrimSpace(d.Reason)
		if err := drafting.ApplyProposal(ctx, pg, check, p, "", store.ActorReviewAgent, note); err != nil {
			left++
			fmt.Printf("    left pending: %v\n", err)
			continue
		}
		applied++
	}
	if *apply {
		fmt.Printf("%d decisions; %d edits applied by %s; %d left for a person\n", seen, applied, store.ActorReviewAgent, left)
	} else {
		fmt.Printf("%d decisions; %d would be applied; %d left for a person; nothing written (add -apply)\n", seen, applied, left)
	}
}

// editVerdict holds a reader's "apply" to the rule. It returns "" when the
// edit may be applied, else why it stays pending.
func editVerdict(ctx context.Context, d editDecision, p store.ProposalRow, check drafting.QuoteCheck) string {
	if strings.ToLower(strings.TrimSpace(d.Verdict)) != "apply" {
		if r := strings.TrimSpace(d.Reason); r != "" {
			return "left by the reader: " + r
		}
		return "left by the reader"
	}
	if strings.TrimSpace(d.Reason) == "" {
		return "apply without a reason; left"
	}
	if p.Proposed == nil {
		return "no replacement to apply; left"
	}
	// A remove or reorder carries no text to check; the store holds the
	// invariants (ADR-025 D4). A merge carries the merged statement.
	if a := p.Proposed.Action; a == store.ActionRemove || a == store.ActionReorder {
		return ""
	}
	if why := statementVerdict(ctx, *p.Proposed, p.Language, p.Position, check); why != "" {
		return why
	}
	if p.Proposed.BodyMD != p.CurrentBody {
		if why := voice.HarderThan(p.Language, p.CurrentBody, p.Proposed.BodyMD); why != "" {
			return why + "; left"
		}
	}
	for i, f := range p.Proposed.Followers {
		if why := statementVerdict(ctx, f, p.Language, p.Position, check); why != "" {
			return fmt.Sprintf("follower %d: %s", i+1, why)
		}
	}
	return ""
}

// statementVerdict holds one proposed statement (the replacement or a
// follower of a split) to the rule: text, voice lint, a quote on every
// citation, every quote confirmed live.
func statementVerdict(ctx context.Context, ps store.ProposedStatement, lang string, position int, check drafting.QuoteCheck) string {
	if strings.TrimSpace(ps.BodyMD) == "" {
		return "the replacement has no text; left"
	}
	if v := voice.LintAll(lang, map[string]string{"body_md": ps.BodyMD}); len(v) > 0 {
		return "the replacement fails the voice lint: " + strings.Join(v, "; ")
	}
	cited := false
	for i, c := range ps.Citations {
		if c.Editorial || c.Kind == "editorial" {
			cited = true
			continue
		}
		u := strings.TrimSpace(c.URL)
		if u == "" {
			return fmt.Sprintf("citation %d has no URL; left", i+1)
		}
		if discover.ReferenceOnly(u) {
			return fmt.Sprintf("citation %d (%s) is reference-only; left", i+1, u)
		}
		if strings.TrimSpace(c.Quote) == "" {
			return fmt.Sprintf("citation %d (%s) has no quote; left", i+1, u)
		}
		v := check(ctx, position, u, c.Quote)
		switch {
		case v.Verified:
		case v.Overridable:
			return fmt.Sprintf("citation %d (%s) could not be read from here; a person confirms this one", i+1, u)
		default:
			return fmt.Sprintf("citation %d: the quote is not at %s; left", i+1, u)
		}
		cited = true
	}
	if !cited {
		return "the replacement cites nothing; left"
	}
	return ""
}
