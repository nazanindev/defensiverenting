// triage is the command-line face of the drafting toolbelt for the review
// queue's triage pass: an agent working the reviewer notes of ADR-018 reads
// a page with its pending notes, fetches sources through the same extractor
// the checker uses, and validates a batch of proposals before cmd/propose
// files them. Nothing here writes to the database.
//
//	triage pages                 pages with pending reviewer notes
//	triage page <playbook-id>    the page's statements, keys, citations, notes
//	triage fetch <url>           readable text of a source, via the toolbelt
//	triage find <jurisdiction>   candidate primary sources for a place
//	triage check <file.json>     lint bodies, confirm quotes, check resolves
//	triage narrow                statute quotes too short to monitor their provision
//	triage recite <entries.json>  re-source listed citations by hand; a propose file
//	triage widen [-dry] <narrow.json>
//	                             widen those quotes to their subsection; a propose file
//	triage stands <file.json> -by <name> [-apply]
//	                             reject the listed notes as "stands as written"
//	triage decide widen [-apply] the review agent approves widen-quote proposals by rule (ADR-021)
//	triage decide flag [<decisions.json> [-apply]]
//	                             list draft-page flags; close those the reader answered with a verbatim passage
//	triage decide edit [<decisions.json> [-apply]]
//	                             list draft-page edits; apply those the reader accepted, every quote confirmed live
//	triage decide pass [<decisions.json> [-apply]]
//	                             list unstamped draft statements; stamp those the reader passed with a passage (ADR-022)
//	triage decide audit          what the review agent has decided
//	triage reject <id>... -by <name> -note <why> [-apply]
//	                             reject the listed drift findings with one note
//	triage merge [-apply]        re-file triage edits a widening superseded, quote carried
//
// stands, reject, and decide are the subcommands that write. stands and
// reject each record a person's decision over many items in one run
// instead of one click per item: stands closes the notes a triage pass
// judged fine; reject closes drift findings the checker should not have
// filed (a fetch that returned a script shell, a redirect page, or fused
// PDF text), with the reason kept as the record. The person named in -by
// runs them. decide is the review agent's own decision (ADR-021), recorded
// under its name with the rule it applied; a person reads what it did
// with decide audit. Without -apply each prints what it would decide.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/nazanindev/defensiverenting/internal/discover"
	"github.com/nazanindev/defensiverenting/internal/drafting"
	"github.com/nazanindev/defensiverenting/internal/store"
	"github.com/nazanindev/defensiverenting/internal/voice"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fatal(fmt.Errorf("DATABASE_URL is not set"))
	}
	ctx := context.Background()
	pg, err := store.New(ctx, dsn)
	if err != nil {
		fatal(err)
	}
	defer pg.Close()
	tb := drafting.New(pg)

	switch os.Args[1] {
	case "pages":
		pages(ctx, pg)
	case "page":
		id, err := strconv.ParseInt(arg(2), 10, 64)
		if err != nil {
			fatal(fmt.Errorf("page id: %w", err))
		}
		page(ctx, pg, id)
	case "fetch":
		out, err := tb.FetchSource(ctx, drafting.FetchSourceInput{URL: arg(2)})
		if err != nil {
			fatal(err)
		}
		if out.Via != "" {
			fmt.Fprintf(os.Stderr, "via: %s (a quote from a snapshot is never confirmed)\n", out.Via)
		}
		if out.Truncated {
			fmt.Fprintln(os.Stderr, "note: text was truncated for length")
		}
		fmt.Println(out.Text)
	case "narrow":
		narrow(ctx, pg)
	case "widen":
		widen(ctx, pg, tb, os.Args[2:])
	case "recite":
		recite(ctx, pg, tb, arg(2))
	case "find":
		out, err := tb.FindSources(ctx, drafting.FindSourcesInput{JurisdictionSlug: arg(2)})
		if err != nil {
			fatal(err)
		}
		emit(out.Candidates)
	case "check":
		check(ctx, pg, tb, arg(2))
	case "stands":
		stands(ctx, pg, os.Args[2:])
	case "reject":
		rejectDrift(ctx, pg, os.Args[2:])
	case "withdraw":
		withdraw(ctx, pg, os.Args[2:])
	case "merge":
		merge(ctx, pg, os.Args[2:])
	case "decide":
		decide(ctx, pg, os.Args[2:])
	default:
		usage()
	}
}

