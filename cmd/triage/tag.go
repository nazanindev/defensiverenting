package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// tagEntry is one tag decision from a tagging pass: the statement, the
// question it answers ("" when it answers none), and why.
type tagEntry struct {
	Key     string `json:"key"`
	Concept string `json:"concept"`
	Why     string `json:"why"`
}

// fillTags gives untagged statements their question (ADR-036 D8: a
// statement is a child of the answer for its place and question). It fills
// an empty tag only, never replaces one, changes no words, and keeps a
// valid review stamp valid. Without -apply it reads and counts.
//
//	triage tag <tags.json> [-apply]
func fillTags(ctx context.Context, pg *store.PG, args []string) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		fatal(errors.New("usage: triage tag <tags.json> [-apply]"))
	}
	path, args := args[0], args[1:]
	fs := flag.NewFlagSet("tag", flag.ExitOnError)
	apply := fs.Bool("apply", false, "write the tags")
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	raw, err := os.ReadFile(path) // #nosec G703 G304 -- the operator names the file
	if err != nil {
		fatal(err)
	}
	var entries []tagEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		fatal(fmt.Errorf("decode %s: %w", path, err))
	}
	concepts, err := pg.ListConcepts(ctx)
	if err != nil {
		fatal(err)
	}
	known := map[string]bool{}
	for _, c := range concepts {
		known[c.Slug] = true
	}
	perConcept := map[string]int{}
	none, bad, rows, statements, already := 0, 0, 0, 0, 0
	for i, e := range entries {
		if e.Concept == "" {
			none++
			continue
		}
		if !known[e.Concept] {
			bad++
			fmt.Fprintf(os.Stderr, "entry %d: %q is not a registry concept\n", i+1, e.Concept)
			continue
		}
		if !*apply {
			perConcept[e.Concept]++
			statements++
			continue
		}
		n, err := pg.FillStatementTag(ctx, e.Key, e.Concept)
		switch {
		case err != nil:
			bad++
			fmt.Fprintf(os.Stderr, "entry %d: %s: %v\n", i+1, e.Key, err)
		case n == 0:
			already++
		default:
			perConcept[e.Concept]++
			statements++
			rows += n
		}
	}
	slugs := make([]string, 0, len(perConcept))
	for s := range perConcept {
		slugs = append(slugs, s)
	}
	sort.Slice(slugs, func(i, j int) bool { return perConcept[slugs[i]] > perConcept[slugs[j]] })
	for _, s := range slugs {
		fmt.Printf("%4d  %s\n", perConcept[s], s)
	}
	if *apply {
		fmt.Printf("tagged %d statement(s) on %d row(s); %d already carried the tag; %d answer no question; %d refused\n", statements, rows, already, none, bad)
		return
	}
	fmt.Printf("would tag %d statement(s); %d answer no question; %d refused. Nothing written: add -apply.\n", statements, none, bad)
}
