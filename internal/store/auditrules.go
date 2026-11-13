package store

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/nazanindev/defensiverenting/internal/voice"
)

// The topic audit (ADR-035 D2, D3) is one grid: a row per place, a column
// per group of checks. This file is the computed checks on one page; audit.go
// loads the pages. Like the quality map it files nothing: it reads what the
// lint, the quality checks and the publish gate already compute, and adds
// the checks no one of them makes (a warning inside a statement, a note with
// no step under it, stage order, a place name in a statement).

// Audit groups, in the order the grid shows them.
const (
	AuditCoverage   = "coverage"
	AuditWarnings   = "warnings"
	AuditActionable = "actionable"
	AuditSources    = "sources"
	AuditShape      = "shape"
	AuditVoice      = "voice"
	AuditFinished   = "finished"
)

// AuditGroups is the grid's columns.
var AuditGroups = []string{AuditCoverage, AuditWarnings, AuditActionable, AuditSources, AuditShape, AuditVoice, AuditFinished}

// AuditFinding is one failing check on one page.
type AuditFinding struct {
	Group string `json:"group"`
	Code  string `json:"code"`
	// Stmt is the 1-based statement; 0 for the page.
	Stmt   int    `json:"stmt,omitempty"`
	Key    string `json:"key,omitempty"`
	Detail string `json:"detail"`
	// Part is the part of the topic shape the finding belongs to (ADR-036
	// D4): scope, tips or size. Empty for a check the shape does not own.
	Part string `json:"part,omitempty"`
	// Hint marks a finding code cannot settle: the reader decides, and the
	// grid shows it apart from failures.
	Hint bool `json:"hint,omitempty"`
}

// AuditStatement is one statement as the audit sees it.
type AuditStatement struct {
	Key       string
	Body      string
	Stage     string
	Concept   string // the concept slug the statement is tagged with, "" for none
	Citations []QualityCitation
}

// AuditNote is a page note or tip on the page.
type AuditNote struct {
	Slug         string
	Kind         string // page_note or tip
	Warns        string
	StatementKey string
}

// AuditInput is one page as the audit sees it.
type AuditInput struct {
	Language   string
	PageKind   string
	Place      string // the place's name, "" for the country
	City       bool   // a city page: the shape may allow it more statements
	Title      string
	Stages     []string // the topic's stage list, in order
	Plain      bool
	Statements []AuditStatement
	Notes      []AuditNote
	// Core is the topic's must-answer questions, in order (topic_questions,
	// role must). NoLaw holds the concepts the place has a "searched, no law
	// found" record for.
	Core  []AuditConcept
	NoLaw map[string]bool
	// Shape is the topic's stored shape (ADR-036 D4): title pattern, caps,
	// extras and tip use. Nil when the caller has none; then the shape
	// checks are silent.
	Shape *TopicShape
}

// The parts of a topic shape a finding can belong to.
const (
	PartScope = "scope"
	PartTips  = "tips"
	PartSize  = "size"
)

// shapePartOf sorts the audit's codes into the parts of the topic shape.
var shapePartOf = map[string]string{
	"title-pattern":        PartScope,
	"title-not-question":   PartScope,
	"tip-missing":          PartTips,
	"tip-misplaced":        PartTips,
	"advice-not-used":      PartTips,
	"warning-missing":      PartTips,
	"warning-in-statement": PartTips,
	"note-orphan":          PartTips,
	"note-misplaced":       PartTips,
	"over-statements":      PartSize,
	"over-words":           PartSize,
	"statement-over-words": PartSize,
	"over-extras":          PartSize,
	"question-stage":       PartSize,
	"stage-order":          PartSize,
	"unstaged":             PartSize,
	"long-page":            PartSize,
}

// PageShapeChecks is PageAuditChecks narrowed to one part of the topic
// shape (PartScope, PartTips or PartSize): what the Quality tab shows as a
// column beside must-answer.
func PageShapeChecks(in AuditInput, part string) []AuditFinding {
	var out []AuditFinding
	for _, f := range PageAuditChecks(in) {
		if f.Part == part {
			out = append(out, f)
		}
	}
	return out
}

// AuditConcept is one core question of a topic.
type AuditConcept struct {
	Slug     string `json:"concept"`
	Question string `json:"question"`
}