func arg(i int) string {
	if len(os.Args) <= i {
		usage()
	}
	return os.Args[i]
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: triage pages | page <id> | narrow | widen <narrow.json> | recite <entries.json> | fetch <url> | find <jurisdiction-slug> | check <file.json> | stands <file.json> -by <name> [-apply] | reject <id>... -by <name> -note <why> [-apply] | withdraw <id>... -note <why> [-apply] | merge [-apply] | decide widen [-apply] [-limit n] | decide flag [<decisions.json> [-apply]] | decide edit [<decisions.json> [-apply]] | decide pass [<decisions.json> [-apply]] | decide work | decide page [<findings.json> [-apply]] | decide audit")
	os.Exit(2)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "triage:", err)
	os.Exit(1)
}

func emit(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fatal(err)
	}
}

type pageRow struct {
	PlaybookID   int64  `json:"playbook_id"`
	Title        string `json:"title"`
	Status       string `json:"status"`
	Jurisdiction string `json:"jurisdiction_slug"`
	Topic        string `json:"topic_slug"`
	Notes        int    `json:"pending_notes"`
}

func pages(ctx context.Context, pg *store.PG) {
	rows, err := pg.ListProposalsByReason(ctx, "pending", "note")
	if err != nil {
		fatal(err)
	}
	var out []pageRow
	for _, r := range rows {
		if n := len(out); n > 0 && out[n-1].PlaybookID == r.TargetPlaybookID {
			out[n-1].Notes++
			continue
		}
		pw, err := pg.AuthorGetPlaybook(ctx, r.TargetPlaybookID)
		if err != nil {
			fatal(err)
		}
		out = append(out, pageRow{PlaybookID: r.TargetPlaybookID, Title: r.Title, Status: r.TargetStatus,
			Jurisdiction: pw.Jurisdiction.Slug, Topic: pw.Topic.Slug, Notes: 1})
	}
	emit(out)
}

type narrowOut struct {
	PlaybookID   int64  `json:"playbook_id"`
	Title        string `json:"title"`
	Status       string `json:"status"`
	Jurisdiction string `json:"jurisdiction_slug"`
	Topic        string `json:"topic_slug"`
	Position     int    `json:"position"`
	Key          string `json:"statement_key"`
	BodyMD       string `json:"body_md"`
	URL          string `json:"url"`
	Publisher    string `json:"publisher"`
	Kind         string `json:"kind"`
	Locator      string `json:"locator"`
	Quote        string `json:"quote"`
	Words        int    `json:"words"`
}

// narrow lists the statute and regulation citations whose quote is too
// short to monitor the provision (drafting.NarrowQuote): the input to a
// widening pass, which fetches each source, replaces the sentence with the
// whole subsection, and files the result as an edit with cmd/propose.
func narrow(ctx context.Context, pg *store.PG) {
	rows, err := pg.ListNarrowQuotes(ctx, drafting.NarrowQuoteWords)
	if err != nil {
		fatal(err)
	}
	out := make([]narrowOut, 0, len(rows))
	for _, r := range rows {
		out = append(out, narrowOut{PlaybookID: r.PlaybookID, Title: r.Title, Status: r.PageStatus, Jurisdiction: r.JurisdictionSlug, Topic: r.TopicSlug,
			Position: r.Position, Key: r.StatementKey, BodyMD: r.BodyMD, URL: r.URL, Publisher: r.Publisher, Kind: r.Kind, Locator: r.Locator, Quote: r.Quote, Words: r.Words})
	}
	emit(out)
}

