// cmd/promote copies playbooks from one database into another, preserving the
// verbatim citation quotes that back every statement.
//
// The two databases have independent id spaces, so nothing is copied by id.
// Jurisdictions and topics are matched by slug (missing ancestors are created
// so a city never lands without its state), sources by URL, and the playbook
// itself is written through the same IngestPlaybook path the drafting agent
// uses — which enforces the "every statement cites a source" guarantee on the
// destination too, inside one transaction per playbook.
//
// Deliberately not a pg_dump: dumping rows would carry ids that mean something
// different in the destination, and the markdown content format is lossy (it
// has no field for a citation quote, so a round-trip through it would drop the
// evidence that makes a statement verifiable).
//
// Dry run unless -apply is given. The dry run reports exactly what would be
// created, reused, and overwritten, and writes nothing.
//
//	promote -dst "$DSN" -status draft -exclude boston
//	promote -dst "$DSN" -status draft -exclude boston -apply
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// localDSN is the development default for the source database. Promoting to a
// real destination requires PROMOTE_DST_DSN, which has no default.
const localDSN = "postgres://postgres:postgres@localhost:5432/tenants?sslmode=disable" // #nosec G101

func main() {
	src := flag.String("src", envOr("PROMOTE_SRC_DSN", localDSN), "source Postgres DSN")
	dst := flag.String("dst", os.Getenv("PROMOTE_DST_DSN"), "destination Postgres DSN")
	status := flag.String("status", "draft", "only promote playbooks with this status")
	exclude := flag.String("exclude", "", "comma-separated jurisdiction slugs to skip")
	skipExisting := flag.Bool("skip-existing", false, "skip playbooks the destination already has, instead of replacing them (use to resume an interrupted run)")
	attempts := flag.Int("attempts", 4, "attempts per playbook before giving up")
	apply := flag.Bool("apply", false, "actually write to the destination")
	flag.Parse()

	if *dst == "" {
		fatal("-dst (or PROMOTE_DST_DSN) is required")
	}

	ctx := context.Background()
	srcPG, err := store.New(ctx, *src)
	if err != nil {
		fatal("connect source: %v", err)
	}
	defer srcPG.Close()
	dstPG, err := store.New(ctx, *dst)
	if err != nil {
		fatal("connect destination: %v", err)
	}
	defer dstPG.Close()

	skip := map[string]bool{}
	for _, s := range strings.Split(*exclude, ",") {
		if s = strings.TrimSpace(s); s != "" {
			skip[s] = true
		}
	}

	plans, err := build(ctx, srcPG, dstPG, *status, skip, *skipExisting)
	if err != nil {
		fatal("%v", err)
	}
	if len(plans) == 0 {
		fmt.Println("Nothing to promote.")
		return
	}
	report(plans, skip, *apply)

	if !*apply {
		fmt.Println("\nDry run — nothing was written. Re-run with -apply to promote.")
		return
	}
	// One playbook's failure must not abandon the other 25. A dropped
	// connection is the likely cause (this runs over a `fly proxy` tunnel) and
	// the pool re-dials on the next acquire, so continuing usually recovers.
	var failed []string
	for _, p := range plans {
		if err := promoteWithRetry(ctx, srcPG, dstPG, p, *attempts); err != nil {
			fmt.Printf("  FAILED   %s/%s: %v\n", p.CitySlug, p.TopicSlug, err)
			failed = append(failed, p.CitySlug+"/"+p.TopicSlug)
			continue
		}
		fmt.Printf("  promoted %s/%s\n", p.CitySlug, p.TopicSlug)
	}

	fmt.Printf("\n%d of %d promoted.\n", len(plans)-len(failed), len(plans))
	if len(failed) > 0 {
		fmt.Printf("%d failed:\n  %s\n", len(failed), strings.Join(failed, "\n  "))
		fmt.Println("\nRe-run with -skip-existing -apply to retry only what is missing.")
		os.Exit(1)
	}
}

// plan is one playbook's promotion, with everything the report needs to
// describe it before any of it happens.
type plan struct {
	SrcID              int64
	CitySlug, CityName string
	TopicSlug          string
	Title              string
	Language           string
	Statements         int
	Citations          int
	Sources            int
	NewJurisdictions   []string // slugs that do not exist in the destination yet
	NewTopic           bool
	Overwrites         string // existing destination status, or "" when the slot is free
}

