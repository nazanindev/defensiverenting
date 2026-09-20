package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/nazanindev/defensiverenting/internal/drafting"
	"github.com/nazanindev/defensiverenting/internal/store"
)

// widen turns the narrow statute quotes (triage narrow) into a cmd/propose
// file, no AI involved: for each citation it fetches the source, finds the
// quote, and widens it to the subsection the locator names, from that
// subsection's marker to the next sibling marker or the end of the
// section. A citation it cannot widen with confidence (quote not found,
// marker not found, passage over the cap) is left for an agent and listed
// on stderr with the reason. Every proposal is the whole statement with
// one quote replaced, reason agent-pass:widen-quote, and goes through the
// queue like any edit; the reviewer sees the citation change marked.
//
//	triage widen <narrow.json> > proposals.json
//	triage widen -dry <narrow.json>     fetch and widen, print each result, file nothing
func widen(ctx context.Context, pg *store.PG, tb *drafting.Toolbelt, args []string) {
	dry := len(args) > 0 && args[0] == "-dry"
	if dry {
		args = args[1:]
	}
	if len(args) == 0 {
		usage()
	}
	path := args[0]
	raw, err := os.ReadFile(path) // #nosec G703 -- the file named on the command line is the job
	if err != nil {
		fatal(err)
	}
	var rows []narrowOut
	if err := json.Unmarshal(raw, &rows); err != nil {
		fatal(fmt.Errorf("decode %s: %w", path, err))
	}
	type entry struct {
		StatementKey string                   `json:"statement_key"`
		PlaybookID   int64                    `json:"playbook_id"`
		Reason       string                   `json:"reason"`
		Proposed     *store.ProposedStatement `json:"proposed"`
		Evidence     map[string]any           `json:"evidence"`
	}
	texts := map[string]drafting.Receipt{}
	fetchErr := map[string]error{}
	byKey := map[string]*entry{}
	var order []string
	manual := 0
	for _, r := range rows {
		rc, ok := texts[r.URL]
		if !ok {
			if e, bad := fetchErr[r.URL]; bad {
				err = e
			} else {
				// The toolbelt's tiers: direct, then a headless render, then
				// an archive snapshot. Legislature sites refuse a plain fetch.
				// A passage read from a snapshot is filed unchecked; the
				// approval reads the live page from the authoring server.
				var out drafting.FetchSourceOutput
				out, err = tb.FetchSource(ctx, drafting.FetchSourceInput{URL: r.URL})
				if err != nil {
					fetchErr[r.URL] = err
				} else {
					rc = drafting.Receipt{URL: r.URL, Text: out.Text, Tier: drafting.TierDirect}
					if out.Via != "" {
						rc.Tier = drafting.TierArchive
						if out.Via == "headless render" {
							rc.Tier = drafting.TierRender
						}
					}
					texts[r.URL] = rc
				}
			}
			if err != nil {
				manual++
				fmt.Fprintf(os.Stderr, "manual: page %d #%d %s: fetch failed: %v\n", r.PlaybookID, r.Position+1, r.Locator, err)
				continue
			}
		}
		if lowerLoc := strings.ToLower(r.Locator); strings.Contains(lowerLoc, "history") || strings.Contains(lowerLoc, "s.b.") || strings.Contains(lowerLoc, "h.b.") || strings.Contains(r.URL, "/billtext/") {
			manual++
			fmt.Fprintf(os.Stderr, "manual: page %d #%d %s: a legislative-history citation is not a provision; left as it is\n", r.PlaybookID, r.Position+1, r.Locator)
			continue
		}
		passage, why := widenQuote(rc.Text, r.Quote, r.Locator)
		if why != "" {
			manual++
			fmt.Fprintf(os.Stderr, "manual: page %d #%d %s (%s): %s\n", r.PlaybookID, r.Position+1, r.Locator, r.URL, why)
			continue
		}
		if dry {
			fmt.Printf("page %d #%d %s: %d → %d words\n  %s\n", r.PlaybookID, r.Position+1, r.Locator, r.Words, len(strings.Fields(passage)), passage)
			continue
		}
		e, ok := byKey[r.Key]
		if !ok {
			st, err := pg.StatementByKey(ctx, r.Key)
			if err != nil {
				fatal(fmt.Errorf("statement %s: %w", r.Key, err))
			}
			e = &entry{StatementKey: r.Key, PlaybookID: r.PlaybookID, Reason: "agent-pass:widen-quote", Proposed: &st, Evidence: map[string]any{}}
			byKey[r.Key] = e
			order = append(order, r.Key)
		}
		replaced := false
		for i := range e.Proposed.Citations {
			c := &e.Proposed.Citations[i]
			if c.URL == r.URL && strings.TrimSpace(c.Quote) == strings.TrimSpace(r.Quote) {
				c.Quote = passage
				c.Checked = rc.Live()
				c.CheckedVia = ""
				if c.Checked {
					c.CheckedVia = rc.Describe()
				}
				replaced = true
			}
		}
		if !replaced {
			manual++
			fmt.Fprintf(os.Stderr, "manual: page %d #%d %s: the statement no longer carries this quote\n", r.PlaybookID, r.Position+1, r.Locator)
			continue
		}
		note := fmt.Sprintf("Widened quote: %s, %d to %d words, so a change anywhere in the provision is caught.", r.Locator, r.Words, len(strings.Fields(passage)))
		if prev, _ := e.Evidence["note"].(string); prev != "" {
			note = prev + " " + note
		}
		e.Evidence["note"] = note
		e.Evidence["old_quote"] = r.Quote
		e.Evidence["new_quote"] = passage
		e.Evidence["source_url"] = r.URL
	}
	if dry {
		fmt.Fprintf(os.Stderr, "%d of %d citations widened; %d left for an agent\n", len(rows)-manual, len(rows), manual)
		return
	}
	out := make([]*entry, 0, len(order))
	for _, k := range order {
		out = append(out, byKey[k])
	}
	emit(out)
	fmt.Fprintf(os.Stderr, "%d of %d citations widened into %d proposals; %d left for an agent\n", len(rows)-manual, len(rows), len(out), manual)
}

