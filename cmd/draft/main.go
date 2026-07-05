// Command draft runs the AI drafting worker from the CLI: given a city and
// topic, Claude researches authoritative primary sources, reads them through
// our fetch_source (so the verbatim guardrail can verify quotes), and writes a
// status="draft" playbook. The loop lives in internal/draftagent, shared with
// the authoring server's "Generate draft" button.
//
// Auth: ANTHROPIC_API_KEY. DB: DATABASE_URL (or -db).
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/nazanindev/defensiverenting/internal/draftagent"
	"github.com/nazanindev/defensiverenting/internal/drafting"
	"github.com/nazanindev/defensiverenting/internal/store"
)

func main() {
	log.SetFlags(0)
	city := flag.String("city", "", "city slug, e.g. boston (required)")
	topic := flag.String("topic", "", "topic slug, e.g. security-deposits (required)")
	topicName := flag.String("topic-name", "", "topic display name (defaults to a title-cased slug)")
	dsn := flag.String("db", os.Getenv("DATABASE_URL"), "Postgres DSN")
	model := flag.String("model", "", "Anthropic model id (default claude-opus-4-8)")
	maxSteps := flag.Int("max-steps", 30, "max tool-use turns before giving up")
	flag.Parse()

	if *city == "" || *topic == "" {
		log.Fatal("draft: -city and -topic are required")
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

	err = draftagent.Run(ctx, drafting.New(pg), draftagent.Options{
		CitySlug:  *city,
		TopicSlug: *topic,
		TopicName: *topicName,
		Model:     *model,
		MaxSteps:  *maxSteps,
		Log:       func(format string, a ...any) { fmt.Fprintf(os.Stderr, format+"\n", a...) },
	})
	if err != nil {
		log.Fatalf("draft: %v", err)
	}
	fmt.Printf("✓ draft saved for %s/%s — open it in the authoring tool to verify and publish.\n", *city, *topic)
}