// Gaps returns the core questions the page neither answers nor has a
// no-law record for: what the research step is handed.
func (in AuditInput) Gaps() []AuditConcept {
	answered := map[string]bool{}
	for _, s := range in.Statements {
		if s.Concept != "" {
			answered[s.Concept] = true
		}
	}
	var out []AuditConcept
	for _, c := range in.Core {
		if !answered[c.Slug] && !in.NoLaw[c.Slug] {
			out = append(out, c)
		}
	}
	return out
}

// rightMarker is a statement that gives the renter a right, a limit on the
// landlord, or a consequence, in the words the guides use.
var rightMarker = regexp.MustCompile(`(?i)\byou (can|may|could|do not have to|don't have to|have (a|the) right|are entitled|get|keep|owe|lose|still owe)\b|\b(your|the|a) landlord (can|cannot|can't|must|may|has to|is not allowed)\b|\b(a|the) (court|judge) (can|may|must|will)\b|\b(illegal|against the law|not allowed)\b`)

// auditGroupOf sorts the quality-check codes into the audit's groups.
var auditGroupOf = map[string]string{
	"below-tier":        AuditSources,
	"nonprofit-alone":   AuditSources,
	"archive-url":       AuditSources,
	"browser-tier-only": AuditSources,
	"no-locator":        AuditSources,
	"unchecked":         AuditSources,
	"plainness":         AuditVoice,
	"first-step":        AuditActionable,
	"deadline-number":   AuditActionable,
	"somewhere-to-go":   AuditActionable,
	"risks-paired":      AuditWarnings,
}

// auditHints are quality checks the audit shows as hints: ADR-027 calls the
// deadline check "a computed hint, read to confirm", and a browser-only
// source is not a fault of the page.
var auditHints = map[string]bool{"deadline-number": true, "browser-tier-only": true}

// warnWords names each risk in the words a finding uses.
var warnWords = map[string]string{
	"owe":     "you can still owe the rent",
	"evict":   "you can owe the rent and face eviction",
	"award":   "you get the money only if you win and the landlord pays",
	"condemn": "an inspector can condemn the home",
}

