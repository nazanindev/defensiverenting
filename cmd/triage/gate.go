package main

import (
	"context"
	"fmt"
	"strconv"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// gate lists what the publish gate holds against each page: the same page
// issues the dashboard shows, one line per issue, "clean" when none.
//
//	triage gate <playbook-id>...
func gate(ctx context.Context, pg *store.PG, args []string) {
	if len(args) == 0 {
		fatal(fmt.Errorf("gate: name at least one page id"))
	}
	for _, a := range args {
		id, err := strconv.ParseInt(a, 10, 64)
		if err != nil {
			fatal(fmt.Errorf("page id %q: %w", a, err))
		}
		issues, err := pg.AuthorPlaybookIssues(ctx, id)
		if err != nil {
			fatal(err)
		}
		if len(issues) == 0 {
			fmt.Printf("page %d: clean\n", id)
			continue
		}
		for _, is := range issues {
			fmt.Printf("page %d: %s: %s\n", id, is.Code, is.Detail)
		}
	}
}
