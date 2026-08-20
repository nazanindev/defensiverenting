// Command draft runs the AI drafting worker: for a jurisdiction (a city, a
// state, or united-states for a nationwide page), it drafts one or more
// playbooks — each researched, cited, and verbatim-guardrailed — as drafts for
// the author to verify and publish. Runs the tool loop in Go (internal/
// draftagent); no `claude` CLI.
//
//	draft -jurisdiction boston                       # seed the core 5 topics (default)
//	draft -jurisdiction boston -topics eviction-defense
//	draft -jurisdiction united-states -topics landlord-entry
//	draft -jurisdiction boston -topics a,b,c -model claude-sonnet-5
//	draft -jurisdiction boston -topics eviction-defense -language es   # translate the published English page
//	draft -jurisdiction boston -transcripts runs                       # keep per-draft JSONL transcripts
//
// -city is an alias for -jurisdiction, kept for muscle memory and old scripts.
//
// -transcripts writes one JSONL file per draft (every model response, tool
// result, and the final spend report), which is how model runs are compared:
// run the same topics twice with different -model values and diff the "done"
// lines.
//
// Auth: ANTHROPIC_API_KEY. DB: DATABASE_URL (or -db).
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nazanindev/defensiverenting/internal/draftagent"
	"github.com/nazanindev/defensiverenting/internal/drafting"
	"github.com/nazanindev/defensiverenting/internal/store"
)

func main() {
	log.SetFlags(0)
	jurisdiction := flag.String("jurisdiction", "", "jurisdiction slug: a city like boston, a state, or united-states (required)")
	flag.StringVar(jurisdiction, "city", "", "alias for -jurisdiction")
	topicsSpec := flag.String("topics", "core", "'core' (the predetermined 5), or a comma-separated list of topic slugs")
	language := flag.String("language", "en", "language to draft in: 'en' (default) or 'es' (translates the published English page; see internal/draftagent's system prompt)")
	model := flag.String("model", "claude-haiku-4-5", "Anthropic model id")
	parallel := flag.Int("parallel", 3, "max drafts to run concurrently")
	maxSteps := flag.Int("max-steps", 30, "max tool-use turns per draft")
	transcripts := flag.String("transcripts", "", "directory for per-draft JSONL transcripts (created if missing); empty disables")
	dsn := flag.String("db", os.Getenv("DATABASE_URL"), "Postgres DSN")
	flag.Parse()

	if *jurisdiction == "" {
		log.Fatal("draft: -jurisdiction is required")
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

	if *transcripts != "" {
		if err := os.MkdirAll(*transcripts, 0o750); err != nil {
			log.Fatalf("draft: create transcripts dir: %v", err)
		}
	}
	runStamp := time.Now().UTC().Format("20060102-150405")

	fmt.Fprintf(os.Stderr, "drafting %d topic(s) for %s on %s (parallel %d)…\n\n", len(topics), *jurisdiction, *model, *parallel)

	type res struct {
		topic  draftagent.Topic
		report draftagent.Report
		err    error
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
			var tw io.Writer
			if *transcripts != "" {
				name := fmt.Sprintf("%s-%s-%s-%s-%s.jsonl", runStamp, *jurisdiction, t.Slug, *language, *model)
				f, ferr := os.Create(filepath.Join(*transcripts, name))
				if ferr != nil {
					results[i] = res{topic: t, err: fmt.Errorf("create transcript: %w", ferr)}
					return
				}
				defer f.Close()
				tw = f
			}
			report, err := draftagent.Run(ctx, tb, draftagent.Options{
				JurisdictionSlug: *jurisdiction,
				TopicSlug:        t.Slug,
				TopicName:        t.Name,
				Language:         *language,
				Model:            *model,
				MaxSteps:         *maxSteps,
				Transcript:       tw,
				Log: func(format string, a ...any) {
					fmt.Fprintf(os.Stderr, "[%s] "+format+"\n", append([]any{t.Slug}, a...)...)
				},
			})
			results[i] = res{t, report, err}
		}(i)
	}
	wg.Wait()

	okN, failN := 0, 0
	var steps int
	var usage draftagent.TokenUsage
	fmt.Println()
	for _, r := range results {
		if r.err != nil {
			failN++
			fmt.Printf("✗ %s/%s — %v\n", *jurisdiction, r.topic.Slug, r.err)
		} else {
			okN++
			fmt.Printf("✓ %s/%s\n", *jurisdiction, r.topic.Slug)
		}
		if r.report.Steps > 0 {
			fmt.Printf("    %s\n", summarize(r.report))
		}
		steps += r.report.Steps
		usage.InputTokens += r.report.Usage.InputTokens
		usage.OutputTokens += r.report.Usage.OutputTokens
		usage.CacheReadTokens += r.report.Usage.CacheReadTokens
		usage.CacheCreationTokens += r.report.Usage.CacheCreationTokens
		usage.WebSearchRequests += r.report.Usage.WebSearchRequests
	}
	fmt.Printf("\ntotals for %s: %d steps; tokens: %d in, %d out, %d cache-read, %d cache-write; %d web searches\n",
		*model, steps, usage.InputTokens, usage.OutputTokens, usage.CacheReadTokens, usage.CacheCreationTokens, usage.WebSearchRequests)
	fmt.Printf("%s: %d drafted, %d failed — review them in the authoring tool.\n", *jurisdiction, okN, failN)
	if failN > 0 {
		os.Exit(1)
	}
}

// summarize renders a Report as the one-line spend summary under each result.
func summarize(r draftagent.Report) string {
	return fmt.Sprintf("%d steps, %d tool calls (%d rejected), %s; tokens: %d in, %d out, %d cache-read, %d cache-write; %d web searches",
		r.Steps, r.ToolCalls, r.Rejections, r.Duration.Round(time.Second),
		r.Usage.InputTokens, r.Usage.OutputTokens, r.Usage.CacheReadTokens, r.Usage.CacheCreationTokens, r.Usage.WebSearchRequests)
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
