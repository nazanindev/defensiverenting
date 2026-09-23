package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/nazanindev/defensiverenting/internal/drafting"
	"github.com/nazanindev/defensiverenting/internal/store"
)

// PASS (ADR-022): a reader in a local session reads a statement against its
// cited sources and, when a passage supports the claim as written and
// nothing nearby narrows it, quotes that passage. The command confirms the
// passage verbatim at the live source and stamps the statement reviewed
// under the review agent's name. What the reader cannot pass stays for a
// person, with the reason printed.
//
//	triage decide pass                       unstamped statements on draft pages, with citations
//	triage decide pass <decisions.json> [-apply]
//	                                         stamp the listed statements, each with its passage

type passItem struct {
	PlaybookID int64         `json:"playbook_id"`
	Key        string        `json:"key"`
	Page       string        `json:"page"`
	Position   int           `json:"position"`
	BodyMD     string        `json:"body_md"`
	Concept    string        `json:"concept,omitempty"`
	Citations  []citationOut `json:"citations"`
}

type passDecision struct {
	PlaybookID int64  `json:"playbook_id"`
	Key        string `json:"key"`
	Verdict    string `json:"verdict"` // pass | leave
	SourceURL  string `json:"source_url"`
	Passage    string `json:"passage"`
	Reason     string `json:"reason"`
}

func decidePass(ctx context.Context, pg *store.PG, args []string) {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		decidePassFile(ctx, pg, args[0], args[1:])
		return
	}
	items := unstamped(ctx, pg)
	emit(items)
	fmt.Fprintf(os.Stderr, "%d unstamped statements on draft pages with no open queue item. Read each against its sources (triage fetch <url>); write decisions.json as [{playbook_id, key, verdict: pass|leave, source_url, passage, reason}]; then triage decide pass decisions.json [-apply].\n", len(items))
}

// unstamped lists every statement on a draft page that lacks a valid review
// stamp and has no open queue item, with its citations.
func unstamped(ctx context.Context, pg *store.PG) []passItem {
	pages, err := pg.AuthorListPlaybooks(ctx)
	if err != nil {
		fatal(err)
	}
	var items []passItem
	for _, row := range pages {
		if row.Status != "draft" || row.Language != "en" {
			continue
		}
		pw, err := pg.AuthorGetPlaybook(ctx, row.ID)
		if err != nil {
			fatal(err)
		}
		for i, st := range pw.Statements {
			if st.ReviewedAt != nil || st.Undecided {
				continue
			}
			it := passItem{PlaybookID: pw.ID, Key: st.Key, Page: pw.Jurisdiction.Name + " · " + pw.Topic.Name, Position: i + 1, BodyMD: st.BodyMD, Concept: st.ConceptSlug}
			for _, c := range st.Citations {
				// Site guidance is listed too (url /editorial, no quote): a
				// reader who cannot see it leaves every risk warning as
				// unbacked.
				it.Citations = append(it.Citations, citationOut{URL: c.SourceURL, Publisher: c.Publisher, Kind: c.SourceKind, Locator: c.Locator, Quote: c.Quote})
			}
			items = append(items, it)
		}
	}
	return items
}

func decidePassFile(ctx context.Context, pg *store.PG, path string, args []string) {
	fs := flag.NewFlagSet("decide pass", flag.ExitOnError)
	apply := fs.Bool("apply", false, "write the stamps; default prints them")
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	raw, err := os.ReadFile(path) // #nosec G703 -- the file named on the command line is the job
	if err != nil {
		fatal(err)
	}
	var decisions []passDecision
	if err := json.Unmarshal(raw, &decisions); err != nil {
		fatal(fmt.Errorf("decode %s: %w", path, err))
	}
	byKey := map[string]passItem{}
	for _, it := range unstamped(ctx, pg) {
		byKey[fmt.Sprintf("%d/%s", it.PlaybookID, it.Key)] = it
	}
	check := drafting.LiveQuoteCheck()
	seen, passed, left, noted := 0, 0, 0, 0
	for _, d := range decisions {
		it, ok := byKey[fmt.Sprintf("%d/%s", d.PlaybookID, strings.ToLower(strings.TrimSpace(d.Key)))]
		if !ok {
			fmt.Printf("%d/%s is not an unstamped statement on a draft page without open items; skipped\n", d.PlaybookID, d.Key)
			continue
		}
		seen++
		fmt.Printf("%s · statement %d\n", it.Page, it.Position)
		why := passVerdict(ctx, d, it, check)
		if why != "" {
			left++
			fmt.Printf("    left: %s\n", why)
			if *apply && strings.HasPrefix(why, "left by the reader: ") {
				// The reason becomes a note for the triage agent to propose
				// a fix for (ADR-022); a refusal by the command itself is
				// printed only, since it says nothing about the claim.
				if err := pg.FileReaderNote(ctx, it.PlaybookID, it.Key, strings.TrimPrefix(why, "left by the reader: ")); err != nil {
					fmt.Printf("    could not file the note: %v\n", err)
				} else {
					noted++
				}
			}
			continue
		}
		fmt.Printf("    pass: %s\n", strings.TrimSpace(d.Reason))
		if !*apply {
			passed++
			continue
		}
		err := pg.PassStatement(ctx, it.PlaybookID, it.Key, store.PassEvidence{SourceURL: strings.TrimSpace(d.SourceURL), Passage: strings.TrimSpace(d.Passage), Reason: strings.TrimSpace(d.Reason)})
		switch {
		case errors.Is(err, store.ErrUndecidedItem):
			left++
			fmt.Println("    left for a person: an item was filed on it meanwhile")
		case err != nil:
			left++
			fmt.Printf("    left for a person: %v\n", err)
		default:
			passed++
		}
	}
	if *apply {
		fmt.Printf("%d statements; %d passed by %s; %d left, %d of them noted for the triage agent\n", seen, passed, store.ActorReviewAgent, left, noted)
	} else {
		fmt.Printf("%d statements; %d would pass; %d left; nothing written (add -apply)\n", seen, passed, left)
	}
}

// passVerdict holds a reader's PASS to the rule: the source is one the
// statement cites, the passage is at most flagPassageCap words and is found
// verbatim in that source as fetched live now, and there is a reason.
// Returns "" when the statement may be stamped, else why it stays.
func passVerdict(ctx context.Context, d passDecision, it passItem, check drafting.QuoteCheck) string {
	if strings.ToLower(strings.TrimSpace(d.Verdict)) != "pass" {
		if r := strings.TrimSpace(d.Reason); r != "" {
			return "left by the reader: " + r
		}
		return "left by the reader"
	}
	if strings.TrimSpace(d.Reason) == "" {
		return "pass without a reason; left"
	}
	passage := strings.TrimSpace(d.Passage)
	if passage == "" {
		return "pass without the passage that supports the claim; left"
	}
	if n := len(strings.Fields(passage)); n > flagPassageCap {
		return fmt.Sprintf("the passage runs %d words, over the %d-word cap; left", n, flagPassageCap)
	}
	u := strings.TrimSpace(d.SourceURL)
	cited := false
	for _, c := range it.Citations {
		if c.URL == u {
			cited = true
		}
	}
	if !cited {
		return "the passage's source is not one the statement cites; left"
	}
	v := check(ctx, it.Position, u, passage)
	switch {
	case v.Verified:
		return ""
	case v.Overridable:
		return "the source could not be read from here; a person confirms this one"
	}
	return "the passage is not verbatim at the live source; left"
}
