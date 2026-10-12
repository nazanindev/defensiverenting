package store

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/nazanindev/defensiverenting/internal/voice"
)

// Quality is a set of named checks, each pass or fail, read against a stated
// rule (ADR-027 D1). This file is the rules; quality.go loads the pages they
// run over. Nothing here files anything: the quality map is a report, and
// the review loop stays the only path that changes a page.
//
// Every check is computed. The one ADR-027 check that needs reading
// ("answer first", D4.1) is not here yet.

// Severity orders failing checks from worst to least. The quality map colors
// a cell by the worst check on it (ADR-027 D5): a source failure on a legal
// rule, then an actionability failure, then everything else.
const (
	SeverityRed    = 3 // a legal rule with no law behind it
	SeverityOrange = 2 // the page does not tell the renter what to do
	SeverityYellow = 1 // a weaker source problem, or hard-to-read words
)

// QualityCheck is one failing check on one page.
type QualityCheck struct {
	// Code names the check: below-tier, nonprofit-alone, archive-url,
	// browser-tier-only, no-locator, unchecked, plainness, first-step,
	// deadline-number, somewhere-to-go, risks-paired.
	Code     string
	Severity int
	// Stmt is the 1-based statement the check failed on; 0 for the page.
	Stmt   int
	Detail string
}

// QualityCitation is what the checks need to know about one citation.
type QualityCitation struct {
	SourceKind string
	URL        string
	Locator    string
	Quote      string
	Checked    bool // checked at the source, or attested by hand
	CheckedVia string
}

// QualityStatement is one statement on a page, as the checks see it.
type QualityStatement struct {
	Body      string
	Citations []QualityCitation
}

// QualityInput is one page, as the checks see it.
type QualityInput struct {
	Language   string
	PageKind   string
	Statements []QualityStatement
	// Warns is the risk the page says once at the top ("owe", "evict").
	Warns map[string]bool
}

// Source tiers, strongest first (ADR-027 D3). A court ruling sits one tier
// below law; a nonprofit may back up a claim but never carry a legal rule
// alone (Nazanin, 2026-09-24).
var sourceTiers = map[string]int{
	"statute":      1,
	"regulation":   1,
	"court_ruling": 2,
	"gov_guidance": 3,
	"nonprofit":    4,
	"editorial":    5,
}

// noTier is the tier of a statement with no citation at all: below every
// source there is.
const noTier = 6

// SourceTier returns a source kind's tier, 1 strongest.
func SourceTier(kind string) int {
	if t, ok := sourceTiers[kind]; ok {
		return t
	}
	return noTier
}

// Claim kinds (ADR-027 D3). A statement is judged by its best citation
// against the kind of claim it makes.
const (
	ClaimLegalRule = "legal rule" // a deadline, an amount, a right, what a landlord must or cannot do
	ClaimProcedure = "procedure"  // where to file, which form, who to call
	ClaimPractical = "practical"  // keep records, take photos
)

var (
	// legalMarker is a number with a unit of time, money or share, or a
	// word of obligation. ADR-027 names days, dollars and percent; hours,
	// weeks, months and years are time limits too.
	legalMarker = regexp.MustCompile(`(?i)\b\d+\s+(?:business |calendar |working )?(?:days?|hours?|weeks?|months?|years?)\b|\$\s?\d|\b\d+(?:\.\d+)?\s?(?:percent|%)|\b(?:must|cannot|can't|may not|has to|have to|is required|are required|is not allowed|are not allowed|is illegal|is against the law)\b`)
	// procedureMarker names an agency, a court or a form.
	procedureMarker = regexp.MustCompile(`(?i)\b(?:court|courthouse|clerk|judge|agency|department|office|commission|board|authority|form|file (?:a|an|your)|hotline|helpline|311|211|attorney general|legal aid|inspector|code enforcement|ombudsman)\b`)
)

// ClaimKindOf reads the kind of claim a statement makes from its text.
func ClaimKindOf(body string) string {
	switch {
	case legalMarker.MatchString(body):
		return ClaimLegalRule
	case procedureMarker.MatchString(body):
		return ClaimProcedure
	default:
		return ClaimPractical
	}
}

