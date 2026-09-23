package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// work lists what the triage agent proposes fixes for (ADR-022): every
// pending item on a draft page that is not a widen proposal, with the
// statement as it reads, its citations, and the item's evidence. Notes the
// review agent left (a refused PASS, a held edit) appear as items on the
// same key as the statement they concern; the triage agent groups by key
// and files one proposal that answers them all.
//
//	triage decide work                       the triage agent's list, JSON

type workItem struct {
	ID           int64                    `json:"id"`
	Reason       string                   `json:"reason"`
	ProposedBy   string                   `json:"proposed_by"`
	StatementKey string                   `json:"statement_key"`
	PlaybookID   int64                    `json:"playbook_id"`
	Page         string                   `json:"page"`
	Position     int                      `json:"position"`
	BodyMD       string                   `json:"body_md"`
	Concept      string                   `json:"concept,omitempty"`
	Citations    []citationOut            `json:"citations"`
	Proposed     *store.ProposedStatement `json:"proposed,omitempty"`
	Evidence     json.RawMessage          `json:"evidence"`
}

func decideWork(ctx context.Context, pg *store.PG) {
	pending, err := pg.ListProposals(ctx, "pending")
	if err != nil {
		fatal(err)
	}
	pages := map[int64]store.PlaybookWithStatements{}
	var items []workItem
	for _, p := range pending {
		if p.TargetStatus != "draft" || !p.OnPage() || p.Reason == ReasonWidenQuote {
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
			it := workItem{ID: p.ID, Reason: p.Reason, ProposedBy: p.ProposedBy, StatementKey: st.Key, PlaybookID: pw.ID,
				Page: pw.Jurisdiction.Name + " · " + pw.Topic.Name, Position: p.Position, BodyMD: st.BodyMD, Concept: st.ConceptSlug,
				Proposed: p.Proposed, Evidence: p.Evidence}
			for _, c := range st.Citations {
				// Site guidance is listed too (url /editorial, no quote): a
				// reader who cannot see it leaves every risk warning as
				// unbacked.
				it.Citations = append(it.Citations, citationOut{URL: c.SourceURL, Publisher: c.Publisher, Kind: c.SourceKind, Locator: c.Locator, Quote: c.Quote})
			}
			items = append(items, it)
		}
	}
	emit(items)
	fmt.Fprintf(os.Stderr, "%d items for the triage agent on draft pages. Group by statement_key; propose one fix per key (triage check, then cmd/propose -by \"triage agent\"); a judge applies it with triage decide edit.\n", len(items))
}
