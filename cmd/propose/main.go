// propose files statement proposals (ADR-014) from a JSON file, for an agent
// pass or an operator to hand the review queue a batch of per-statement
// changes without touching any page.
//
//	propose -file proposals.json -by "agent-pass rerun-sources"
//
// The file is an array of proposals:
//
//	[{"statement_key": "…", "reason": "agent-pass:rerun-sources",
//	  "proposed": {"body_md": "…", "concept": "", "citations": [
//	     {"url": "https://…", "publisher": "…", "kind": "statute",
//	      "locator": "§ 1", "quote": "…", "checked": true}]},
//	  "evidence": {"old_quote": "…", "new_quote": "…"}}]
//
// proposed may be null for a work item with no replacement. playbook_id is
// optional; the page is found from the key when it is omitted. Nothing here
// writes to a page: approval on the queue does that, under a person's name.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/nazanindev/defensiverenting/internal/store"
)

type fileEntry struct {
	StatementKey string                   `json:"statement_key"`
	PlaybookID   int64                    `json:"playbook_id,omitempty"`
	Reason       string                   `json:"reason"`
	Proposed     *store.ProposedStatement `json:"proposed"`
	Evidence     json.RawMessage          `json:"evidence"`
}

func main() {
	dsn := flag.String("db", os.Getenv("DATABASE_URL"), "Postgres DSN")
	file := flag.String("file", "", "JSON file of proposals (- for stdin)")
	by := flag.String("by", "", "who is proposing: an agent pass name or a person's first name")
	dry := flag.Bool("dry-run", false, "validate and print what would be filed, write nothing")
	flag.Parse()
	if *file == "" || *by == "" {
		fmt.Fprintln(os.Stderr, "usage: propose -file proposals.json -by <name> [-dry-run]")
		os.Exit(2)
	}
	in := os.Stdin
	if *file != "-" {
		f, err := os.Open(*file)
		if err != nil {
			fatal(err)
		}
		defer f.Close()
		in = f
	}
	var entries []fileEntry
	if err := json.NewDecoder(in).Decode(&entries); err != nil {
		fatal(fmt.Errorf("decode %s: %w", *file, err))
	}
	for i, e := range entries {
		if !store.ValidReason(e.Reason) {
			fatal(fmt.Errorf("entry %d: reason %q is not source-drift, agent-pass:<name>, or source-quality:<signal>", i+1, e.Reason))
		}
		if e.Proposed != nil && e.Proposed.BodyMD == "" {
			fatal(fmt.Errorf("entry %d: proposed has no body_md; use null for a work item", i+1))
		}
	}
	if *dry {
		for i, e := range entries {
			fmt.Printf("%d. %s key %s: %s\n", i+1, e.Reason, e.StatementKey, describe(e.Proposed))
		}
		fmt.Printf("%d proposal(s) valid; nothing written (dry run)\n", len(entries))
		return
	}
	ctx := context.Background()
	pg, err := store.New(ctx, *dsn)
	if err != nil {
		fatal(err)
	}
	defer pg.Close()
	for i, e := range entries {
		id, err := pg.FileProposal(ctx, store.FileProposalParams{
			StatementKey: e.StatementKey, PlaybookID: e.PlaybookID, Reason: e.Reason,
			Proposed: e.Proposed, Evidence: e.Evidence, ProposedBy: *by,
		})
		if err != nil {
			fatal(fmt.Errorf("entry %d (key %s): %w", i+1, e.StatementKey, err))
		}
		fmt.Printf("filed proposal %d: %s key %s: %s\n", id, e.Reason, e.StatementKey, describe(e.Proposed))
	}
}

func describe(p *store.ProposedStatement) string {
	if p == nil {
		return "work item, no replacement"
	}
	r := []rune(p.BodyMD)
	if len(r) > 70 {
		return string(r[:70]) + "…"
	}
	return p.BodyMD
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "propose:", err)
	os.Exit(1)
}