// PageAuditChecks runs the computed audit checks over one page.
func PageAuditChecks(in AuditInput) []AuditFinding {
	var out []AuditFinding
	add := func(group, code string, stmt int, hint bool, detail string) {
		key := ""
		if stmt > 0 {
			key = in.Statements[stmt-1].Key
		}
		out = append(out, AuditFinding{Group: group, Code: code, Part: shapePartOf[code], Stmt: stmt, Key: key, Detail: detail, Hint: hint})
	}

	// Coverage: every core question answered, or recorded as no law found.
	for _, c := range in.Gaps() {
		add(AuditCoverage, "gap", 0, false, fmt.Sprintf("no statement answers %q (%s), and no record says the law was searched", c.Question, c.Slug))
	}

	page := voice.Page{Warns: map[string]bool{}, Tips: map[string]bool{}, Plain: in.Plain}
	for _, n := range in.Notes {
		if n.Kind == "tip" {
			page.Tips[n.Slug] = true
		} else if n.Warns != "" {
			page.Warns[n.Warns] = true
		}
	}

	// The quality checks, sorted into groups. risks-paired is renamed: the
	// audit says which warning is missing.
	q := QualityInput{Language: in.Language, PageKind: in.PageKind, Warns: page.Warns}
	for _, s := range in.Statements {
		q.Statements = append(q.Statements, QualityStatement{Body: s.Body, Citations: s.Citations})
	}
	for _, c := range PageQualityChecks(q) {
		if c.Code == "risks-paired" {
			continue // said below, with the kind of warning
		}
		add(auditGroupOf[c.Code], c.Code, c.Stmt, auditHints[c.Code], c.Detail)
	}

	// Warnings. needs[i] is the risk statement i relies on the page to say.
	needs := make([]string, len(in.Statements))
	firstNeeding := map[string]int{} // risk -> 0-based statement
	for i, s := range in.Statements {
		w := voice.NeedsWarning(in.Language, s.Body)
		needs[i] = w
		if w != "" {
			if _, ok := firstNeeding[w]; !ok {
				firstNeeding[w] = i
			}
			if !page.Warns[w] {
				add(AuditWarnings, "warning-missing", i+1, false,
					fmt.Sprintf("statement %d names a step whose risk the page never says (%s): attach the note", i+1, warnWords[w]))
			}
		}
		// The lint's award words miss "can order your landlord to pay" and
		// "owes you": money a court makes the landlord pay is an award too.
		if w == "" && !page.Warns["award"] && awardOwedRe.MatchString(s.Body) &&
			!slices.Contains(voice.SaidOnceKinds(in.Language, s.Body), "award") {
			add(AuditWarnings, "warning-missing", i+1, false,
				fmt.Sprintf("statement %d says the landlord can owe the renter money and the page never says it is not automatic: attach the award note", i+1))
		}
		// A12: the rent warnings are notes, never sentences.
		for _, k := range voice.SaidOnceKinds(in.Language, s.Body) {
			if k == "owe" || k == "evict" || page.Warns[k] {
				add(AuditWarnings, "warning-in-statement", i+1, false,
					fmt.Sprintf("statement %d says the %q warning as a sentence: it is a page note", i+1, k))
			}
		}
	}
	keyPos := map[string]int{}
	for i, s := range in.Statements {
		keyPos[s.Key] = i
	}
	for _, n := range in.Notes {
		if n.Kind == "tip" || n.Warns == "" {
			continue
		}
		first, needed := firstNeeding[n.Warns]
		// A statement that says the risk itself still counts as the note's
		// reason: the fix there is to strip the sentence, not the note.
		if !needed {
			for i, s := range in.Statements {
				if slices.Contains(voice.SaidOnceKinds(in.Language, s.Body), n.Warns) ||
					(n.Warns == "award" && awardOwedRe.MatchString(s.Body)) {
					first, needed = i, true
					break
				}
			}
		}
		switch {
		case !needed:
			add(AuditWarnings, "note-orphan", 0, false,
				fmt.Sprintf("the page carries the note %q but no statement names a step that needs it", n.Slug))
		case n.StatementKey == "":
			// Said at the top: allowed (A11), nothing to check.
		default:
			at, on := keyPos[n.StatementKey]
			if !on {
				add(AuditWarnings, "note-misplaced", 0, false,
					fmt.Sprintf("the note %q sits under a statement that left the page: move it under statement %d", n.Slug, first+1))
			} else if at != first {
				add(AuditWarnings, "note-misplaced", at+1, false,
					fmt.Sprintf("the note %q sits under statement %d; the first statement that needs it is %d", n.Slug, at+1, first+1))
			}
		}
	}

	// Tips (D8). A tip sits under a statement that names its step, and the
	// court papers tip is on every page that tells the renter to answer or
	// appear. A statement may carry more than one tip when each fits it.
	hasAnswer := slices.ContainsFunc(in.Statements, func(s AuditStatement) bool { return s.Concept == "answer-the-case" })
	for _, n := range in.Notes {
		if n.Kind != "tip" {
			continue
		}
		at, on := keyPos[n.StatementKey]
		if !on {
			continue // tip-orphan is a gate issue already
		}
		rule, ok := tipRules[n.Slug]
		if !ok {
			continue
		}
		st := in.Statements[at]
		// A tip fits when one sentence names its step and that sentence is
		// not the excluded kind (paying into court, a court filing).
		fits := slices.Contains(rule.concepts, st.Concept)
		if !fits && rule.under != nil {
			for _, sent := range strings.SplitAfter(st.Body, ". ") {
				if rule.under.MatchString(sent) && (rule.not == nil || !rule.not.MatchString(sent)) {
					fits = true
					break
				}
			}
		}
		if n.Slug == "answer-court-papers" && st.Concept == "default-judgment" {
			add(AuditWarnings, "tip-misplaced", at+1, false,
				fmt.Sprintf("the court papers tip sits under statement %d, which says what happens if the renter misses court: that statement already says it, move the tip to the statement on answering court papers", at+1))
			continue
		}
		if n.Slug == "answer-court-papers" && hasAnswer && st.Concept != "answer-the-case" {
			add(AuditWarnings, "tip-misplaced", at+1, false,
				fmt.Sprintf("the court papers tip sits under statement %d, and the page has a statement on answering court papers: move the tip there", at+1))
			continue
		}
		if !fits {
			add(AuditWarnings, "tip-misplaced", at+1, false,
				fmt.Sprintf("the tip %q sits under statement %d, which does not name %s", n.Slug, at+1, rule.what))
		}
	}
	// Which tips and notes the topic uses comes from its shape: one every
	// page carries, one no page carries.
	if in.Shape != nil {
		onPage := map[string]bool{}
		for _, n := range in.Notes {
			onPage[n.Slug] = true
		}
		for _, a := range in.Shape.Advice {
			switch {
			case a.Use == "every_page" && !onPage[a.Slug]:
				at := 0
				for i, s := range in.Statements {
					if slices.Contains(tipRules[a.Slug].concepts, s.Concept) {
						at = i + 1
						break
					}
				}
				detail := fmt.Sprintf("every page of this topic carries %q and this page does not", a.Slug)
				if at > 0 {
					detail += fmt.Sprintf(": attach it under statement %d", at)
				}
				add(AuditWarnings, "tip-missing", at, false, detail)
			case a.Use == "never" && onPage[a.Slug]:
				add(AuditWarnings, "advice-not-used", 0, false,
					fmt.Sprintf("the page carries %q, which this topic never uses: remove it", a.Slug))
			}
		}
	}

	// Actionable and shape, per statement.
	stageAt := func(name string) int { return slices.Index(in.Stages, name) }
	lastStage := -1
	var placeRe *regexp.Regexp
	if in.Place != "" {
		p := regexp.QuoteMeta(in.Place)
		placeRe = regexp.MustCompile(`\bIn ` + p + `,|\bUnder ` + p + ` law\b|\b` + p + ` law (says|gives|lets|requires|does)\b`)
	}
	words := 0
	for i, s := range in.Statements {
		n := i + 1
		body := strings.TrimSpace(s.Body)
		words += len(strings.Fields(body))
		if voice.BareDeadline(body) {
			add(AuditActionable, "deadline-shape", n, false,
				fmt.Sprintf("statement %d gives a number of days without saying what they count from", n))
		}
		if in.PageKind == "playbook" && !tellsAStep(body) && !legalMarker.MatchString(body) &&
			!helpPlace.MatchString(body) && !rightMarker.MatchString(body) {
			add(AuditActionable, "renter-use", n, true,
				fmt.Sprintf("statement %d gives no step, right, limit, number or place to get help: read it for renter use", n))
		}
		if len(in.Stages) > 0 && in.PageKind == "playbook" {
			switch at := stageAt(s.Stage); {
			case s.Stage == "":
				add(AuditShape, "unstaged", n, false, fmt.Sprintf("statement %d has no stage", n))
			case at < 0:
				add(AuditShape, "unstaged", n, false, fmt.Sprintf("statement %d has the stage %q, which the topic does not list", n, s.Stage))
			case at < lastStage:
				add(AuditShape, "stage-order", n, false,
					fmt.Sprintf("statement %d (%s) comes after a later stage (%s)", n, s.Stage, in.Stages[lastStage]))
			default:
				lastStage = at
			}
		}
		if placeRe != nil {
			if m := placeRe.FindString(body); m != "" {
				add(AuditShape, "place-in-statement", n, false, fmt.Sprintf("statement %d names the place (%q): the page already says where", n, m))
			}
		}
		if m := voice.SectionRef(body); m != "" {
			add(AuditShape, "section-number", n, false, fmt.Sprintf("statement %d names a code section (%q): the source line names it", n, m))
		}
		for _, v := range voice.LintOn(in.Language, map[string]string{"body_md": body}, page) {
			// The risk rules are said once, under warnings.
			if strings.Contains(v, "a court judges only afterwards") || strings.Contains(v, "tells the renter they owe nothing") {
				continue
			}
			add(AuditVoice, "voice", n, false, fmt.Sprintf("statement %d: %s", n, strings.TrimPrefix(v, "body_md: ")))
		}
	}
	if !strings.HasSuffix(strings.TrimSpace(in.Title), "?") {
		add(AuditShape, "title-not-question", 0, false, fmt.Sprintf("the title is not a question: %q", in.Title))
	}
	if sh := in.Shape; sh != nil {
		if sh.Scope.TitlePattern != "" {
			if want := sh.Title(in.Place); strings.TrimSpace(in.Title) != want {
				add(AuditShape, "title-pattern", 0, false, fmt.Sprintf("the title is %q; the topic's title is %q", in.Title, want))
			}
		}
		// Caps. A statement on a never-cut extra (a defense with its
		// exceptions) may run long: that is a hint, read against the
		// topic's order rules.
		must := map[string]string{} // concept -> stage
		for _, q := range sh.Questions {
			must[q.Slug] = q.Stage
		}
		neverCut := map[string]bool{}
		for _, q := range sh.Size.Extras {
			neverCut[q.Slug] = q.NeverCut
		}
		maxStatements, maxExtras := sh.Size.MaxStatements, sh.Size.MaxExtras
		if in.City {
			maxStatements += sh.Size.CityMore
			maxExtras += sh.Size.CityMore
		}
		if sh.Size.MaxStatements > 0 && len(in.Statements) > maxStatements {
			add(AuditShape, "over-statements", 0, false,
				fmt.Sprintf("the page has %d statements; the topic allows %d", len(in.Statements), maxStatements))
		}
		// A must question may have a second statement under a later stage
		// (the judgment, after the hearing). It fails only when none of its
		// statements sits under the question's own stage.
		inStage := map[string]bool{}
		for _, s := range in.Statements {
			if must[s.Concept] == s.Stage {
				inStage[s.Concept] = true
			}
		}
		longDefense := false
		extras := 0
		for i, s := range in.Statements {
			n := len(strings.Fields(s.Body))
			if sh.Size.MaxStatementWords > 0 && n > sh.Size.MaxStatementWords {
				longDefense = longDefense || neverCut[s.Concept]
				add(AuditShape, "statement-over-words", i+1, neverCut[s.Concept],
					fmt.Sprintf("statement %d runs %d words; the topic allows %d", i+1, n, sh.Size.MaxStatementWords))
			}
			stage, isMust := must[s.Concept]
			if !isMust {
				extras++
			} else if s.Stage != "" && s.Stage != stage && !inStage[s.Concept] {
				add(AuditShape, "question-stage", i+1, false,
					fmt.Sprintf("statement %d answers a %q question and sits under %q", i+1, stage, s.Stage))
			}
		}
		if sh.Size.MaxWords > 0 && words > sh.Size.MaxWords {
			// A page with a long never-cut defense may bend the cap: a hint.
			add(AuditShape, "over-words", 0, longDefense,
				fmt.Sprintf("the page runs %d words; the topic allows %d", words, sh.Size.MaxWords))
		}
		if len(sh.Questions) > 0 && extras > maxExtras {
			add(AuditShape, "over-extras", 0, false,
				fmt.Sprintf("the page has %d statements beyond the must-answer questions; the topic allows %d", extras, maxExtras))
		}
	}
	// The caps gate plain-layout topics already (finished group). On other
	// topics they are a hint until the topic's spec sets its own.
	if in.Shape == nil && !in.Plain && in.PageKind == "playbook" {
		if len(in.Statements) > PlainMaxStatements {
			add(AuditShape, "long-page", 0, true, fmt.Sprintf("the page has %d statements (the plain layout allows %d)", len(in.Statements), PlainMaxStatements))
		} else if words > PlainMaxPageWords {
			add(AuditShape, "long-page", 0, true, fmt.Sprintf("the page runs %d words (the plain layout allows %d)", words, PlainMaxPageWords))
		}
	}
	return out
}

