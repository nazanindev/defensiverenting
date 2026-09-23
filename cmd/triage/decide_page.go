package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// Page review (ADR-025, step 1: advisory, drafts only). A reviewer in a
// local session reads a page as a renter would, once every statement on it
// is stamped and nothing is pending. It sees the page skeleton only: title,
// intro, and each statement's position, key, text and concept tag, plus the
// concept gaps against the page one level up. No quotes and no sources: the
// statement loop already checked those, and a small context keeps the
// judgement on the page.
//
//	triage decide page                       ready draft pages, as skeletons
//	triage decide page <findings.json> [-apply]
//	                                         file each finding as a page note
//
// A finding is {playbook_id, kind, keys, note}. It is filed as a reviewer
// note on the first key it names, under the review agent, so it shows on
// that statement's card and holds the page from publishing until a person
// decides it. The agent never edits the page from here.

var pageKinds = []string{"duplicate", "off-topic", "order", "contradiction", "gap", "intro"}

type pageStatement struct {
	Position int    `json:"position"`
	Key      string `json:"key"`
	BodyMD   string `json:"body_md"`
	Concept  string `json:"concept,omitempty"`
}

type pageSkeleton struct {
	PlaybookID int64           `json:"playbook_id"`
	Page       string          `json:"page"`
	Title      string          `json:"title"`
	IntroMD    string          `json:"intro_md"`
	Statements []pageStatement `json:"statements"`
	Gaps       []string        `json:"concept_gaps,omitempty"`
}

type pageFinding struct {
	PlaybookID int64    `json:"playbook_id"`
	Kind       string   `json:"kind"`
	Keys       []string `json:"keys"`
	Note       string   `json:"note"`
}

func decidePage(ctx context.Context, pg *store.PG, args []string) {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		decidePageFile(ctx, pg, args[0], args[1:])
		return
	}
	pages, err := pg.AuthorListPlaybooks(ctx)
	if err != nil {
		fatal(err)
	}
	var out []pageSkeleton
	for _, row := range pages {
		if row.Status != "draft" || row.Language != "en" {
			continue
		}
		pw, err := pg.AuthorGetPlaybook(ctx, row.ID)
		if err != nil {
			fatal(err)
		}
		if !pageReady(pw) {
			continue
		}
		sk := pageSkeleton{PlaybookID: pw.ID, Page: pw.Jurisdiction.Name + " · " + pw.Topic.Name, Title: pw.Title, IntroMD: pw.IntroMD}
		for i, st := range pw.Statements {
			sk.Statements = append(sk.Statements, pageStatement{Position: i + 1, Key: st.Key, BodyMD: st.BodyMD, Concept: st.ConceptSlug})
		}
		if sk.Gaps, err = pg.ConceptGaps(ctx, pw.ID); err != nil {
			fatal(err)
		}
		out = append(out, sk)
	}
	emit(out)
	fmt.Fprintf(os.Stderr, "%d draft pages ready for page review (every statement stamped, nothing pending). Read each as a renter would; write findings.json as [{playbook_id, kind: %s, keys, note}]; then triage decide page findings.json [-apply].\n", len(out), strings.Join(pageKinds, "|"))
}

// pageReady is a draft page whose every statement carries a current stamp
// and has no open queue item.
func pageReady(pw store.PlaybookWithStatements) bool {
	if len(pw.Statements) == 0 {
		return false
	}
	for _, st := range pw.Statements {
		if st.ReviewedAt == nil || st.Undecided {
			return false
		}
	}
	return true
}

func decidePageFile(ctx context.Context, pg *store.PG, path string, args []string) {
	fs := flag.NewFlagSet("decide page", flag.ExitOnError)
	apply := fs.Bool("apply", false, "file the notes; default prints them")
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	raw, err := os.ReadFile(path) // #nosec G703 -- the file named on the command line is the job
	if err != nil {
		fatal(err)
	}
	var findings []pageFinding
	if err := json.Unmarshal(raw, &findings); err != nil {
		fatal(fmt.Errorf("decode %s: %w", path, err))
	}
	pages := map[int64]store.PlaybookWithStatements{}
	filed, refused := 0, 0
	for i, f := range findings {
		pw, ok := pages[f.PlaybookID]
		if !ok {
			if pw, err = pg.AuthorGetPlaybook(ctx, f.PlaybookID); err != nil {
				fatal(err)
			}
			pages[f.PlaybookID] = pw
		}
		why := checkFinding(pw, f)
		if why != "" {
			fmt.Printf("finding %d refused: %s\n", i+1, why)
			refused++
			continue
		}
		note := fmt.Sprintf("Page review (%s): %s Statements: %s.", f.Kind, strings.TrimSpace(f.Note), positionsOf(pw, f.Keys))
		fmt.Printf("#%d %s · %s\n    %s\n", pw.ID, pw.Jurisdiction.Name, pw.Topic.Name, note)
		if !*apply {
			continue
		}
		ok, err := pg.FileReviewerNote(ctx, pw.ID, f.Keys[0], note, store.ActorReviewAgent)
		if err != nil {
			fatal(err)
		}
		if ok {
			filed++
		}
	}
	if *apply {
		fmt.Printf("%d findings; %d page notes filed by %s; %d refused\n", len(findings), filed, store.ActorReviewAgent, refused)
	} else {
		fmt.Printf("%d findings; %d would be filed; %d refused; nothing written (add -apply)\n", len(findings), len(findings)-refused, refused)
	}
}

// checkFinding refuses a finding that names no known kind, no statement, a
// statement not on this draft page, or gives no reason.
func checkFinding(pw store.PlaybookWithStatements, f pageFinding) string {
	if pw.Status != "draft" {
		return "page review files notes on draft pages only (ADR-025 D5)"
	}
	if !slices.Contains(pageKinds, f.Kind) {
		return fmt.Sprintf("kind %q is not one of %s", f.Kind, strings.Join(pageKinds, ", "))
	}
	if strings.TrimSpace(f.Note) == "" {
		return "a finding needs a note saying what is wrong"
	}
	if len(f.Keys) == 0 {
		return "a finding names at least one statement key"
	}
	for _, k := range f.Keys {
		if !slices.ContainsFunc(pw.Statements, func(st store.CitedStatement) bool { return st.Key == k }) {
			return fmt.Sprintf("key %s is not on page %d", k, pw.ID)
		}
	}
	return ""
}

func positionsOf(pw store.PlaybookWithStatements, keys []string) string {
	var ps []string
	for _, k := range keys {
		for i, st := range pw.Statements {
			if st.Key == k {
				ps = append(ps, fmt.Sprint(i+1))
			}
		}
	}
	return strings.Join(ps, ", ")
}
