package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/nazanindev/defensiverenting/internal/store"
	"github.com/nazanindev/defensiverenting/internal/voice"
)

// backfill plans the advice backfill of one draft page (ADR-016 A9): which
// page notes the page needs and may carry (a statement relies on the risk,
// and the note has a confirmed quote for this place), which tips fit, and
// the edits that take the now-redundant sentences out of statements. It
// writes files for the ordinary tools; it decides nothing itself and writes
// nothing to the database:
//
//	refs-<id>.json    for triage advice <file> -apply
//	props-<id>.json   for triage check + propose, then decide edit
//	review-<id>.json  editorial-only statements a tip may replace: a person reads these
//
//	triage backfill <draft-id>... -dir <folder>
func backfill(ctx context.Context, pg *store.PG, args []string) {
	fs := flag.NewFlagSet("backfill", flag.ExitOnError)
	dir := fs.String("dir", ".", "folder for the output files")
	var ids []int64
	rest := args
	for len(rest) > 0 {
		id, err := strconv.ParseInt(rest[0], 10, 64)
		if err != nil {
			break
		}
		ids = append(ids, id)
		rest = rest[1:]
	}
	if err := fs.Parse(rest); err != nil {
		fatal(err)
	}
	if len(ids) == 0 {
		fatal(fmt.Errorf("backfill: name at least one draft page id"))
	}
	for _, id := range ids {
		backfillOne(ctx, pg, id, *dir)
	}
}

// noteFor is the registry entry that says each risk once at the top.
var noteFor = map[string][]string{
	"evict":   {"withhold-risk-evict"},
	"award":   {"award-win-and-pays"},
	"condemn": {"inspector-may-condemn"},
	// The leave risk has a full note and a short one plus the site-voice
	// line (option B); the first backed pair wins.
	"owe": {"leave-risk-owe", "leave-owe-short"},
}

// tipAnchors is where each tip sits: the first statement whose text matches.
var tipAnchors = []struct {
	slug   string
	topics map[string]bool
	match  *regexp.Regexp
}{
	{"keep-a-record", nil, regexp.MustCompile(`(?i)\b(in writing|written|notice|letter|certified mail|keep a copy)\b`)},
	{"photos-of-the-problem", map[string]bool{"repairs-and-habitability": true, "heat-not-working": true, "constructive-eviction": true}, regexp.MustCompile(`(?i)\b(repair|fix|problem|condition)s?\b`)},
	{"private-inspector", map[string]bool{"repairs-and-habitability": true, "heat-not-working": true, "constructive-eviction": true}, regexp.MustCompile(`(?i)\b(inspector|inspection|code enforcement|code office|housing code|health department)\b`)},
	{"ask-for-a-receipt", map[string]bool{"cant-pay-rent": true}, regexp.MustCompile(`(?i)\b(pay|paid|payment)\b`)},
	{"plan-in-writing", map[string]bool{"cant-pay-rent": true, "rent-increase": true}, regexp.MustCompile(`(?i)\b(plan|agree|agreement|increase|raise)\b`)},
}

type backfillReview struct {
	Key  string `json:"statement_key"`
	Body string `json:"body_md"`
	Tip  string `json:"tip,omitempty"`
}