func build(ctx context.Context, srcPG, dstPG *store.PG, status string, skip map[string]bool, skipExisting bool) ([]plan, error) {
	rows, err := srcPG.AuthorListPlaybooks(ctx)
	if err != nil {
		return nil, fmt.Errorf("list source playbooks: %w", err)
	}

	// What the destination already holds, keyed the same way the playbooks
	// unique constraint is: jurisdiction + topic + language.
	dstRows, err := dstPG.AuthorListPlaybooks(ctx)
	if err != nil {
		return nil, fmt.Errorf("list destination playbooks: %w", err)
	}
	occupied := map[string]string{}
	for _, r := range dstRows {
		occupied[key(r.JurisdictionSlug, r.TopicSlug, r.Language)] = r.Status
	}

	// Cache lookups so a 26-playbook run does not re-ask the destination for
	// the same five cities twice each.
	jurSeen, topicSeen := map[string]bool{}, map[string]bool{}

	var plans []plan
	for _, r := range rows {
		if r.Status != status || skip[r.JurisdictionSlug] {
			continue
		}
		full, err := srcPG.AuthorGetPlaybook(ctx, r.ID)
		if err != nil {
			return nil, fmt.Errorf("read source playbook %d: %w", r.ID, err)
		}
		if skipExisting && occupied[key(full.Jurisdiction.Slug, full.Topic.Slug, full.Playbook.Language)] != "" {
			continue
		}

		p := plan{
			SrcID:     r.ID,
			CitySlug:  full.Jurisdiction.Slug,
			CityName:  full.Jurisdiction.Name,
			TopicSlug: full.Topic.Slug,
			Title:     full.Playbook.Title,
			Language:  full.Playbook.Language,
			Overwrites: occupied[key(full.Jurisdiction.Slug, full.Topic.Slug,
				full.Playbook.Language)],
		}

		urls := map[string]bool{}
		for _, st := range full.Statements {
			p.Statements++
			for _, c := range st.Citations {
				p.Citations++
				urls[c.SourceURL] = true
			}
		}
		p.Sources = len(urls)

		// Which ancestors are missing in the destination, walking up from the
		// city so a state is reported before the city that needs it.
		chain, err := ancestry(ctx, srcPG, full.Jurisdiction)
		if err != nil {
			return nil, err
		}
		for _, j := range chain {
			if jurSeen[j.Slug] {
				continue
			}
			jurSeen[j.Slug] = true
			if _, err := dstPG.GetJurisdictionBySlug(ctx, j.Slug); errors.Is(err, store.ErrNotFound) {
				p.NewJurisdictions = append(p.NewJurisdictions, j.Slug)
			} else if err != nil {
				return nil, fmt.Errorf("check jurisdiction %s: %w", j.Slug, err)
			}
		}
		if !topicSeen[full.Topic.Slug] {
			topicSeen[full.Topic.Slug] = true
			if _, err := dstPG.GetTopicBySlug(ctx, full.Topic.Slug); errors.Is(err, store.ErrNotFound) {
				p.NewTopic = true
			} else if err != nil {
				return nil, fmt.Errorf("check topic %s: %w", full.Topic.Slug, err)
			}
		}
		plans = append(plans, p)
	}
	sort.Slice(plans, func(i, j int) bool {
		if plans[i].CitySlug != plans[j].CitySlug {
			return plans[i].CitySlug < plans[j].CitySlug
		}
		return plans[i].TopicSlug < plans[j].TopicSlug
	})
	return plans, nil
}

// ancestry returns j's ancestors outermost-first, then j itself, so callers can
// create or check them in an order where every parent precedes its child.
func ancestry(ctx context.Context, pg *store.PG, j store.Jurisdiction) ([]store.Jurisdiction, error) {
	var chain []store.Jurisdiction
	for cur := j; ; {
		chain = append([]store.Jurisdiction{cur}, chain...)
		if cur.ParentID == nil {
			return chain, nil
		}
		parent, err := pg.GetJurisdictionByID(ctx, *cur.ParentID)
		if err != nil {
			return nil, fmt.Errorf("resolve parent of %s: %w", cur.Slug, err)
		}
		cur = parent
	}
}

// promoteWithRetry retries a playbook with backoff before giving up.
//
// This normally runs across a `fly proxy` tunnel, which flaps: connections die
// mid-run and the listener recovers seconds later. Every step of promote() is
// idempotent — jurisdictions and topics resolve by slug, sources by URL, and
// IngestPlaybook upserts on (jurisdiction, topic, language) — so retrying after
// a partial failure re-does work rather than duplicating it.
func promoteWithRetry(ctx context.Context, srcPG, dstPG *store.PG, p plan, attempts int) error {
	var err error
	for i := range attempts {
		if i > 0 {
			delay := time.Duration(1<<uint(i-1)) * 2 * time.Second
			fmt.Printf("  retrying %s/%s in %s (%v)\n", p.CitySlug, p.TopicSlug, delay, err)
			time.Sleep(delay)
		}
		if err = promote(ctx, srcPG, dstPG, p); err == nil {
			return nil
		}
	}
	return err
}

