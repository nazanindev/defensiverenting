package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/nazanindev/defensiverenting/internal/drafting"
	"github.com/nazanindev/defensiverenting/internal/sourcecheck"
	"github.com/nazanindev/defensiverenting/internal/store"
)

// advice lists the registry (ADR-016, amended 2026-10-09) or attaches
// entries to draft pages. Agents reference entries; they never write them,
// so there is no verb to add one: a new entry is a migration a person
// reviews.
//
//	triage advice                       the registry, with backing status
//	triage advice check                 confirm every backing quote at its live source
//	triage advice <refs.json> [-apply]  attach [{playbook_id, slug, statement_key}]
//	triage advice <refs.json> -remove [-apply]
func advice(ctx context.Context, pg *store.PG, args []string) {
	if len(args) == 0 {
		list, err := pg.ListAdvice(ctx)
		if err != nil {
			fatal(err)
		}
		type row struct {
			Slug   string   `json:"slug"`
			Kind   string   `json:"kind"`
			Warns  string   `json:"warns,omitempty"`
			Body   string   `json:"body_md"`
			Backed bool     `json:"backed"`
			From   []string `json:"from,omitempty"`
		}
		var out []row
		for _, a := range list {
			r := row{Slug: a.Slug, Kind: a.Kind, Warns: a.Warns, Body: a.BodyMD, Backed: a.Backed()}
			for _, c := range a.Citations {
				place := c.Place
				if place == "" {
					place = "national"
				}
				r.From = append(r.From, c.Publisher+" ("+place+")")
			}
			out = append(out, r)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			fatal(err)
		}
		return
	}
	if args[0] == "check" {
		var res sourcecheck.Result
		logf := func(format string, a ...any) { fmt.Fprintf(os.Stderr, format+"\n", a...) }
		if err := sourcecheck.RunAdvice(ctx, pg, drafting.FetchExtract, logf, &res); err != nil {
			fatal(err)
		}
		fmt.Printf("advice: %d quote(s) confirmed, %d missing from their source\n", res.AdviceConfirmed, res.AdviceDrifted)
		return
	}
	fs := flag.NewFlagSet("advice", flag.ExitOnError)
	apply := fs.Bool("apply", false, "write the references")
	remove := fs.Bool("remove", false, "detach the named references instead")
	if err := fs.Parse(args[1:]); err != nil {
		fatal(err)
	}
	raw, err := os.ReadFile(args[0]) // #nosec G703 G304 -- the operator names the file, like psql -f
	if err != nil {
		fatal(err)
	}
	var refs []store.AdviceRef
	if err := json.Unmarshal(raw, &refs); err != nil {
		fatal(fmt.Errorf("%s: %w", args[0], err))
	}
	if !*apply {
		for _, r := range refs {
			verb := "attach"
			if *remove {
				verb = "remove"
			}
			fmt.Printf("would %s %s on page %d %s\n", verb, r.Slug, r.PlaybookID, r.StatementKey)
		}
		fmt.Println("nothing written (add -apply)")
		return
	}
	if *remove {
		err = pg.RemovePageAdvice(ctx, refs)
	} else {
		err = pg.SetPageAdvice(ctx, refs, store.ActorReviewAgent)
	}
	if err != nil {
		fatal(err)
	}
	fmt.Printf("%d reference(s) written\n", len(refs))
}