func backfillOne(ctx context.Context, pg *store.PG, id int64, dir string) {
	pb, err := pg.AuthorGetPlaybook(ctx, id)
	if err != nil {
		fatal(fmt.Errorf("page %d: %w", id, err))
	}
	if pb.Status != "draft" {
		fmt.Printf("page %d is %s, not a draft; skipped\n", id, pb.Status)
		return
	}
	backed, err := pg.BackedAdviceOn(ctx, id)
	if err != nil {
		fatal(err)
	}
	have := map[string]bool{}
	for _, a := range pb.Advice {
		have[a.Slug] = true
	}
	// Which risks the statements rely on. A statement that still carries
	// its own warning says no risk to NeedsWarning, so ask about the text
	// with every such line taken out.
	every := voice.Page{Warns: map[string]bool{"owe": true, "evict": true, "award": true, "condemn": true}, Tips: map[string]bool{"keep-a-record": true}}
	needs := map[string]bool{}
	for _, s := range pb.Statements {
		bare, _ := voice.StripSaidOnce(pb.Language, s.BodyMD, every)
		if k := voice.NeedsWarning(pb.Language, bare); k != "" {
			needs[k] = true
		}
	}
	refs := []store.AdviceRef{}
	page := voice.Page{Warns: map[string]bool{}, Tips: map[string]bool{}}
	for _, a := range pb.Advice {
		if a.Kind == "tip" {
			page.Tips[a.Slug] = true
		} else if a.Warns != "" {
			page.Warns[a.Warns] = true
		}
	}
	unbacked := []string{}
	for kind := range needs {
		if page.Warns[kind] {
			continue
		}
		attached := false
		for _, slug := range noteFor[kind] {
			if backed[slug] {
				refs = append(refs, store.AdviceRef{PlaybookID: id, Slug: slug})
				if slug == "leave-owe-short" {
					refs = append(refs, store.AdviceRef{PlaybookID: id, Slug: "leave-court-may-disagree"})
				}
				page.Warns[kind] = true
				attached = true
				break
			}
		}
		if !attached {
			unbacked = append(unbacked, kind)
		}
	}
	// An editorial-only statement is one a tip may replace (A4 step 5), so
	// a tip never anchors on it.
	editorialOnly := func(s store.CitedStatement) bool {
		for _, c := range s.Citations {
			if c.SourceKind != "editorial" {
				return false
			}
		}
		return true
	}
	for _, t := range tipAnchors {
		if have[t.slug] || (t.topics != nil && !t.topics[pb.Topic.Slug]) {
			continue
		}
		for _, s := range pb.Statements {
			if !editorialOnly(s) && t.match.MatchString(s.BodyMD) {
				refs = append(refs, store.AdviceRef{PlaybookID: id, Slug: t.slug, StatementKey: s.Key})
				page.Tips[t.slug] = true
				break
			}
		}
	}
	// Edits: the sentences the page now says once come out of statements.
	type proposal struct {
		StatementKey string                  `json:"statement_key"`
		PlaybookID   int64                   `json:"playbook_id"`
		Reason       string                  `json:"reason"`
		Proposed     store.ProposedStatement `json:"proposed"`
		Evidence     map[string]any          `json:"evidence"`
	}
	props := []proposal{}
	review := []backfillReview{}
	for _, s := range pb.Statements {
		text, removed := voice.StripSaidOnce(pb.Language, s.BodyMD, page)
		if len(removed) > 0 && text != "" {
			ps := store.ProposedStatement{BodyMD: text, Concept: s.ConceptSlug, TopicRef: s.TopicRefSlug}
			for _, c := range s.Citations {
				if c.SourceKind == "editorial" {
					ps.Citations = append(ps.Citations, store.ProposedCitation{URL: "/editorial", Publisher: c.Publisher, Kind: "editorial", Editorial: true})
					continue
				}
				ps.Citations = append(ps.Citations, store.ProposedCitation{URL: c.SourceURL, Publisher: c.Publisher, Kind: c.SourceKind, Locator: c.Locator, Quote: c.Quote, Checked: true})
			}
			props = append(props, proposal{StatementKey: s.Key, PlaybookID: id, Reason: "agent-pass:triage", Proposed: ps,
				Evidence: map[string]any{"note": "Advice backfill (ADR-016 A9): the page now says this once at the top or as a tip; removed: " + fmt.Sprint(removed)}})
		}
		if editorialOnly(s) {
			r := backfillReview{Key: s.Key, Body: s.BodyMD}
			for _, t := range tipAnchors {
				if t.match.MatchString(s.BodyMD) && (t.topics == nil || t.topics[pb.Topic.Slug]) {
					r.Tip = t.slug
					break
				}
			}
			review = append(review, r)
		}
	}
	write := func(name string, v any) {
		b, _ := json.MarshalIndent(v, "", " ")
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%s-%d.json", name, id)), b, 0o600); err != nil {
			fatal(err)
		}
	}
	write("refs", refs)
	write("props", props)
	write("review", review)
	fmt.Printf("page %d (%s / %s): %d refs, %d edits, %d editorial-only statements to read; needs %v, no backed note for %v\n",
		id, pb.Jurisdiction.Slug, pb.Topic.Slug, len(refs), len(props), len(review), keys(needs), unbacked)
}

func keys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
