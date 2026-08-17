// Command draft runs the AI drafting worker: for a city, it drafts one or more
// playbooks — each researched, cited, and verbatim-guardrailed — as drafts for
// the author to verify and publish. Runs the tool loop in Go (internal/
// draftagent); no `claude` CLI.
//
//	draft -city boston                       # seed the core 5 topics (default)
//	draft -city boston -topics eviction-defense
//	draft -city boston -topics a,b,c -model claude-sonnet-5
//	draft -city boston -topics eviction-defense -language es   # translate the published English page
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
	language := flag.String("language", "en", "language to draft in: 'en' (default) or 'es' (translates the published English page; see internal/draftagent's system prompt)")
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
	ctx := context.Background()
	pg, err := store.New(ctx, *dsn)
	if err != nil {
		log.Fatalf("draft: connect db: %v", err)
	}
	defer pg.Close()

	// Resolved against the registry, so an unknown topic fails now rather than
	// after minutes of paid API calls.
	topics, err := resolveTopics(ctx, pg, *topicsSpec)
	if err != nil {
		log.Fatalf("draft: %v", err)
	}
	if len(topics) == 0 {
		log.Fatal("draft: no topics to draft")
	}
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
				Language:  *language,
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

// resolveTopics turns the -topics spec into a topic list: "core" (the topics
// flagged is_core in the registry) or a comma-separated list of slugs.
//
// Both forms resolve against the topics table rather than a list compiled into
// this binary. A slug that is not in the registry is refused here, so the run
// fails in a second instead of after minutes of API calls that save_draft would
// then reject. See docs/ADRs/ADR-005 D5.
func resolveTopics(ctx context.Context, pg *store.PG, spec string) ([]draftagent.Topic, error) {
	registry, err := pg.ListTopicRegistry(ctx)
	if err != nil {
		return nil, fmt.Errorf("read topic registry: %w", err)
	}
	bySlug := make(map[string]store.Topic, len(registry))
	for _, t := range registry {
		bySlug[t.Slug] = t
	}

	spec = strings.TrimSpace(spec)
	if spec == "" || spec == "core" {
		var out []draftagent.Topic
		for _, t := range registry {
			if t.IsCore {
				out = append(out, draftagent.Topic{Slug: t.Slug, Name: t.Name})
			}
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("no core topics in the registry — has migration 000014 run?")
		}
		return out, nil
	}

	var out []draftagent.Topic
	for _, s := range strings.Split(spec, ",") {
		slug := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), " ", "-")
		if slug == "" {
			continue
		}
		t, ok := bySlug[slug]
		if !ok {
			return nil, fmt.Errorf("unknown topic %q — topics are a fixed registry; known slugs: %s",
				slug, strings.Join(slugsOf(registry), ", "))
		}
		out = append(out, draftagent.Topic{Slug: t.Slug, Name: t.Name})
	}
	return out, nil
}

func slugsOf(ts []store.Topic) []string {
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		out = append(out, t.Slug)
	}
	return out
}