func promote(ctx context.Context, srcPG, dstPG *store.PG, p plan) error {
	full, err := srcPG.AuthorGetPlaybook(ctx, p.SrcID)
	if err != nil {
		return err
	}

	chain, err := ancestry(ctx, srcPG, full.Jurisdiction)
	if err != nil {
		return err
	}
	var parentID *int64
	var jurID int64
	for _, j := range chain {
		existing, err := dstPG.GetJurisdictionBySlug(ctx, j.Slug)
		switch {
		case err == nil:
			jurID = existing.ID
		case errors.Is(err, store.ErrNotFound):
			created, cerr := dstPG.UpsertJurisdiction(ctx, store.UpsertJurisdictionParams{
				ParentID: parentID, Kind: j.Kind, Name: j.Name, Slug: j.Slug,
			})
			if cerr != nil {
				return fmt.Errorf("create jurisdiction %s: %w", j.Slug, cerr)
			}
			jurID = created.ID
		default:
			return fmt.Errorf("lookup jurisdiction %s: %w", j.Slug, err)
		}
		id := jurID
		parentID = &id
	}

	topic, err := dstPG.UpsertTopic(ctx, store.UpsertTopicParams{
		Slug: full.Topic.Slug, Name: full.Topic.Name,
	})
	if err != nil {
		return fmt.Errorf("resolve topic %s: %w", full.Topic.Slug, err)
	}

	// Sources are shared, so map by URL rather than carrying source ids across.
	srcIDByURL := map[string]int64{}
	for _, st := range full.Statements {
		for _, c := range st.Citations {
			if _, done := srcIDByURL[c.SourceURL]; done {
				continue
			}
			s, serr := dstPG.UpsertSource(ctx, store.UpsertSourceParams{
				URL: c.SourceURL, Publisher: c.Publisher,
				JurisdictionID: &jurID, Kind: c.SourceKind,
			})
			if serr != nil {
				return fmt.Errorf("upsert source %s: %w", c.SourceURL, serr)
			}
			srcIDByURL[c.SourceURL] = s.ID
		}
	}

	stmts := make([]store.IngestStatementParams, 0, len(full.Statements))
	for _, st := range full.Statements {
		cites := make([]store.IngestCitationParams, 0, len(st.Citations))
		for _, c := range st.Citations {
			cites = append(cites, store.IngestCitationParams{
				SourceID: srcIDByURL[c.SourceURL], Locator: c.Locator, Quote: c.Quote,
			})
		}
		stmts = append(stmts, store.IngestStatementParams{
			BodyMD: st.BodyMD, Language: full.Playbook.Language, Sources: cites,
		})
	}

	return dstPG.IngestPlaybook(ctx, store.IngestPlaybookParams{
		JurisdictionID: jurID,
		TopicID:        topic.ID,
		Language:       full.Playbook.Language,
		Slug:           full.Playbook.Slug,
		Title:          full.Playbook.Title,
		IntroMD:        full.Playbook.IntroMD,
		PageKind:       full.Playbook.PageKind,
		Status:         full.Playbook.Status,
		Statements:     stmts,
	})
}

func report(plans []plan, skip map[string]bool, apply bool) {
	mode := "DRY RUN"
	if apply {
		mode = "APPLYING"
	}
	fmt.Printf("%s — %d playbooks\n\n", mode, len(plans))

	var newJur []string
	var newTopics, overwrites, stmts, cites int
	seenTopic := map[string]bool{}
	for _, p := range plans {
		newJur = append(newJur, p.NewJurisdictions...)
		if p.NewTopic && !seenTopic[p.TopicSlug] {
			seenTopic[p.TopicSlug] = true
			newTopics++
		}
		if p.Overwrites != "" {
			overwrites++
		}
		stmts += p.Statements
		cites += p.Citations
	}

	for _, p := range plans {
		flag := "new"
		if p.Overwrites != "" {
			flag = "OVERWRITES " + p.Overwrites
		}
		fmt.Printf("  %-16s %-28s %3d stmts %3d cites  %s\n",
			p.CitySlug, p.TopicSlug, p.Statements, p.Citations, flag)
	}

	fmt.Printf("\n  statements: %d   citations: %d\n", stmts, cites)
	if len(newJur) > 0 {
		fmt.Printf("  jurisdictions to create: %s\n", strings.Join(newJur, ", "))
	}
	if newTopics > 0 {
		fmt.Printf("  topics to create: %d\n", newTopics)
	}
	if len(skip) > 0 {
		var s []string
		for k := range skip {
			s = append(s, k)
		}
		sort.Strings(s)
		fmt.Printf("  excluded jurisdictions: %s\n", strings.Join(s, ", "))
	}
	if overwrites > 0 {
		fmt.Printf("\n  WARNING: %d playbook(s) already exist in the destination and would be\n"+
			"  replaced, including their statements. Pass -skip-existing to leave them\n"+
			"  alone, or exclude the jurisdiction, unless replacing is what you want.\n", overwrites)
	}
}

func key(jur, topic, lang string) string { return jur + "\x00" + topic + "\x00" + lang }

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "promote: "+format+"\n", args...)
	os.Exit(1)
}
