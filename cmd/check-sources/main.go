// Command check-sources re-fetches every cited primary source and flags any
// whose content changed since it was last reviewed, for the author to reconcile
// in the authoring dashboard. Run on a cadence (cron) or on demand.
//
// DB: DATABASE_URL (or -db). No API key needed — this is a plain fetch + hash.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/nazanindev/defensiverenting/internal/drafting"
	"github.com/nazanindev/defensiverenting/internal/sourcecheck"
	"github.com/nazanindev/defensiverenting/internal/store"
)

func main() {
	log.SetFlags(0)
	dsn := flag.String("db", os.Getenv("DATABASE_URL"), "Postgres DSN")
	flag.Parse()
	if *dsn == "" {
		log.Fatal("check-sources: DATABASE_URL (or -db) is required")
	}

	ctx := context.Background()
	pg, err := store.New(ctx, *dsn)
	if err != nil {
		log.Fatalf("check-sources: connect db: %v", err)
	}
	defer pg.Close()

	res, err := sourcecheck.Run(ctx, pg, drafting.FetchExtract, func(format string, a ...any) {
		fmt.Fprintf(os.Stderr, format+"\n", a...)
	})
	if err != nil {
		log.Fatalf("check-sources: %v", err)
	}
	fmt.Printf("checked %d sources — %d flagged (cited quote missing), %d failed\n", res.Sources, res.Flagged, res.Failed)
	if res.Skipped > 0 {
		// Reported on its own line, and last, because it is the number that
		// silently made previous runs look clean: these citations were never
		// examined at all. Backfilling their quotes is what brings them in.
		fmt.Printf("NOT CHECKED: %d citation(s) have no verbatim quote — nothing about them was verified\n", res.Skipped)
	}
}
