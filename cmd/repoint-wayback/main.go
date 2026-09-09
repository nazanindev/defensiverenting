// Command repoint-wayback migrates citations off web.archive.org snapshot URLs
// and onto the live pages those snapshots preserve. Early drafting (before the
// fetcher grew its headless-render tier) could not read bot-blocking sites, so
// the drafting agent cited the snapshot URL itself; this repairs those rows.
//
// A source is repointed only when the live page fetches and every non-empty
// cited quote still appears in it verbatim — the same check the save guardrail
// runs, via the same functions. A source whose live page cannot be fetched is
// left on its snapshot URL and reported. A source whose live page fetched but
// where a quote has drifted is also held, and each drifted statement gets a
// source-drift proposal in the review queue (ADR-014 D4): the statement with
// that citation moved to the live URL and the nearest passage offered as the
// replacement quote when the text merely moved. The snapshot never drifts, so
// check-sources cannot see this; only a comparison against the live page can.
// Dry run by default; -apply writes.
//
// DB: DATABASE_URL (or -db).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nazanindev/defensiverenting/internal/drafting"
	"github.com/nazanindev/defensiverenting/internal/sourcecheck"
	"github.com/nazanindev/defensiverenting/internal/store"
)

// checkedBy is the actor handle stamped on citations this command confirms,
// in the same short style as "source check" and "drafting agent".
const checkedBy = "wayback repoint"

// reWayback extracts the original URL embedded in a snapshot URL. The middle
// segment is the timestamp in whichever precision the citing agent used
// ("2025", "20250101000000", "2id_").
var reWayback = regexp.MustCompile(`^https?://web\.archive\.org/web/[^/]+/(https?://.+)$`)

type citation struct {
	statementID int64
	key         string // the statement's durable key (ADR-014 D1)
	quote       string
}

func main() {
	log.SetFlags(0)
	dsn := flag.String("db", os.Getenv("DATABASE_URL"), "Postgres DSN")
	apply := flag.Bool("apply", false, "write the repointing; default is a dry run")
	flag.Parse()
	if *dsn == "" {
		log.Fatal("repoint-wayback: DATABASE_URL (or -db) is required")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, *dsn)
	if err != nil {
		log.Fatalf("repoint-wayback: connect db: %v", err)
	}
	defer pool.Close()
	db, err := store.New(ctx, *dsn)
	if err != nil {
		log.Fatalf("repoint-wayback: connect store: %v", err)
	}
	defer db.Close()

	rows, err := pool.Query(ctx, `
		SELECT id, url, publisher FROM sources
		WHERE url LIKE 'https://web.archive.org/%' OR url LIKE 'http://web.archive.org/%'
		ORDER BY id`)
	if err != nil {
		log.Fatalf("repoint-wayback: list snapshot sources: %v", err)
	}
	type src struct {
		id       int64
		url, pub string
	}
	var srcs []src
	for rows.Next() {
		var s src
		if err := rows.Scan(&s.id, &s.url, &s.pub); err != nil {
			log.Fatalf("repoint-wayback: scan: %v", err)
		}
		srcs = append(srcs, s)
	}
	rows.Close()
	if len(srcs) == 0 {
		fmt.Println("no snapshot-URL sources found — nothing to do")
		return
	}

	repointed, held, proposed := 0, 0, 0
	for _, s := range srcs {
		m := reWayback.FindStringSubmatch(s.url)
		if m == nil {
			held++
			fmt.Printf("⚑ HOLD source %d (%s): unrecognized snapshot URL form %s\n", s.id, s.pub, s.url)
			continue
		}
		live := m[1]

		cites, err := listCitations(ctx, pool, s.id)
		if err != nil {
			log.Fatalf("repoint-wayback: citations of source %d: %v", s.id, err)
		}

		text, err := drafting.FetchExtract(live)
		if err != nil {
			held++
			fmt.Printf("⚑ HOLD source %d (%s): live page did not fetch: %s: %v\n", s.id, s.pub, live, err)
			continue
		}

		var missing []citation
		unverifiable := 0
		for _, c := range cites {
			switch {
			case c.quote == "":
				unverifiable++
			case !drafting.QuoteAppearsIn(text, c.quote):
				missing = append(missing, c)
			}
		}
		if len(missing) > 0 {
			held++
			fmt.Printf("⚑ HOLD source %d (%s): %d of %d cited quote(s) not on the live page %s\n",
				s.id, s.pub, len(missing), len(cites), live)
			for _, c := range missing {
				fmt.Printf("    statement %d: %q\n", c.statementID, clip(c.quote, 90))
				n, err := proposeDrift(ctx, db, s.url, live, s.pub, text, c, *apply)
				if err != nil {
					log.Fatalf("repoint-wayback: propose drift for statement %d: %v", c.statementID, err)
				}
				proposed += n
			}
			continue
		}

		repointed++
		note := ""
		if unverifiable > 0 {
			note = fmt.Sprintf(" (%d empty-quote citation(s) carried over unverified)", unverifiable)
		}
		if !*apply {
			fmt.Printf("· would repoint source %d (%s): %d citation(s) → %s%s\n", s.id, s.pub, len(cites), live, note)
			continue
		}
		if err := repoint(ctx, pool, s.id, live); err != nil {
			log.Fatalf("repoint-wayback: repoint source %d: %v", s.id, err)
		}
		fmt.Printf("✓ repointed source %d (%s): %d citation(s) → %s%s\n", s.id, s.pub, len(cites), live, note)
	}

	verb := "would repoint"
	if *apply {
		verb = "repointed"
	}
	filed := "would file"
	if *apply {
		filed = "filed"
	}
	fmt.Printf("%s %d of %d snapshot source(s); %d held for human review; %s %d drift proposal(s) for the review queue\n",
		verb, repointed, len(srcs), held, filed, proposed)
	if !*apply && (repointed > 0 || proposed > 0) {
		fmt.Println("dry run — re-run with -apply to write")
	}
}

