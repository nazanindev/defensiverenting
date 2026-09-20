package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// merge re-files each triage edit that a widen-quote proposal superseded
// unread, with the widened quote carried into it, so one edit holds both
// the answer and the wider quote. The new filing supersedes the plain
// widening, and the reviewer decides once. Prints what it would file;
// writes only with -apply.
//
//	triage merge [-apply]
func merge(ctx context.Context, pg *store.PG, args []string) {
	apply := len(args) > 0 && args[0] == "-apply"
	superseded, err := pg.ListProposals(ctx, "superseded")
	if err != nil {
		fatal(err)
	}
	pending, err := pg.ListProposalsByReason(ctx, "pending", "other")
	if err != nil {
		fatal(err)
	}
	widen := map[string]store.ProposalRow{}
	for _, p := range pending {
		if p.Reason == "agent-pass:widen-quote" {
			widen[p.StatementKey] = p
		}
	}
	filed := 0
	for _, t := range superseded {
		// A person's decision stamps decided_at; the supersede path does not.
		if t.Reason != "agent-pass:triage" || t.DecidedAt != nil || t.Proposed == nil {
			continue
		}
		w, ok := widen[t.StatementKey]
		if !ok || w.Proposed == nil {
			continue
		}
		var ev map[string]any
		_ = json.Unmarshal(w.Evidence, &ev)
		oldQ, _ := ev["old_quote"].(string)
		newQ, _ := ev["new_quote"].(string)
		src, _ := ev["source_url"].(string)
		merged := *t.Proposed
		merged.Citations = append([]store.ProposedCitation(nil), t.Proposed.Citations...)
		carried := false
		for i := range merged.Citations {
			c := &merged.Citations[i]
			if c.URL != src || strings.TrimSpace(c.Quote) != strings.TrimSpace(oldQ) {
				continue
			}
			for _, wc := range w.Proposed.Citations {
				if wc.URL == src && wc.Quote == newQ {
					c.Quote, c.Checked, c.CheckedVia = wc.Quote, wc.Checked, wc.CheckedVia
					carried = true
				}
			}
		}
		fmt.Printf("triage #%d + widen #%d on %s: %s\n", t.ID, w.ID, t.StatementKey[:8], truncate(t.Title, 60))
		if !carried {
			fmt.Println("  the triage edit no longer carries the narrow quote; nothing to merge, restore it by hand")
			continue
		}
		var tev map[string]any
		_ = json.Unmarshal(t.Evidence, &tev)
		if tev == nil {
			tev = map[string]any{}
		}
		note, _ := tev["note"].(string)
		wnote, _ := ev["note"].(string)
		tev["note"] = strings.TrimSpace(note + " " + wnote)
		tev["old_quote"], tev["new_quote"], tev["source_url"] = oldQ, newQ, src
		evJSON, err := json.Marshal(tev)
		if err != nil {
			fatal(err)
		}
		if !apply {
			continue
		}
		id, err := pg.FileProposal(ctx, store.FileProposalParams{
			StatementKey: t.StatementKey, PlaybookID: t.PlaybookID, Reason: "agent-pass:triage",
			Proposed: &merged, Evidence: evJSON, ProposedBy: t.ProposedBy,
		})
		if err != nil {
			fatal(fmt.Errorf("file merged edit for %s: %w", t.StatementKey, err))
		}
		fmt.Printf("  filed #%d, superseding widen #%d\n", id, w.ID)
		filed++
	}
	if apply {
		fmt.Printf("%d merged edits filed\n", filed)
	} else {
		fmt.Println("nothing written (add -apply)")
	}
}
