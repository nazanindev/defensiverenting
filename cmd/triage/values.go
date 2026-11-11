package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// Rule values (ADR-034).
//
//	triage values list [-place <slug>] [-concept <slug>]
//	                     the rows as they stand, with their citations and stamps
//	triage values input <concept>
//	                     every law-backed statement tagged to the concept, with
//	                     its quotes: what the backfill agent reads (D7)
//	triage values <values.json> [-by <name>] [-remove] [-apply]
//	                     file (or take back) rows; without -apply, say what would happen
//
// A number lives once per place, concept and condition. Filing the same key
// again replaces the row. The source is named by URL and must already be a
// source some statement cites; a new source is a drafting decision.

// valueEntry is one row in a values file.
type valueEntry struct {
	Place     string   `json:"place"`
	Concept   string   `json:"concept"`
	Condition string   `json:"condition,omitempty"`
	Kind      string   `json:"kind"`
	Value     float64  `json:"value"`
	ValueMax  *float64 `json:"value_max,omitempty"`
	URL       string   `json:"url"`
	Locator   string   `json:"locator,omitempty"`
	Quote     string   `json:"quote"`
	Note      string   `json:"note,omitempty"`
}

type valueRow struct {
	valueEntry
	Publisher       string `json:"publisher,omitempty"`
	CheckedBy       string `json:"checked_by"`
	CheckedAt       string `json:"checked_at"`
	SourceCheckedAt string `json:"source_checked_at,omitempty"`
	DriftAt         string `json:"drift_at,omitempty"`
	DriftNote       string `json:"drift_note,omitempty"`
}

type valueInput struct {
	Key       string                   `json:"key"`
	Place     string                   `json:"place"`
	PlaceName string                   `json:"place_name"`
	Page      string                   `json:"page"`
	Status    string                   `json:"status"`
	Body      string                   `json:"body"`
	Citations []store.ProposedCitation `json:"citations"`
}

func values(ctx context.Context, pg *store.PG, args []string) {
	if len(args) < 1 {
		usage()
	}
	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("values list", flag.ExitOnError)
		place := fs.String("place", "", "only this place")
		concept := fs.String("concept", "", "only this concept")
		if err := fs.Parse(args[1:]); err != nil {
			fatal(err)
		}
		rows, err := pg.ListRuleValues(ctx, *place, *concept)
		if err != nil {
			fatal(err)
		}
		out := make([]valueRow, 0, len(rows))
		for _, r := range rows {
			v := valueRow{valueEntry: valueEntry{
				Place: r.JurisdictionSlug, Concept: r.ConceptSlug, Condition: r.Condition,
				Kind: r.Kind, Value: r.Value, ValueMax: r.ValueMax,
				URL: r.SourceURL, Locator: r.Locator, Quote: r.Quote, Note: r.Note,
			}, Publisher: r.Publisher, CheckedBy: r.CheckedBy, CheckedAt: r.CheckedAt.Format("2006-01-02"), DriftNote: r.DriftNote}
			if r.SourceCheckedAt != nil {
				v.SourceCheckedAt = r.SourceCheckedAt.Format("2006-01-02")
			}
			if r.DriftAt != nil {
				v.DriftAt = r.DriftAt.Format("2006-01-02")
			}
			out = append(out, v)
		}
		emit(out)
	case "input":
		if len(args) < 2 {
			usage()
		}
		rows, err := pg.StatementsForConcept(ctx, args[1])
		if err != nil {
			fatal(err)
		}
		out := make([]valueInput, 0, len(rows))
		for _, r := range rows {
			out = append(out, valueInput{Key: r.Key, Place: r.JurisdictionSlug, PlaceName: r.JurisdictionName,
				Page: r.PageTitle, Status: r.Status, Body: r.BodyMD, Citations: r.Citations})
		}
		emit(out)
	default:
		fileValues(ctx, pg, args)
	}
}

func fileValues(ctx context.Context, pg *store.PG, args []string) {
	fs := flag.NewFlagSet("values", flag.ExitOnError)
	apply := fs.Bool("apply", false, "file the rows")
	remove := fs.Bool("remove", false, "take the named rows back instead of filing them")
	by := fs.String("by", store.ActorReviewAgent, "who filed")
	if err := fs.Parse(args[1:]); err != nil {
		fatal(err)
	}
	raw, err := os.ReadFile(args[0]) // #nosec G703 G304 -- the operator names the file, like psql -f
	if err != nil {
		fatal(err)
	}
	var entries []valueEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		fatal(fmt.Errorf("%s: %w", args[0], err))
	}
	name := func(e valueEntry) string {
		if e.Condition == "" {
			return e.Place + " " + e.Concept
		}
		return e.Place + " " + e.Concept + " (" + e.Condition + ")"
	}
	done := 0
	for _, e := range entries {
		if *remove {
			if !*apply {
				fmt.Printf("would remove: %s\n", name(e))
				continue
			}
			ok, err := pg.RemoveRuleValue(ctx, e.Place, e.Concept, e.Condition)
			switch {
			case err != nil:
				fmt.Printf("refused %s: %v\n", name(e), err)
			case !ok:
				fmt.Printf("no row: %s\n", name(e))
			default:
				done++
				fmt.Printf("removed: %s\n", name(e))
			}
			continue
		}
		if !*apply {
			fmt.Printf("would file: %s = %s %s\n", name(e), formatValue(e), e.Kind)
			continue
		}
		err := pg.FileRuleValue(ctx, store.FileRuleValueParams{
			JurisdictionSlug: e.Place, ConceptSlug: e.Concept, Condition: e.Condition,
			Kind: e.Kind, Value: e.Value, ValueMax: e.ValueMax,
			SourceURL: e.URL, Locator: e.Locator, Quote: e.Quote, Note: e.Note, By: *by,
		})
		if err != nil {
			fmt.Printf("refused %s: %v\n", name(e), err)
			continue
		}
		done++
		fmt.Printf("filed: %s = %s %s\n", name(e), formatValue(e), e.Kind)
	}
	if *apply {
		verb := "filed"
		if *remove {
			verb = "removed"
		}
		fmt.Printf("%d of %d %s\n", done, len(entries), verb)
	}
}

func formatValue(e valueEntry) string {
	if e.ValueMax != nil {
		return fmt.Sprintf("%g to %g", e.Value, *e.ValueMax)
	}
	return fmt.Sprintf("%g", e.Value)
}