type noteOut struct {
	ID   int64  `json:"id"`
	Note string `json:"note"`
}

type citationOut struct {
	URL       string `json:"url"`
	Publisher string `json:"publisher"`
	Kind      string `json:"kind"`
	Locator   string `json:"locator"`
	Quote     string `json:"quote"`
}

type statementOut struct {
	Position  int           `json:"position"`
	Key       string        `json:"key"`
	BodyMD    string        `json:"body_md"`
	Concept   string        `json:"concept,omitempty"`
	TopicRef  string        `json:"topic_ref,omitempty"`
	Citations []citationOut `json:"citations"`
	Notes     []noteOut     `json:"notes,omitempty"`
}

type pageOut struct {
	PlaybookID   int64          `json:"playbook_id"`
	Title        string         `json:"title"`
	Status       string         `json:"status"`
	Jurisdiction string         `json:"jurisdiction_slug"`
	Topic        string         `json:"topic_slug"`
	Language     string         `json:"language"`
	Statements   []statementOut `json:"statements"`
}

func page(ctx context.Context, pg *store.PG, id int64) {
	pw, err := pg.AuthorGetPlaybook(ctx, id)
	if err != nil {
		fatal(err)
	}
	rows, err := pg.ListProposalsByReason(ctx, "pending", "note")
	if err != nil {
		fatal(err)
	}
	notes := map[string][]noteOut{}
	for _, r := range rows {
		if r.TargetPlaybookID != id {
			continue
		}
		var f store.ReviewerFlagEvidence
		if json.Unmarshal(r.Evidence, &f) != nil {
			continue
		}
		notes[r.StatementKey] = append(notes[r.StatementKey], noteOut{ID: r.ID, Note: f.Note})
	}
	out := pageOut{PlaybookID: pw.ID, Title: pw.Title, Status: pw.Status, Jurisdiction: pw.Jurisdiction.Slug,
		Topic: pw.Topic.Slug, Language: pw.Language}
	for i, st := range pw.Statements {
		so := statementOut{Position: i + 1, Key: st.Key, BodyMD: st.BodyMD, Concept: st.ConceptSlug, TopicRef: st.TopicRefSlug, Notes: notes[st.Key]}
		for _, c := range st.Citations {
			so.Citations = append(so.Citations, citationOut{URL: c.SourceURL, Publisher: c.Publisher, Kind: c.SourceKind, Locator: c.Locator, Quote: c.Quote})
		}
		out.Statements = append(out.Statements, so)
	}
	emit(out)
}

// fileEntry mirrors cmd/propose's input so a checked file files unchanged.
type fileEntry struct {
	StatementKey string                   `json:"statement_key"`
	PlaybookID   int64                    `json:"playbook_id,omitempty"`
	Reason       string                   `json:"reason"`
	Proposed     *store.ProposedStatement `json:"proposed"`
	Evidence     json.RawMessage          `json:"evidence"`
}