// awardOwedRe is money a court makes the landlord pay the renter, in the
// words the lint's award rule does not know.
var awardOwedRe = regexp.MustCompile(`(?i)\b(court|judge) can (order|make) your landlord (to )?pay|\borders your landlord to pay\b|\bmust pay you\b|\bcan owe you\b|\bowes you (your|up to|\d)|\byou can (also )?(recover|ask for \d+ times)\b|\bmoney you may recover\b|\bcan be taken to court for the harm\b`)

// tipRule says which statement a tip may sit under (ADR-035 D8): one tagged
// with one of the concepts, or one whose words match under and not not.
type tipRule struct {
	concepts []string
	under    *regexp.Regexp
	not      *regexp.Regexp
	what     string
}

var tipRules = map[string]tipRule{
	"answer-court-papers": {
		concepts: []string{"answer-the-case", "eviction-court-process"},
		what:     "answering court papers or going to court",
	},
	"ask-for-a-receipt": {
		under: regexp.MustCompile(`(?i)\bpay(s|ing|ment)?\b|\bpaid\b|\boffer`),
		not:   regexp.MustCompile(`(?i)into (the )?court|court registry|to the (court )?clerk`),
		what:  "paying the landlord",
	},
	"plan-in-writing": {
		under: regexp.MustCompile(`(?i)\bagree|\baccept|payment plan|\bsettle|\bmediat`),
		what:  "an agreement with the landlord",
	},
	"keep-a-record": {
		under: regexp.MustCompile(`(?i)(complain|ask|tell|write|notice|letter|request)[^.]*\blandlord\b|\blandlord\b[^.]*(in writing|written notice)|\b(ask(ed|ing)?|request)\b[^.]*\brepairs?\b|\b(written|repair) (notice|request)\b`),
		not:   regexp.MustCompile(`(?i)\b(file|filed|filing)\b[^.]*\b(court|answer|objection)`),
		what:  "writing or complaining to the landlord",
	},
}
