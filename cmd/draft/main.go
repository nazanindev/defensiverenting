// Command draft runs the AI drafting worker: for a city, it drafts one or more
// playbooks — each researched, cited, and verbatim-guardrailed — as drafts for
// the author to verify and publish. Runs the tool loop in Go (internal/
// draftagent); no `claude` CLI.
//
//	draft -city boston                       # seed the core 5 topics (default)
//	draft -city boston -topics eviction-defense
//	draft -city boston -topics a,b,c -model claude-sonnet-5
//
// Auth: ANTHROPIC_API_KEY. DB: DATABASE_URL (or -db).
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/nazanindev/defensiverenting/internal/draftagent"
	"github.com/nazanindev/defensiverenting/internal/drafting"
	"github.com/nazanindev/defensiverenting/internal/store"
)

func main() {
	log.SetFlags(0)
	city := flag.String("city", "", "city slug, e.g. boston (required)")
	topicsSpec := flag.String("topics", "core", "'core' (the predetermined 5), or a comma-separated list of topic slugs")
	model := flag.String("model", "claude-haiku-4-5", "Anthropic model id")
	parallel := flag.Int("parallel", 3, "max drafts to run concurrently")
	maxSteps := flag.Int("max-steps", 30, "max tool-use turns per draft")
	dsn := flag.String("db", os.Getenv("DATABASE_URL"), "Postgres DSN")
	flag.Parse()

	if *city == "" {
		log.Fatal("draft: -city is required")
	}
	if *dsn == "" {
		log.Fatal("draft: DATABASE_URL (or -db) is required")
	}
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		log.Fatal("draft: ANTHROPIC_API_KEY is required")
	}
	topics := resolveTopics(*topicsSpec)
	if len(topics) == 0 {
		log.Fatal("draft: no topics to draft")
	}

	ctx := context.Background()
	pg, err := store.New(ctx, *dsn)
	if err != nil {
		log.Fatalf("draft: connect db: %v", err)
	}
	defer pg.Close()
	tb := drafting.New(pg) // shared toolbelt (fetch cache is concurrency-safe)

	fmt.Fprintf(os.Stderr, "drafting %d topic(s) for %s on %s (parallel %d)…\n\n", len(topics), *city, *model, *parallel)

	type res struct {
		topic draftagent.Topic
		err   error
	}
	results := make([]res, len(topics))
	sem := make(chan struct{}, *parallel)
	var wg sync.WaitGroup
	for i := range topics {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			t := topics[i]
			err := draftagent.Run(ctx, tb, draftagent.Options{
				CitySlug:  *city,
				TopicSlug: t.Slug,
				TopicName: t.Name,
				Model:     *model,
				MaxSteps:  *maxSteps,
				Log: func(format string, a ...any) {
					fmt.Fprintf(os.Stderr, "[%s] "+format+"\n", append([]any{t.Slug}, a...)...)
				},
			})
			results[i] = res{t, err}
		}(i)
	}
	wg.Wait()

	okN, failN := 0, 0
	fmt.Println()
	for _, r := range results {
		if r.err != nil {
			failN++
			fmt.Printf("✗ %s/%s — %v\n", *city, r.topic.Slug, r.err)
		} else {
			okN++
			fmt.Printf("✓ %s/%s\n", *city, r.topic.Slug)
		}
	}
	fmt.Printf("\n%s: %d drafted, %d failed — review them in the authoring tool.\n", *city, okN, failN)
	if failN > 0 {
		os.Exit(1)
	}
}

// resolveTopics turns the -topics spec into a topic list: "core" (the
// predetermined set) or a comma-separated list of slugs.
func resolveTopics(spec string) []draftagent.Topic {
	spec = strings.TrimSpace(spec)
	if spec == "" || spec == "core" {
		return draftagent.CoreTopics
	}
	var out []draftagent.Topic
	for _, s := range strings.Split(spec, ",") {
		slug := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), " ", "-")
		if slug != "" {
			out = append(out, draftagent.Topic{Slug: slug}) // Name empty → draftagent title-cases it
		}
	}
	return out
}