// widenCap is the longest passage widen offers on its own. Massachusetts
// writes sections of seven hundred words and Washington's landlord duties
// run to two thousand; the point is to watch the whole provision, so the
// cap is only a guard against a marker missed on a page-long run.
const widenCap = 2500

var (
	// subsectionRE matches one parenthesised part of a locator: (a), (3), (ii).
	subsectionRE = regexp.MustCompile(`\(([A-Za-z0-9]+)\)`)
	// sectionNumRE matches the section number in a locator: 59.18.060,
	// 5-12-050, 1950.5, 9-207, 15B, 92.0081, 250.512.
	sectionNumRE = regexp.MustCompile(`[0-9][0-9A-Za-z.\-/]*[0-9A-Za-z]`)
)

// widenQuote returns the passage of text that is the whole subsection the
// locator names and contains the quote, or a reason it could not be found.
// text is the page as fetched; the passage is whitespace-normalized, which
// the verbatim check tolerates.
func widenQuote(text, quote, locator string) (string, string) {
	hay := strings.Join(strings.Fields(text), " ")
	needle := strings.Join(strings.Fields(quote), " ")
	at, n := foldedIndex(hay, needle)
	if at < 0 {
		return "", "quote not found in the page text"
	}
	qEnd := at + n

	parts := subsectionRE.FindAllStringSubmatch(locator, -1)
	if len(parts) == 0 {
		return widenSection(hay, at, qEnd, locator)
	}
	inner := parts[len(parts)-1][1]
	// A range or list, "(1)-(2)" or "(7), (8)": the passage runs from the
	// first part to the end of the last.
	first := inner
	if len(parts) >= 2 && (strings.Contains(locator, ")-(") || strings.Contains(locator, "), (")) {
		first = parts[len(parts)-2][1]
	}
	start := markerBefore(hay, at, first)
	if start < 0 && (first == "a" || first == "1" || first == "i" || first == "A") {
		// The first subsection follows the section heading, which ends in
		// no punctuation on most code sites ("Late Payment of Rent; Fees
		// (a) A landlord…"), so the nearest plain occurrence will do.
		start = markerBeforeLoose(hay, at, first)
	}
	if start < 0 {
		return "", fmt.Sprintf("marker (%s) not found before the quote", first)
	}
	end := len(hay)
	if e := sectionBoundaryAfter(hay, qEnd); e >= 0 && e < end {
		end = e
	}
	for _, m := range nextMarkers(parts, inner) {
		if e := markerAfter(hay, qEnd, end, m); e >= 0 && e < end {
			end = e
		}
	}
	passage := strings.TrimSpace(hay[start:end])
	if w := len(strings.Fields(passage)); w > widenCap {
		return "", fmt.Sprintf("subsection runs %d words, over the %d-word cap", w, widenCap)
	}
	return passage, ""
}