// tierNeeded is the weakest tier that carries each kind of claim.
func tierNeeded(claim string) int {
	switch claim {
	case ClaimLegalRule:
		return 2
	case ClaimProcedure:
		return 3
	default:
		return noTier
	}
}

// stepVerb opens a sentence that tells the renter to do something. "Do not"
// counts; "You can" does not, it names a right.
var stepVerb = regexp.MustCompile(`^(?:\*\*)?(?:Write|Send|Keep|Ask|Call|Take|Get|Pay|File|Tell|Check|Save|Give|Go|Read|Contact|Look|Make|Put|Use|Bring|Talk|Find|Apply|Report|Document|Sign|Mail|Email|Text|Photograph|Record|Note|Count|Wait|Gather|Collect|Show|Pick|Start|Stay|Leave|Move|Print|Fill|Hand|Deliver|Return|Request|Do not|Don't|Never|Always)\b`)

// A step often follows its condition: "If they do not fix it, tell the
// court." The condition is stripped before the verb is looked for.
var (
	sentenceEnd = regexp.MustCompile(`[.!?]\s+`)
	condition   = regexp.MustCompile(`^(?:If|When|Whenever|Before|After|Once|Unless|Until|While|To [a-z][^,]*|For [a-z][^,]*)\b[^,]*,\s*(?:then\s+)?`)
)

// tellsAStep reports whether any sentence of a statement tells the renter
// to do something.
func tellsAStep(body string) bool {
	for _, sent := range sentenceEnd.Split(body, -1) {
		sent = strings.TrimSpace(sent)
		if stepVerb.MatchString(sent) || stepVerb.MatchString(capitalize(condition.ReplaceAllString(sent, ""))) {
			return true
		}
	}
	return false
}

// capitalize upper-cases the first letter, so "tell" after a condition
// matches the verb list.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// helpPlace is a place to get help: an agency, a court, or legal aid.
var helpPlace = regexp.MustCompile(`(?i)\b(?:legal aid|lawyer|attorney|court|hotline|helpline|311|211|housing authority|attorney general|tenant(?:s|s')? (?:union|rights|council)|code enforcement|inspector|health department|housing department|consumer protection|fair housing|HUD)\b`)

// dayNumber is a number of days in a quote or a statement.
var dayNumber = regexp.MustCompile(`(?i)\b(\d+)\s+(?:\(\d+\)\s+)?(?:business |calendar |working |judicial )?days?\b`)