// proposeDrift files one source-drift proposal for a statement whose quote is
// not on the live page: the statement as it stands, with this citation moved
// from the snapshot URL to the live URL and its quote replaced by the nearest
// passage when the match is close, or a finding when it is not. Approval on
// the queue then performs the repoint for that statement. Returns 1 when a
// proposal was (or in a dry run, would be) filed.
func proposeDrift(ctx context.Context, db *store.PG, snapshotURL, liveURL, publisher, liveText string, c citation, apply bool) (int, error) {
	if c.key == "" {
		return 0, nil // orphaned statement, no page cites it
	}
	done, err := db.DriftAlreadyFiled(ctx, c.key, c.quote)
	if err != nil || done {
		return 0, err
	}
	passage, score := sourcecheck.Nearest(liveText, c.quote)
	evidence := map[string]any{
		"snapshot_url": snapshotURL, "source_url": liveURL, "publisher": publisher,
		"old_quote": c.quote, "new_quote": passage, "similarity": score,
	}
	var proposed *store.ProposedStatement
	if score >= 0.6 {
		st, err := db.StatementByKey(ctx, c.key)
		if err != nil {
			if err == store.ErrNotFound {
				return 0, nil
			}
			return 0, err
		}
		for i := range st.Citations {
			if st.Citations[i].URL == snapshotURL && st.Citations[i].Quote == c.quote {
				st.Citations[i].URL = liveURL
				st.Citations[i].Quote = passage
				st.Citations[i].Checked = true // verbatim in the live text just fetched
			}
		}
		proposed = &st
		evidence["note"] = "This citation points at an archive snapshot. On the live page the quoted text has changed; the nearest passage is offered as the replacement, citing the live URL. Check the statement is still true under it."
	} else {
		evidence["note"] = "This citation points at an archive snapshot, and the quoted text is not on the live page at all. The page was rewritten or reorganized; re-verify the statement by hand against the live URL."
	}
	if !apply {
		fmt.Printf("      would file a source-drift proposal (similarity %.2f)\n", score)
		return 1, nil
	}
	ev, _ := json.Marshal(evidence)
	id, err := db.FileProposal(ctx, store.FileProposalParams{
		StatementKey: c.key, Reason: "source-drift", Proposed: proposed, Evidence: ev, ProposedBy: checkedBy,
	})
	if err != nil {
		return 0, err
	}
	fmt.Printf("      filed source-drift proposal %d (similarity %.2f)\n", id, score)
	return 1, nil
}

func listCitations(ctx context.Context, pool *pgxpool.Pool, sourceID int64) ([]citation, error) {
	rows, err := pool.Query(ctx, `
		SELECT c.statement_id, COALESCE(s.key::text, ''), c.quote
		FROM citations c JOIN statements s ON s.id = c.statement_id
		WHERE c.source_id = $1 ORDER BY c.statement_id`, sourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []citation
	for rows.Next() {
		var c citation
		if err := rows.Scan(&c.statementID, &c.key, &c.quote); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// repoint moves one source's citations onto the live URL, inside a single
// transaction. When a row for the live URL already exists (sources.url is
// UNIQUE), the citations move to it and the snapshot row is deleted; otherwise
// the snapshot row's url is rewritten in place. Either way the quotes were
// just confirmed against the live page, so checked_at/checked_by are stamped
// on every non-empty-quote citation — empty quotes verified nothing and keep
// their prior state.
func repoint(ctx context.Context, pool *pgxpool.Pool, snapID int64, live string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	// Rollback after a successful Commit is a documented no-op error; there
	// is nothing to do with it either way.
	defer func() { _ = tx.Rollback(ctx) }()

	var liveID int64
	err = tx.QueryRow(ctx, `SELECT id FROM sources WHERE url = $1`, live).Scan(&liveID)
	switch err {
	case nil:
		// Merge into the existing live row. A statement citing both the
		// snapshot and the live row keeps the live citation it already has.
		if _, err := tx.Exec(ctx, `
			INSERT INTO citations (statement_id, source_id, locator, quote, manually_verified, checked_at, checked_by)
			SELECT statement_id, $1, locator, quote, manually_verified,
			       CASE WHEN quote <> '' THEN NOW() ELSE checked_at END,
			       CASE WHEN quote <> '' THEN $3 ELSE checked_by END
			FROM citations WHERE source_id = $2
			ON CONFLICT (statement_id, source_id) DO NOTHING`, liveID, snapID, checkedBy); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM citations WHERE source_id = $1`, snapID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE source_candidates SET source_id = $1 WHERE source_id = $2`, liveID, snapID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM sources WHERE id = $1`, snapID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE sources SET retrieved_at = NOW(), last_checked_at = NOW() WHERE id = $1`, liveID); err != nil {
			return err
		}
	case pgx.ErrNoRows:
		if _, err := tx.Exec(ctx, `
			UPDATE sources SET url = $1, retrieved_at = NOW(), last_checked_at = NOW() WHERE id = $2`, live, snapID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE citations SET checked_at = NOW(), checked_by = $2
			WHERE source_id = $1 AND quote <> ''`, snapID, checkedBy); err != nil {
			return err
		}
	default:
		return err
	}
	return tx.Commit(ctx)
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