// widenSection handles a locator that names a whole section: the passage
// runs from the section's own heading to the next section heading.
func widenSection(hay string, at, qEnd int, locator string) (string, string) {
	num := sectionNumRE.FindString(locator)
	if num == "" {
		return "", "locator names no section number"
	}
	start := strings.LastIndex(hay[:at], num)
	if start < 0 || at-start > 6000 {
		return "", fmt.Sprintf("section heading %s not found before the quote", num)
	}
	// Back up to the start of the heading's first token, and take a short
	// code prefix before it ("RCW 59.18.070", "§ 15B") as part of the heading.
	for start > 0 && hay[start-1] != ' ' {
		start--
	}
	if start > 0 {
		prev := strings.LastIndex(strings.TrimRight(hay[:start], " "), " ") + 1
		if tok := strings.TrimSpace(hay[prev:start]); tok != "" && len(tok) <= 5 && strings.Trim(tok, "ABCDEFGHIJKLMNOPQRSTUVWXYZ§.") == "" {
			start = prev
		}
	}
	end := len(hay)
	if e := sectionBoundaryAfter(hay, qEnd); e >= 0 {
		end = e
	}
	passage := strings.TrimSpace(hay[start:end])
	if w := len(strings.Fields(passage)); w > widenCap {
		return "", fmt.Sprintf("section runs %d words, over the %d-word cap", w, widenCap)
	}
	return passage, ""
}

// markerForms are the ways a page writes the marker of subsection x: "(x) "
// everywhere, and "x. " on California code pages, which number the
// subdivisions of a section that way. The dotted form is used only when
// the page never writes the bracketed one, since "Sec. 1. " would match it.
func markerForms(hay, x string) []string {
	forms := []string{"(" + x + ") ", "(" + x + ")("}
	if x[0] >= '0' && x[0] <= '9' && !strings.Contains(hay, "("+x+") ") {
		forms = append(forms, x+". ")
	}
	return forms
}

// markerBefore finds the last marker of subsection x at or before pos that
// opens a subsection (opensSubsection). Returns its offset or -1.
func markerBefore(hay string, pos int, x string) int {
	for _, marker := range markerForms(hay, x) {
		// The quote may itself open with the marker.
		if strings.HasPrefix(hay[pos:], marker) {
			return pos
		}
		from := pos
		for i := 0; i < 50; i++ {
			j := strings.LastIndex(hay[:from], marker)
			if j < 0 || pos-j > 8000 {
				break
			}
			if opensSubsection(hay, j) {
				return j
			}
			from = j
		}
	}
	return -1
}

// markerBeforeLoose is markerBefore without the opening-position test,
// for the first subsection of a section. Returns the nearest occurrence
// within reach, or -1.
func markerBeforeLoose(hay string, pos int, x string) int {
	for _, marker := range markerForms(hay, x) {
		if j := strings.LastIndex(hay[:pos], marker); j >= 0 && pos-j <= 8000 {
			return j
		}
	}
	return -1
}

// markerAfter finds the first marker of subsection x after pos and before
// limit, or -1. A marker at a subsection-opening position is preferred;
// failing that, any occurrence counts, because the end marker is a known
// sibling ("(ii)" after "(i)") and Massachusetts runs list items together
// with no punctuation between them.
func markerAfter(hay string, pos, limit int, x string) int {
	loose := -1
	for _, marker := range markerForms(hay, x) {
		from := pos
		for i := 0; i < 50; i++ {
			j := strings.Index(hay[from:limit], marker)
			if j < 0 {
				break
			}
			j += from
			if opensSubsection(hay, j) {
				return j
			}
			if loose < 0 && marker[0] == '(' {
				loose = j
			}
			from = j + len(marker)
		}
	}
	return loose
}

// opensSubsection reports whether the marker at j sits where a subsection
// begins: at the start of the text, or after a sentence end, a colon or
// semicolon, or the "; and" / "; or" that joins list items. "under
// subdivision (b)" mid-sentence does not qualify.
func opensSubsection(hay string, j int) bool {
	tail := strings.TrimRight(hay[:j], " ")
	for _, joiner := range []string{" and", " or"} {
		if strings.HasSuffix(tail, joiner) {
			tail = strings.TrimRight(strings.TrimSuffix(tail, joiner), " ")
			break
		}
	}
	if tail == "" {
		return true
	}
	switch tail[len(tail)-1] {
	case '.', ';', ':', ',', '"', ')':
		return true
	}
	return strings.HasSuffix(tail, "”") || strings.HasSuffix(tail, "’")
}

