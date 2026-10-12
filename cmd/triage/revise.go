package main

import (
	"context"
	"flag"
	"fmt"
	"strconv"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// revise opens a draft revision beside a live page: the live page's own
// statements, saved into the slot as a draft, keys and review stamps kept,
// the page's advice references copied (ADR-016 A2). The advice backfill
// (ADR-016 A9) works on that draft, so a live page changes only when a
// person publishes the revision.
//
//	triage revise <playbook-id>... [-apply]
func revise(ctx context.Context, pg *store.PG, args []string) {
	fs := flag.NewFlagSet("revise", flag.ExitOnError)
	apply := fs.Bool("apply", false, "write the drafts")
	var ids []int64
	rest := args
	for len(rest) > 0 {
		id, err := strconv.ParseInt(rest[0], 10, 64)
		if err != nil {
			break
		}
		ids = append(ids, id)
		rest = rest[1:]
	}
	if err := fs.Parse(rest); err != nil {
		fatal(err)
	}
	if len(ids) == 0 {
		fatal(fmt.Errorf("revise: name at least one live page id"))
	}
	for _, id := range ids {
		pb, err := pg.AuthorGetPlaybook(ctx, id)
		if err != nil {
			fatal(fmt.Errorf("page %d: %w", id, err))
		}
		if pb.Status != "published" {
			fmt.Printf("page %d is %s, not live; skipped\n", id, pb.Status)
			continue
		}
		existing, err := pg.DraftInSlot(ctx, pb.Jurisdiction.Slug, pb.Topic.Slug, pb.Language)
		if err != nil {
			fatal(err)
		}
		if existing != 0 {
			fmt.Printf("page %d (%s / %s): draft %d already in the slot; skipped\n", id, pb.Jurisdiction.Slug, pb.Topic.Slug, existing)
			continue
		}
		params := store.IngestPlaybookParams{
			JurisdictionID:  pb.Jurisdiction.ID,
			TopicID:         pb.Topic.ID,
			Language:        pb.Language,
			Slug:            pb.Topic.Slug,
			Title:           pb.Title,
			IntroMD:         pb.IntroMD,
			PageKind:        pb.PageKind,
			Status:          "draft",
			UpdatedBy:       store.ActorReviewAgent,
			AllowIncomplete: true,
		}
		for _, s := range pb.Statements {
			st := store.IngestStatementParams{BodyMD: s.BodyMD, Language: pb.Language, Key: s.Key, ConceptSlug: s.ConceptSlug, TopicRefSlug: s.TopicRefSlug, Stage: s.Stage}
			for _, c := range s.Citations {
				st.Sources = append(st.Sources, store.IngestCitationParams{SourceID: c.SourceID, Locator: c.Locator, Quote: c.Quote, ManuallyVerified: c.ManuallyVerified})
			}
			params.Statements = append(params.Statements, st)
		}
		if !*apply {
			fmt.Printf("page %d (%s / %s): would open a draft revision with %d statements\n", id, pb.Jurisdiction.Slug, pb.Topic.Slug, len(params.Statements))
			continue
		}
		if err := pg.IngestPlaybook(ctx, params); err != nil {
			fatal(fmt.Errorf("page %d: %w", id, err))
		}
		draftID, err := pg.DraftInSlot(ctx, pb.Jurisdiction.Slug, pb.Topic.Slug, pb.Language)
		if err != nil {
			fatal(err)
		}
		fmt.Printf("page %d (%s / %s): draft %d opened with %d statements\n", id, pb.Jurisdiction.Slug, pb.Topic.Slug, draftID, len(params.Statements))
	}
}
