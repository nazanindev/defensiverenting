// backfill-notes files reviewer notes from the pre-ADR-018 review sheets as
// queue items on the statements they concern, so the doubts the drafting
// agents wrote into markdown reach the review queue like every note since.
//
//	backfill-notes -file review-flags.json            # dry run: resolve and print
//	backfill-notes -file review-flags.json -apply     # file them
//
// The file is an array of notes. Each names a page by slugs and a statement
// either by number or by a phrase its body contains:
//
//	[{"jurisdiction": "california", "topic": "security-deposits",
//	  "stmt": 14, "match": "", "note": "…"},
//	 {"jurisdiction": "texas", "topic": "eviction-defense",
//	  "stmt": 0, "match": "three days", "note": "…"}]
//
// The draft in the slot is preferred; the live page is used when there is
// no draft. A note whose statement cannot be resolved is reported and
// skipped, never guessed. Filing goes through store.FileReviewerNote, so a
// second run files nothing new.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/nazanindev/defensiverenting/internal/store"
)

type noteEntry struct {
	Jurisdiction string `json:"jurisdiction"`
	Topic        string `json:"topic"`
	Stmt         int    `json:"stmt"`
	Match        string `json:"match"`
	Note         string `json:"note"`
}

type resolved struct {
	entry      noteEntry
	playbookID int64
	status     string
	position   int
	key        string
	body       string
	problem    string
}

func main() {
	dsn := flag.String("db", os.Getenv("DATABASE_URL"), "Postgres DSN")
	file := flag.String("file", "", "JSON file of notes")
	by := flag.String("by", store.ActorDraftingAgent, "who the notes are from")
	apply := flag.Bool("apply", false, "file the notes; without it, resolve and print only")
	flag.Parse()
	if *file == "" {
		fmt.Fprintln(os.Stderr, "usage: backfill-notes -file review-flags.json [-apply]")
		os.Exit(2)
	}
	raw, err := os.ReadFile(*file)
	if err != nil {
		fatal(err)
	}
	var entries []noteEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		fatal(fmt.Errorf("decode %s: %w", *file, err))
	}

	ctx := context.Background()
	pg, err := store.New(ctx, *dsn)
	if err != nil {
		fatal(err)
	}
	defer pg.Close()

	pages := map[string]store.PlaybookWithStatements{}
	var out []resolved
	for _, e := range entries {
		r := resolved{entry: e}
		if strings.TrimSpace(e.Note) == "" {
			r.problem = "empty note"
			out = append(out, r)
			continue
		}
		slot := e.Jurisdiction + "/" + e.Topic
		pw, ok := pages[slot]
		if !ok {
			pw, err = findPage(ctx, pg, e.Jurisdiction, e.Topic)
			if err != nil {
				r.problem = err.Error()
				out = append(out, r)
				continue
			}
			pages[slot] = pw
		}
		r.playbookID, r.status = pw.Playbook.ID, pw.Playbook.Status
		st, pos, problem := pick(pw.Statements, e)
		if problem != "" {
			r.problem = problem
			out = append(out, r)
			continue
		}
		r.position, r.key, r.body = pos, st.Key, st.BodyMD
		out = append(out, r)
	}

	filed, skipped := 0, 0
	for i, r := range out {
		if r.problem != "" {
			skipped++
			fmt.Printf("%3d. SKIP %s/%s stmt %d %q: %s\n", i+1, r.entry.Jurisdiction, r.entry.Topic, r.entry.Stmt, r.entry.Match, r.problem)
			continue
		}
		fmt.Printf("%3d. %s/%s (%s #%d) stmt %d %q\n       note: %s\n", i+1, r.entry.Jurisdiction, r.entry.Topic, r.status, r.playbookID, r.position, clip(r.body, 70), r.entry.Note)
		if !*apply {
			continue
		}
		ok, err := pg.FileReviewerNote(ctx, r.playbookID, r.key, r.entry.Note, *by)
		if err != nil {
			fatal(fmt.Errorf("entry %d: %w", i+1, err))
		}
		if ok {
			filed++
			fmt.Println("       filed")
		} else {
			fmt.Println("       already on file")
		}
	}
	if *apply {
		fmt.Printf("%d note(s) filed, %d already on file, %d skipped\n", filed, len(out)-skipped-filed, skipped)
	} else {
		fmt.Printf("%d note(s) resolved, %d skipped; nothing written (add -apply)\n", len(out)-skipped, skipped)
	}
	if skipped > 0 {
		os.Exit(1)
	}
}

// findPage returns the draft in the English slot, else the live page.
func findPage(ctx context.Context, pg *store.PG, jSlug, tSlug string) (store.PlaybookWithStatements, error) {
	j, err := pg.GetJurisdictionBySlug(ctx, jSlug)
	if err != nil {
		return store.PlaybookWithStatements{}, fmt.Errorf("jurisdiction %q: %w", jSlug, err)
	}
	t, err := pg.GetTopicBySlug(ctx, tSlug)
	if err != nil {
		return store.PlaybookWithStatements{}, fmt.Errorf("topic %q: %w", tSlug, err)
	}
	id, err := pg.AuthorFindDraft(ctx, j.ID, t.ID, "en")
	switch {
	case err == nil:
		return pg.AuthorGetPlaybook(ctx, id)
	case errors.Is(err, store.ErrNotFound):
		pw, err := pg.GetPlaybook(ctx, jSlug, tSlug, "en")
		if err != nil {
			return pw, fmt.Errorf("no draft or live page for %s/%s: %w", jSlug, tSlug, err)
		}
		return pw, nil
	default:
		return store.PlaybookWithStatements{}, err
	}
}

// pick finds the statement a note names: by 1-based position when given,
// else by a phrase its body contains, which must match exactly one.
func pick(stmts []store.CitedStatement, e noteEntry) (store.CitedStatement, int, string) {
	if e.Stmt > 0 {
		if e.Stmt > len(stmts) {
			return store.CitedStatement{}, 0, fmt.Sprintf("statement %d named but the page has %d", e.Stmt, len(stmts))
		}
		return stmts[e.Stmt-1], e.Stmt, ""
	}
	needle := strings.ToLower(strings.TrimSpace(e.Match))
	if needle == "" {
		return store.CitedStatement{}, 0, "no statement number and no match phrase"
	}
	var hits []int
	for i, st := range stmts {
		if strings.Contains(strings.ToLower(st.BodyMD), needle) {
			hits = append(hits, i)
		}
	}
	switch len(hits) {
	case 0:
		return store.CitedStatement{}, 0, fmt.Sprintf("no statement contains %q", e.Match)
	case 1:
		return stmts[hits[0]], hits[0] + 1, ""
	default:
		return store.CitedStatement{}, 0, fmt.Sprintf("%d statements contain %q", len(hits), e.Match)
	}
}

func clip(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return string(r)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