// check reports every problem cmd/propose or an approval would hit: an
// unknown reason, a body the voice lint refuses, a citation whose quote is
// not verbatim in the page as fetched now, a reference-only host, or a
// resolves list that names something other than a pending note on the
// same statement. Exit status 1 when anything is wrong.
func check(ctx context.Context, pg *store.PG, tb *drafting.Toolbelt, path string) {
	raw, err := os.ReadFile(path) // #nosec G703 -- the file named on the command line is the job
	if err != nil {
		fatal(err)
	}
	var entries []fileEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		fatal(fmt.Errorf("decode %s: %w", path, err))
	}
	pending, err := pg.ListProposalsByReason(ctx, "pending", "note")
	if err != nil {
		fatal(err)
	}
	noteKey := map[int64]string{}
	for _, r := range pending {
		noteKey[r.ID] = r.StatementKey
	}
	texts := map[string]string{}
	seen := map[string]int{}
	problems := 0
	bad := func(i int, format string, args ...any) {
		problems++
		fmt.Printf("entry %d: %s\n", i+1, fmt.Sprintf(format, args...))
	}
	for i, e := range entries {
		key := strings.ToLower(strings.TrimSpace(e.StatementKey))
		if first, dup := seen[key]; dup {
			bad(i, "same statement as entry %d; filing both would supersede the first. Merge them into one proposal", first)
		}
		seen[key] = i + 1
		if !store.ValidReason(e.Reason) {
			bad(i, "reason %q is not valid", e.Reason)
		}
		var ev struct {
			Note     string  `json:"note"`
			Resolves []int64 `json:"resolves"`
		}
		if len(e.Evidence) > 0 {
			if err := json.Unmarshal(e.Evidence, &ev); err != nil {
				bad(i, "evidence is not an object: %v", err)
			}
		}
		if strings.TrimSpace(ev.Note) == "" {
			bad(i, "evidence.note is empty: tell the reviewer what you did and why")
		}
		for _, id := range ev.Resolves {
			if k, ok := noteKey[id]; !ok {
				bad(i, "resolves %d is not a pending reviewer note", id)
			} else if k != key {
				bad(i, "resolves %d is a note on a different statement (%s)", id, k)
			}
		}
		if e.Proposed == nil {
			continue
		}
		// A remove or reorder carries no statement text; the store checks
		// it against the page when it is applied (ADR-025 D4).
		switch e.Proposed.Action {
		case store.ActionRemove:
			continue
		case store.ActionReorder:
			if len(e.Proposed.Order) == 0 {
				bad(i, "a reorder lists every key on the page in the new order")
			}
			continue
		case store.ActionMerge:
			if strings.TrimSpace(e.Proposed.MergeKey) == "" {
				bad(i, "a merge names merge_key, the statement it folds in")
			}
		case "":
		default:
			bad(i, "action %q is not remove, merge, or reorder", e.Proposed.Action)
			continue
		}
		if strings.TrimSpace(e.Proposed.BodyMD) == "" {
			bad(i, "proposed.body_md is empty; use null proposed for a work item")
		}
		// The check judges what the proposal changes. A body it leaves as
		// it reads today, or a citation it carries over unchanged, is not
		// the proposal's doing: an older statement that trips a lint rule
		// written since, or cites a source that blocks the fetcher, must
		// not stop a widened quote on another of its citations.
		current, err := pg.StatementByKey(ctx, key)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			fatal(err)
		}
		lang := "en"
		if e.Proposed.BodyMD != current.BodyMD {
			if v := voice.LintAll(lang, map[string]string{"body_md": e.Proposed.BodyMD}); len(v) > 0 {
				bad(i, "voice lint:\n  - %s", strings.Join(v, "\n  - "))
			}
			if why := voice.HarderThan(lang, current.BodyMD, e.Proposed.BodyMD); why != "" {
				bad(i, "%s", why)
			}
		}
		if len(e.Proposed.Citations) == 0 {
			bad(i, "no citations")
		}
		for fi, f := range e.Proposed.Followers {
			if v := voice.LintAll(lang, map[string]string{"body_md": f.BodyMD}); len(v) > 0 {
				bad(i, "follower %d voice lint:\n  - %s", fi+1, strings.Join(v, "\n  - "))
			}
			if len(f.Citations) == 0 {
				bad(i, "follower %d has no citations", fi+1)
			}
		}
		cites := append([]store.ProposedCitation{}, e.Proposed.Citations...)
		for _, f := range e.Proposed.Followers {
			cites = append(cites, f.Citations...)
		}
		for ci, c := range cites {
			// Site guidance, however an agent spells it: the flag, the kind,
			// or the /editorial url the reader lists show.
			if c.Editorial || c.Kind == "editorial" || c.URL == editorialURL {
				continue
			}
			if slices.ContainsFunc(current.Citations, func(x store.ProposedCitation) bool {
				return x.URL == c.URL && strings.TrimSpace(x.Quote) == strings.TrimSpace(c.Quote)
			}) {
				continue // carried over as it is
			}
			if c.URL == "" || strings.TrimSpace(c.Quote) == "" {
				bad(i, "citation %d needs url and quote", ci+1)
				continue
			}
			if discover.ReferenceOnly(c.URL) {
				bad(i, "citation %d cites %s, which is reference-only; cite the law it summarizes", ci+1, c.URL)
				continue
			}
			text, ok := texts[c.URL]
			if !ok {
				out, err := tb.FetchSource(ctx, drafting.FetchSourceInput{URL: c.URL})
				if err != nil {
					bad(i, "citation %d: %v", ci+1, err)
					texts[c.URL] = ""
					continue
				}
				if out.Via != "" {
					fmt.Printf("entry %d: citation %d read via %s; the quote will not count as confirmed\n", i+1, ci+1, out.Via)
				}
				text = out.Text
				texts[c.URL] = text
			}
			if text != "" && !drafting.QuoteAppearsIn(text, c.Quote) {
				bad(i, "citation %d quote is not verbatim in %s: %q", ci+1, c.URL, truncate(c.Quote, 100))
			}
			if n := drafting.NarrowQuote(c.Kind, c.Locator, c.Quote); n != "" {
				// A warning, as at save time: the entry still files.
				fmt.Printf("entry %d: citation %d: %s\n", i+1, ci+1, n)
			}
		}
	}
	fmt.Printf("%d entries, %d problems\n", len(entries), problems)
	if problems > 0 {
		os.Exit(1)
	}
}

