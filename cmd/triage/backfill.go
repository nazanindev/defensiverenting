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
//	triage backfill <live-id>... -dir <folder> -live   size a topic before revisions are opened
func backfill(ctx context.Context, pg *store.PG, args []string) {
	fs := flag.NewFlagSet("backfill", flag.ExitOnError)
	dir := fs.String("dir", ".", "folder for the output files")
	live := fs.Bool("live", false, "plan against live pages too, to size a topic before revisions are opened; the files are for reading, not for -apply")
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
		backfillOne(ctx, pg, id, *dir, *live)
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

// topicWarns are the site-voice warnings every page of a topic carries,
// whether or not a statement names the step (ADR-016 A10): on a repairs
// page the renter can always call an inspector, so the condemnation
// caution leads the page. A warning that varies by state is never here.
var topicWarns = map[string]map[string]bool{
	"condemn": {"repairs-and-habitability": true, "heat-not-working": true, "constructive-eviction": true},
}

// tipAnchors is where each tip sits: the first statement whose text matches.
var tipAnchors = []struct {
	slug   string
	topics map[string]bool
	match  *regexp.Regexp
}{
	// The tip is about what the renter sends the landlord, so it sits under
	// a statement about the renter writing, never under the landlord's own
	// notice to enter or to raise the rent.
	{"keep-a-record", nil, regexp.MustCompile(`(?i)\b(written (request|demand|complaint|notice to your landlord)|demand letter|certified mail|keep a copy|(ask|tell|write to|send|give) your landlord[^.]{0,40}\b(writing|written|letter|notice|email|text)|letter to your landlord)\b`)},
	{"photos-of-the-problem", map[string]bool{"repairs-and-habitability": true, "heat-not-working": true, "constructive-eviction": true}, regexp.MustCompile(`(?i)\b(repair|fix|problem|condition)s?\b`)},
	{"private-inspector", map[string]bool{"repairs-and-habitability": true, "heat-not-working": true, "constructive-eviction": true}, regexp.MustCompile(`(?i)\b(inspector|inspection|code enforcement|code office|housing code|health department)\b`)},
	{"ask-for-a-receipt", map[string]bool{"cant-pay-rent": true}, regexp.MustCompile(`(?i)\b(pay|paid|payment)\b`)},
	// The tip is for a rent change or payment plan the renter agrees to,
	// so it sits under the renter agreeing or asking, not under the
	// landlord's notice or the lease's own terms.
	{"plan-in-writing", map[string]bool{"cant-pay-rent": true, "rent-increase": true}, regexp.MustCompile(`(?i)\b(you (both )?agree|agreed|agree (on|to|in)|negotiat\w*|payment plan|smaller increase|lower (the )?(rent|increase|amount)|ask (your landlord )?(for|to) )`)},
}

type backfillReview struct {
	Key  string `json:"statement_key"`
	Body string `json:"body_md"`
	Tip  string `json:"tip,omitempty"`
	// Stub marks a statement that would be left as a fragment once the
	// repeated line comes out ("You can then move out."): it pointed at the
	// statement before it and needs a merge, not a strip.
	Stub string `json:"stub,omitempty"`
}

// stubWords is the fewest words a statement may keep after a strip before
// it reads as a fragment and goes to a person instead.
const stubWords = 10

var pointerOpener = regexp.MustCompile(`(?i)^(you can then|then|this|these|also)\b`)

func backfillOne(ctx context.Context, pg *store.PG, id int64, dir string, live bool) {
	pb, err := pg.AuthorGetPlaybook(ctx, id)
	if err != nil {
		fatal(fmt.Errorf("page %d: %w", id, err))
	}
	sizing := live && pb.Status == "published"
	if pb.Status != "draft" && !sizing {
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
	for kind, topics := range topicWarns {
		if topics[pb.Topic.Slug] {
			needs[kind] = true
		}
	}
	for _, s := range pb.Statements {
		bare, _ := voice.StripSaidOnce(pb.Language, s.BodyMD, every)
		if k := voice.NeedsWarning(pb.Language, bare); k != "" {
			needs[k] = true
		}
		for _, k := range voice.SaidOnceKinds(pb.Language, s.BodyMD) {
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
		if len(removed) > 0 && (len(regexp.MustCompile(`\S+`).FindAllString(text, -1)) < stubWords || pointerOpener.MatchString(text)) {
			review = append(review, backfillReview{Key: s.Key, Body: s.BodyMD, Stub: text})
			continue
		}
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
