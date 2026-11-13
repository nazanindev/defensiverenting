package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// topicAudit prints a topic's grid (ADR-035 D2): a row per place, a column per
// group of checks, each cell the number of failures or a dot. It writes
// nothing, and exits 1 when any place fails a check or has no page.
//
//	triage audit <topic>                  the grid, and a count per check
//	triage audit <topic> -v               every finding under its place
//	triage audit <topic> -place <slug>    one place's findings
//	triage audit <topic> -code <code>     only this check, across places
//	triage audit <topic> -drafts          draft pages only
//	triage audit <topic> -json            the rows, for an agent's input file
func topicAudit(ctx context.Context, pg *store.PG, args []string) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		fatal(fmt.Errorf("audit: name a topic slug"))
	}
	fs := flag.NewFlagSet("audit", flag.ExitOnError)
	verbose := fs.Bool("v", false, "list every finding")
	place := fs.String("place", "", "one place slug")
	code := fs.String("code", "", "one check code")
	drafts := fs.Bool("drafts", false, "draft pages only")
	asJSON := fs.Bool("json", false, "print the rows as JSON")
	if err := fs.Parse(args[1:]); err != nil {
		fatal(err)
	}
	a, err := pg.AuditTopic(ctx, args[0])
	if err != nil {
		fatal(err)
	}
	rows := a.Rows[:0:0]
	for _, r := range a.Rows {
		if *place != "" && r.PlaceSlug != *place {
			continue
		}
		if *drafts && r.Status != "draft" {
			continue
		}
		if *code != "" {
			kept := r.Findings[:0:0]
			for _, f := range r.Findings {
				if f.Code == *code {
					kept = append(kept, f)
				}
			}
			if len(kept) == 0 {
				continue
			}
			r.Findings = kept
		}
		rows = append(rows, r)
	}
	if *asJSON {
		emit(rows)
		return
	}

	detail := *verbose || *place != "" || *code != ""
	fmt.Printf("%-22s %-6s %-5s %4s %5s", "place", "page", "", "stmt", "words")
	for _, g := range store.AuditGroups {
		fmt.Printf(" %5.5s", g)
	}
	fmt.Printf(" %5s\n", "hints")
	failing, missing := 0, 0
	byCode := map[string]int{}
	pagesWith := map[string]int{}
	for _, r := range rows {
		if r.PageID == 0 {
			missing++
			fmt.Printf("%-22.22s %-6s %-5s\n", r.Place, "-", "none")
			continue
		}
		fmt.Printf("%-22.22s %-6d %-5.5s %4d %5d", r.Place, r.PageID, r.Status, r.Statements, r.Words)
		bad := false
		for _, g := range store.AuditGroups {
			if n := r.Fails(g); n > 0 {
				bad = true
				fmt.Printf(" %5d", n)
			} else {
				fmt.Printf(" %5s", ".")
			}
		}
		if h := r.Hints(); h > 0 {
			fmt.Printf(" %5d", h)
		} else {
			fmt.Printf(" %5s", ".")
		}
		fmt.Println()
		if bad {
			failing++
		}
		seen := map[string]bool{}
		for _, f := range r.Findings {
			byCode[f.Code]++
			if !seen[f.Code] {
				seen[f.Code] = true
				pagesWith[f.Code]++
			}
			if detail {
				mark := " "
				if f.Hint {
					mark = "?"
				}
				fmt.Printf("    %s %-22s %s\n", mark, f.Code, f.Detail)
			}
		}
	}
	codes := make([]string, 0, len(byCode))
	for c := range byCode {
		codes = append(codes, c)
	}
	sort.Slice(codes, func(i, j int) bool { return byCode[codes[i]] > byCode[codes[j]] })
	fmt.Println()
	for _, c := range codes {
		fmt.Printf("%5d  %-24s on %d page(s)\n", byCode[c], c, pagesWith[c])
	}
	fmt.Printf("\n%d place(s): %d failing, %d with no page\n", len(rows), failing, missing)
	if failing > 0 || missing > 0 {
		os.Exit(1)
	}
}