type standEntry struct {
	NoteID       int64  `json:"note_id"`
	StatementKey string `json:"statement_key"`
	Reason       string `json:"reason"`
}

// stands rejects each listed reviewer note with the standing decision note
// plus the triage reason, under the named person. It refuses an entry whose
// id is not a pending note on the stated key, so a stale file cannot decide
// something else.
func stands(ctx context.Context, pg *store.PG, args []string) {
	fs := flag.NewFlagSet("stands", flag.ExitOnError)
	by := fs.String("by", "", "the person deciding (first name)")
	apply := fs.Bool("apply", false, "write the decisions; default prints them")
	if len(args) < 1 {
		usage()
	}
	path := args[0]
	if err := fs.Parse(args[1:]); err != nil {
		fatal(err)
	}
	if strings.TrimSpace(*by) == "" {
		fatal(fmt.Errorf("-by is required: the person deciding"))
	}
	raw, err := os.ReadFile(path) // #nosec G703 -- the file named on the command line is the job
	if err != nil {
		fatal(err)
	}
	var entries []standEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		fatal(fmt.Errorf("decode %s: %w", path, err))
	}
	pending, err := pg.ListProposalsByReason(ctx, "pending", "note")
	if err != nil {
		fatal(err)
	}
	byID := map[int64]store.ProposalRow{}
	for _, r := range pending {
		byID[r.ID] = r
	}
	decided := 0
	for i, e := range entries {
		row, ok := byID[e.NoteID]
		if !ok || row.StatementKey != strings.ToLower(strings.TrimSpace(e.StatementKey)) {
			fmt.Printf("entry %d: note %d is not a pending note on %s; skipped\n", i+1, e.NoteID, e.StatementKey)
			continue
		}
		var f store.ReviewerFlagEvidence
		_ = json.Unmarshal(row.Evidence, &f)
		fmt.Printf("#%d %s\n    note:   %s\n    stands: %s\n", e.NoteID, row.Title, truncate(f.Note, 140), e.Reason)
		if !*apply {
			continue
		}
		note := "Read; the statement stands as written. Triage: " + strings.TrimSpace(e.Reason)
		if err := pg.DecideProposal(ctx, e.NoteID, "rejected", *by, note, nil); err != nil {
			fatal(fmt.Errorf("note %d: %w", e.NoteID, err))
		}
		decided++
	}
	if *apply {
		fmt.Printf("%d of %d notes decided by %s\n", decided, len(entries), *by)
	} else {
		fmt.Printf("%d entries; nothing written (add -apply)\n", len(entries))
	}
}