// PageQualityChecks runs every computed check over one page and returns the
// failures, worst first, then by statement position.
func PageQualityChecks(in QualityInput) []QualityCheck {
	var out []QualityCheck
	add := func(stmt int, code string, sev int, detail string) {
		out = append(out, QualityCheck{Code: code, Severity: sev, Stmt: stmt, Detail: detail})
	}
	daysOnPage := map[string]bool{}
	for _, s := range in.Statements {
		for _, m := range dayNumber.FindAllStringSubmatch(s.Body, -1) {
			daysOnPage[m[1]] = true
		}
	}
	hasStep, hasHelp := false, false

	for i, s := range in.Statements {
		n := i + 1
		body := strings.TrimSpace(s.Body)
		if body == "" {
			continue
		}

		// D3: source strength against the kind of claim, by the best citation.
		claim := ClaimKindOf(body)
		best := noTier
		for _, c := range s.Citations {
			best = min(best, SourceTier(c.SourceKind))
		}
		if need := tierNeeded(claim); best > need {
			switch {
			case claim == ClaimLegalRule && best == noTier:
				add(n, "below-tier", SeverityRed, fmt.Sprintf("statement %d states a legal rule with no citation", n))
			case claim == ClaimLegalRule && best >= sourceTiers["nonprofit"]:
				add(n, "nonprofit-alone", SeverityRed, fmt.Sprintf("statement %d states a legal rule but cites only %s", n, tierWords(best)))
			case claim == ClaimLegalRule:
				add(n, "below-tier", SeverityRed, fmt.Sprintf("statement %d states a legal rule but cites only %s, not the law itself", n, tierWords(best)))
			case best == noTier:
				add(n, "below-tier", SeverityYellow, fmt.Sprintf("statement %d describes a procedure with no citation", n))
			default:
				add(n, "below-tier", SeverityYellow, fmt.Sprintf("statement %d describes a procedure but cites only %s", n, tierWords(best)))
			}
		}

		// D3 signals on the citations themselves, once per statement: two
		// citations with the same problem are one thing to fix.
		signals := map[string]string{}
		for _, c := range s.Citations {
			switch {
			case strings.Contains(c.URL, "web.archive.org/"):
				signals["archive-url"] = fmt.Sprintf("statement %d cites an archive snapshot", n)
			case c.CheckedVia == "render":
				signals["browser-tier-only"] = fmt.Sprintf("statement %d cites a source only a browser can read", n)
			}
			if c.SourceKind == "statute" && !looksLikeSection(c.Locator) {
				signals["no-locator"] = fmt.Sprintf("statement %d cites a statute without naming the section", n)
			}
			if c.SourceKind != "editorial" && (strings.TrimSpace(c.Quote) == "" || !c.Checked) {
				signals["unchecked"] = fmt.Sprintf("statement %d has a quote nobody has checked at the source", n)
			}
		}
		for _, code := range []string{"archive-url", "browser-tier-only", "no-locator", "unchecked"} {
			if d, ok := signals[code]; ok {
				add(n, code, SeverityYellow, d)
			}
		}

		// D2: plainness, the ab48c10 lint as one line.
		if in.Language == "en" {
			if words := len(strings.Fields(body)); words > voice.MaxStatementWords {
				add(n, "plainness", SeverityYellow, fmt.Sprintf("statement %d runs %d words", n, words))
			} else if uw := voice.UnfamiliarWords(body); len(uw) > 0 {
				add(n, "plainness", SeverityYellow, fmt.Sprintf("statement %d uses words a renter may not know: %s", n, strings.Join(uw, ", ")))
			} else if sc := voice.ReadingScore(body); sc >= voice.MaxStatementScore {
				add(n, "plainness", SeverityYellow, fmt.Sprintf("statement %d reads too hard (%.1f)", n, sc))
			}
		}

		// D4: actionability, on situation pages only. A rules page holds
		// facts, a directory lists places, a checklist is its own steps.
		if in.PageKind != "playbook" {
			continue
		}
		if tellsAStep(body) {
			hasStep = true
		}
		if helpPlace.MatchString(body) {
			hasHelp = true
		}
		if w := voice.NeedsWarning(in.Language, body); w != "" && !in.Warns[w] {
			add(n, "risks-paired", SeverityOrange, fmt.Sprintf("statement %d names a risky step without saying the risk", n))
		}
		for _, c := range s.Citations {
			for _, m := range dayNumber.FindAllStringSubmatch(c.Quote, -1) {
				if !daysOnPage[m[1]] {
					add(n, "deadline-number", SeverityOrange, fmt.Sprintf("statement %d cites a %s-day limit the page never states", n, m[1]))
					break
				}
			}
		}
	}

	if in.PageKind == "playbook" && len(in.Statements) > 0 {
		if !hasStep {
			add(0, "first-step", SeverityOrange, "no statement tells the renter to do something")
		}
		if !hasHelp {
			add(0, "somewhere-to-go", SeverityOrange, "the page names nowhere to get help")
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return out[i].Severity > out[j].Severity
		}
		return out[i].Stmt < out[j].Stmt
	})
	return out
}

// tierWords names a tier in the words the quality map uses.
func tierWords(tier int) string {
	switch tier {
	case 1:
		return "the law"
	case 2:
		return "a court ruling"
	case 3:
		return "government guidance"
	case 4:
		return "a nonprofit page"
	case 5:
		return "editorial guidance"
	default:
		return "nothing"
	}
}

// CheckNames describes each check in plain words, for the list under the map.
var CheckNames = map[string]string{
	"below-tier":        "A legal rule or procedure cites no source strong enough for it",
	"nonprofit-alone":   "A legal rule cites only a nonprofit or editorial source",
	"archive-url":       "A citation points at an archive snapshot",
	"browser-tier-only": "A source only a browser can read",
	"no-locator":        "A statute citation with no section",
	"unchecked":         "A quote nobody has checked at the source",
	"plainness":         "A statement that reads too hard",
	"first-step":        "No statement tells the renter to do something",
	"deadline-number":   "A cited deadline the page never states in days",
	"somewhere-to-go":   "The page names nowhere to get help",
	"risks-paired":      "A risky step without its warning",
}