// nextMarkers lists the markers that end the subsection named by inner:
// its next sibling, and the next sibling of each enclosing level, so a
// passage for (a)(3) ends at (4) or at (b). Texas inserts subsections
// between letters, "(a-1)" after "(a)", so each level's "-1" form counts
// as a sibling too.
func nextMarkers(parts [][]string, inner string) []string {
	var out []string
	for i := len(parts) - 1; i >= 0; i-- {
		if n := nextLabel(parts[i][1]); n != "" {
			out = append(out, n)
		}
		out = append(out, parts[i][1]+"-1")
	}
	_ = inner
	return out
}

// nextLabel is the sibling marker that follows x: 3 → 4, b → c, B → C,
// ii → iii, iv → v. Roman numerals beyond a few are left alone.
func nextLabel(x string) string {
	roman := []string{"i", "ii", "iii", "iv", "v", "vi", "vii", "viii", "ix", "x"}
	for i, r := range roman[:len(roman)-1] {
		if x == r {
			return roman[i+1]
		}
		if x == strings.ToUpper(r) {
			return strings.ToUpper(roman[i+1])
		}
	}
	if n := 0; len(x) > 0 && x[0] >= '0' && x[0] <= '9' {
		if _, err := fmt.Sscanf(x, "%d", &n); err == nil {
			return fmt.Sprintf("%d", n+1)
		}
	}
	if len(x) == 1 && ((x[0] >= 'a' && x[0] < 'z') || (x[0] >= 'A' && x[0] < 'Z')) {
		return string(x[0] + 1)
	}
	return ""
}

// sectionHeadingRE matches the start of a section heading after a sentence
// end: "§ 1950.6", "Sec. 5", "1950.6. (a)" (California), "59.18.070" (RCW),
// "5-12-050" (Chicago), "735 ILCS", "RCW 59".
var sectionHeadingRE = regexp.MustCompile(`(?:^ ?|[.;:] )(?:§+ ?[0-9]|Sec(?:tion|\.) [0-9]|[0-9]{2,}(?:\.[0-9]+)?\. (?:\(|[A-Z])|[0-9]{1,3}\.[0-9]{2}\.[0-9]{3}\b|[0-9]{1,2}-[0-9]{1,3}-[0-9]{3}\b|[0-9]+ ILCS |RCW [0-9])`)

// historyRE matches the legislative history that trails a section on most
// code sites: "Added by Acts 2007", "Acts 2009, 81st Leg.", "(Amended by
// Stats. 2025", "(Source: P.A. 103-224", "Source:", "History:".
var historyRE = regexp.MustCompile(`(?:^ ?|[.;:)] )(?:\(?(?:Added|Amended|Repealed|Renumbered) (?:by |\(as )|Acts [0-9]{4}|\(?Source: |History: |\[Statutory Authority|Credits )`)

// chromeRE matches the page furniture that follows the last provision on a
// code site: footers, share bars, "up to date" stamps.
var chromeRE = regexp.MustCompile(`(?:^| )(?:NYSenate\.gov|Skip to |Share this|Sign up|Follow us|Subscribe|Contact Us|Privacy Policy|Terms of Use|© ?[0-9]{4}|Up to date|Verified:|Stay Connected|Remove ads|Disclaimer:|Print |Email )`)

// sectionBoundaryAfter finds where the next section begins after pos, or
// where the section's legislative history or the page's own furniture
// starts, so a subsection that is the last in its section ends there.
// Returns -1 when none follows.
func sectionBoundaryAfter(hay string, pos int) int {
	loc := sectionHeadingRE.FindStringIndex(hay[pos:])
	for _, re := range []*regexp.Regexp{historyRE, chromeRE} {
		if h := re.FindStringIndex(hay[pos:]); h != nil && (loc == nil || h[0] < loc[0]) {
			loc = h
		}
	}
	if loc == nil {
		return -1
	}
	start := pos + loc[0]
	// Skip the sentence end that anchored the match.
	for start < len(hay) && (hay[start] == '.' || hay[start] == ';' || hay[start] == ':' || hay[start] == ' ') {
		start++
	}
	return start
}

// foldedIndex locates needle in hay with typography folded on both sides
// (drafting.FoldTypography) and maps the span back to hay's own bytes.
func foldedIndex(hay, needle string) (int, int) {
	if at := strings.Index(hay, needle); at >= 0 {
		return at, len(needle)
	}
	needle = drafting.FoldTypography(needle)
	var folded strings.Builder
	origin := make([]int, 0, len(hay)+1)
	for i, r := range hay {
		f := drafting.FoldTypography(string(r))
		for range []byte(f) {
			origin = append(origin, i)
		}
		folded.WriteString(f)
	}
	origin = append(origin, len(hay))
	at := strings.Index(folded.String(), needle)
	if at < 0 {
		return -1, 0
	}
	return origin[at], origin[at+len(needle)] - origin[at]
}
