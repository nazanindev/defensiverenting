package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// Rules pages (ADR-028 D4, D5, D9).
//
//	triage gaps <place> [<rules-topic>]
//	                     per rules topic: each concept's answer, record, or gap
//	triage nolaw <records.json> [-apply]
//	                     file "searched, no law found" coverage records
//
// gaps is what a rules-page drafter gets: only the concepts with neither a
// statement nor a record, each with its question. nolaw is the reviewer's
// record that the law was searched for and not found; it refuses a place
// that already has a statement with the concept.

type gapAnswer struct {
	Concept  string `json:"concept"`
	Question string `json:"question"`
	// One of "answered", "no-law", "gap".
	State     string `json:"state"`
	Page      string `json:"page,omitempty"`
	Status    string `json:"status,omitempty"`
	CheckedAt string `json:"checked_at,omitempty"`
}

type gapTopic struct {
	RulesTopic string      `json:"rules_topic"`
	Gaps       int         `json:"gaps"`
	Answers    []gapAnswer `json:"answers"`
}

func gaps(ctx context.Context, pg *store.PG, args []string) {
	if len(args) < 1 {
		usage()
	}
	all, err := pg.RulesGaps(ctx, args[0])
	if err != nil {
		fatal(err)
	}
	var out []gapTopic
	for _, g := range all {
		if len(args) > 1 && g.RulesTopicSlug != args[1] {
			continue
		}
		gt := gapTopic{RulesTopic: g.RulesTopicSlug, Gaps: len(g.Gaps())}
		for _, a := range g.Answers {
			ga := gapAnswer{Concept: a.Concept.Slug, Question: a.Concept.Question, State: "gap"}
			switch {
			case a.Statement != nil:
				ga.State, ga.Page, ga.Status = "answered", a.PageTitle, a.Status
			case a.NoLaw != nil:
				ga.State, ga.CheckedAt = "no-law", a.NoLaw.CheckedAt.Format("2006-01-02")
			}
			gt.Answers = append(gt.Answers, ga)
		}
		out = append(out, gt)
	}
	emit(out)
}

// noLawEntry is one record in a nolaw file.
type noLawEntry struct {
	Place          string   `json:"place"`
	Concept        string   `json:"concept"`
	SourcesChecked []string `json:"sources_checked"`
	Note           string   `json:"note"`
}

func nolaw(ctx context.Context, pg *store.PG, args []string) {
	fs := flag.NewFlagSet("nolaw", flag.ExitOnError)
	apply := fs.Bool("apply", false, "file the records")
	by := fs.String("by", store.ActorReviewAgent, "who searched")
	if len(args) < 1 {
		usage()
	}
	if err := fs.Parse(args[1:]); err != nil {
		fatal(err)
	}
	raw, err := os.ReadFile(args[0]) // #nosec G703 G304 -- the operator names the file, like psql -f
	if err != nil {
		fatal(err)
	}
	var entries []noLawEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		fatal(fmt.Errorf("%s: %w", args[0], err))
	}
	filed := 0
	for _, e := range entries {
		if len(e.SourcesChecked) == 0 {
			fmt.Printf("skip %s %s: no place searched\n", e.Place, e.Concept)
			continue
		}
		if !*apply {
			fmt.Printf("would file: %s %s (%d sources)\n", e.Place, e.Concept, len(e.SourcesChecked))
			continue
		}
		err := pg.FileCoverageRecord(ctx, store.FileCoverageParams{
			JurisdictionSlug: e.Place, ConceptSlug: e.Concept, SourcesChecked: e.SourcesChecked, Note: e.Note, By: *by,
		})
		if err != nil {
			fmt.Printf("refused %s %s: %v\n", e.Place, e.Concept, err)
			continue
		}
		filed++
		fmt.Printf("filed: %s %s\n", e.Place, e.Concept)
	}
	if *apply {
		fmt.Printf("%d of %d filed\n", filed, len(entries))
	}
}

// retagEntry is one tag decision: the statement, the concept it should carry
// ("" to take a wrong tag off), and why.
type retagEntry struct {
	Key     string `json:"key"`
	Concept string `json:"concept"`
	Why     string `json:"why"`
}

// retag turns tag decisions into a propose file (ADR-028 D9 step 3): each
// statement as it stands, with only its concept changed. Body and citations
// are copied unchanged, so a person approving it changes the tag and nothing
// else. Reads only; cmd/propose files the output.
func retag(ctx context.Context, pg *store.PG, path string) {
	raw, err := os.ReadFile(path) // #nosec G703 G304 -- the operator names the file
	if err != nil {
		fatal(err)
	}
	var entries []retagEntry
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
	type proposal struct {
		StatementKey string                   `json:"statement_key"`
		Reason       string                   `json:"reason"`
		Proposed     *store.ProposedStatement `json:"proposed"`
		Evidence     map[string]string        `json:"evidence"`
	}
	var out []proposal
	for i, e := range entries {
		if e.Concept != "" && !known[e.Concept] {
			fmt.Fprintf(os.Stderr, "entry %d: %q is not a registry concept\n", i+1, e.Concept)
			continue
		}
		st, err := pg.StatementByKey(ctx, e.Key)
		if err != nil {
			fmt.Fprintf(os.Stderr, "entry %d: %s: %v\n", i+1, e.Key, err)
			continue
		}
		if st.Concept == e.Concept {
			continue
		}
		if st.TopicRef != "" && e.Concept != "" {
			fmt.Fprintf(os.Stderr, "entry %d: %s is a whole-topic summary; it carries no concept\n", i+1, e.Key)
			continue
		}
		from := st.Concept
		st.Concept = e.Concept
		out = append(out, proposal{
			StatementKey: e.Key,
			Reason:       "agent-pass:tag-change",
			Proposed:     &st,
			Evidence:     map[string]string{"note": "Tag change only (ADR-028 D9): " + tagLabel(from) + " to " + tagLabel(e.Concept) + ". " + e.Why},
		})
	}
	fmt.Fprintf(os.Stderr, "%d of %d entries change a tag\n", len(out), len(entries))
	emit(out)
}

func tagLabel(slug string) string {
	if slug == "" {
		return "no tag"
	}
	return slug
}