// rejectDrift rejects the listed source-drift findings under one note, for
// the case where the checker filed what a fetch failure looked like. It
// refuses an id that is not a pending drift finding, so a mistyped number
// cannot decide an edit. Flags may follow the ids.
func rejectDrift(ctx context.Context, pg *store.PG, args []string) {
	fs := flag.NewFlagSet("reject", flag.ExitOnError)
	by := fs.String("by", "", "the person deciding (first name)")
	note := fs.String("note", "", "why, kept as the record")
	apply := fs.Bool("apply", false, "write the decisions; default prints them")
	var ids []int64
	for len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			fatal(fmt.Errorf("proposal id %q: %w", args[0], err))
		}
		ids = append(ids, id)
		args = args[1:]
	}
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	if len(ids) == 0 {
		usage()
	}
	if strings.TrimSpace(*by) == "" || strings.TrimSpace(*note) == "" {
		fatal(fmt.Errorf("-by and -note are required: who decides, and why"))
	}
	pending, err := pg.ListProposalsByReason(ctx, "pending", "drift")
	if err != nil {
		fatal(err)
	}
	byID := map[int64]store.ProposalRow{}
	for _, r := range pending {
		byID[r.ID] = r
	}
	decided := 0
	for _, id := range ids {
		row, ok := byID[id]
		if !ok {
			fmt.Printf("#%d is not a pending drift finding; skipped\n", id)
			continue
		}
		var ev struct {
			SourceURL string `json:"source_url"`
			OldQuote  string `json:"old_quote"`
		}
		_ = json.Unmarshal(row.Evidence, &ev)
		fmt.Printf("#%d %s\n    source: %s\n    quote:  %s\n", id, row.Title, ev.SourceURL, truncate(ev.OldQuote, 120))
		if !*apply {
			continue
		}
		if err := pg.DecideProposal(ctx, id, "rejected", *by, strings.TrimSpace(*note), nil); err != nil {
			fatal(fmt.Errorf("proposal %d: %w", id, err))
		}
		decided++
	}
	if *apply {
		fmt.Printf("%d of %d findings rejected by %s\n", decided, len(ids), *by)
	} else {
		fmt.Printf("%d ids; nothing written (add -apply)\n", len(ids))
	}
}

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// withdraw takes back the triage agent's own pending replacements, for a
// batch filed by mistake. The store refuses anything the triage agent did
// not file, and any note.
func withdraw(ctx context.Context, pg *store.PG, args []string) {
	fs := flag.NewFlagSet("withdraw", flag.ExitOnError)
	note := fs.String("note", "", "why, kept as the record")
	apply := fs.Bool("apply", false, "write the withdrawals; default prints them")
	var ids []int64
	for len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			fatal(fmt.Errorf("proposal id %q: %w", args[0], err))
		}
		ids = append(ids, id)
		args = args[1:]
	}
	if err := fs.Parse(args); err != nil {
		fatal(err)
	}
	if len(ids) == 0 || strings.TrimSpace(*note) == "" {
		usage()
	}
	done := 0
	for _, id := range ids {
		if !*apply {
			fmt.Printf("#%d would be withdrawn: %s\n", id, *note)
			continue
		}
		if err := pg.WithdrawProposal(ctx, id, triageAgent, *note); err != nil {
			fmt.Printf("#%d not withdrawn: %v\n", id, err)
			continue
		}
		done++
	}
	if *apply {
		fmt.Printf("%d of %d withdrawn by %s\n", done, len(ids), triageAgent)
	}
}

// triageAgent is the name cmd/propose files the triage agent's proposals
// under (-by "triage agent"); withdraw acts only on those.
const triageAgent = "triage agent"
