package store

import (
	"slices"
	"strings"
	"testing"
)

func auditCodes(fs []AuditFinding) map[string]int {
	out := map[string]int{}
	for _, f := range fs {
		out[f.Code]++
	}
	return out
}

func TestPageAuditChecks(t *testing.T) {
	law := []QualityCitation{{SourceKind: "statute", URL: "https://law.example/1", Locator: "§ 1", Quote: "q", Checked: true}}
	in := AuditInput{
		Language: "en", PageKind: "playbook", Place: "Ohio", Title: "Facing Eviction in Ohio: What Can I Do?",
		Stages: []string{"The notice", "Going to court"},
		Statements: []AuditStatement{
			{Key: "k1", Stage: "Going to court", Citations: law,
				Body: "In Ohio, your landlord must give you a written notice. You have 3 days to move out."},
			{Key: "k2", Stage: "The notice", Citations: law,
				Body: "Go to your hearing. The court clerk can tell you the date. See section 1923.04 for more."},
			{Key: "k3", Stage: "", Citations: law,
				Body: "You can hold back your rent. If a court later disagrees, you can owe the rent and face eviction. Get legal help first."},
		},
		Notes: []AuditNote{
			{Slug: "inspector-may-condemn", Kind: "page_note", Warns: "condemn"},
			{Slug: "withhold-risk-evict", Kind: "page_note", Warns: "evict", StatementKey: "k1"},
		},
	}
	got := auditCodes(PageAuditChecks(in))
	for _, code := range []string{
		"place-in-statement",   // "In Ohio,"
		"deadline-shape",       // 3 days, counted from nothing
		"stage-order",          // The notice after Going to court
		"unstaged",             // k3
		"section-number",       // section 1923.04
		"warning-in-statement", // the evict sentence in k3
		"note-orphan",          // nothing needs the condemn note
		"note-misplaced",       // the evict note sits under k1, k3 needs it
	} {
		if got[code] == 0 {
			t.Errorf("want a %s finding, got %v", code, got)
		}
	}
	if got["first-step"] != 0 {
		t.Errorf("k2 tells a step; got first-step: %v", got)
	}
}

func TestPageAuditChecksClean(t *testing.T) {
	law := []QualityCitation{{SourceKind: "statute", URL: "https://law.example/1", Locator: "§ 1", Quote: "q", Checked: true}}
	in := AuditInput{
		Language: "en", PageKind: "playbook", Place: "Ohio", Title: "Facing Eviction in Ohio: What Can I Do?",
		Stages: []string{"The notice"},
		Statements: []AuditStatement{{Key: "k1", Stage: "The notice", Citations: law,
			Body: "You have 3 days after you get the notice to pay the rent. Go to your hearing at the court if your landlord files a case."}},
	}
	for _, f := range PageAuditChecks(in) {
		if !f.Hint {
			t.Errorf("clean page failed %s: %s", f.Code, f.Detail)
		}
	}
}

func TestPageAuditTips(t *testing.T) {
	shape := &TopicShape{
		Advice: []ShapeAdvice{
			{Slug: "answer-court-papers", Kind: "tip", Use: "every_page"},
			{Slug: "inspector-may-condemn", Kind: "page_note", Use: "never"},
		},
		Questions: []ShapeQuestion{
			{Slug: "late-rent-notice", Stage: "The notice"},
			{Slug: "eviction-court-process", Stage: "Going to court"},
			{Slug: "default-judgment", Stage: "Going to court"},
			{Slug: "answer-the-case", Stage: "Going to court"},
		},
	}
	shape.Scope.TitlePattern = "Facing Eviction in {Place}: What Can I Do?"
	shape.Size.MaxStatements, shape.Size.MaxWords, shape.Size.MaxStatementWords, shape.Size.MaxExtras = 12, 650, 75, 4
	shape.Size.Extras = []ShapeQuestion{{Slug: "retaliation-protection", NeverCut: true}}
	in := AuditInput{
		Language: "en", PageKind: "situation", Shape: shape,
		Place: "Ohio", Title: "Facing Eviction in Ohio: What Can I Do?",
		Statements: []AuditStatement{
			{Key: "a", Concept: "late-rent-notice", Body: "Pay all the rent you owe within 7 days after the notice."},
			{Key: "b", Concept: "eviction-court-process", Body: "Go to the hearing. You can file a written answer with the court."},
			{Key: "c", Concept: "retaliation", Body: "A court can order your landlord to pay you 2 months of rent."},
		},
		Notes: []AuditNote{
			{Slug: "keep-a-record", Kind: "tip", StatementKey: "b"},
			{Slug: "ask-for-a-receipt", Kind: "tip", StatementKey: "a"},
		},
	}
	got := map[string]int{}
	for _, f := range PageAuditChecks(in) {
		got[f.Code]++
	}
	for _, code := range []string{"tip-misplaced", "tip-missing", "warning-missing"} {
		if got[code] != 1 {
			t.Errorf("%s: got %d findings, want 1 (all: %v)", code, got[code], got)
		}
	}
	in.Notes = []AuditNote{
		{Slug: "answer-court-papers", Kind: "tip", StatementKey: "b"},
		{Slug: "ask-for-a-receipt", Kind: "tip", StatementKey: "a"},
		{Slug: "award-win-and-pays", Kind: "page_note", Warns: "award"},
	}
	for _, f := range PageAuditChecks(in) {
		if f.Group == AuditWarnings {
			t.Errorf("clean page: unexpected %s: %s", f.Code, f.Detail)
		}
	}
	// Two tips that both fit one statement are fine.
	in.Statements[1].Body = "Go to the hearing. Your landlord may agree to a payment plan there."
	in.Notes = append(in.Notes, AuditNote{Slug: "plan-in-writing", Kind: "tip", StatementKey: "b"})
	for _, f := range PageAuditChecks(in) {
		if f.Group == AuditWarnings {
			t.Errorf("two fitting tips: unexpected %s: %s", f.Code, f.Detail)
		}
	}
	// The shape: a tip the topic never uses, a wrong title, too many extras,
	// a must answer under the wrong stage, and a long statement. A long
	// never-cut defense is a hint.
	long := strings.Repeat("word ", 80)
	sh := in
	sh.Title = "Eviction in Ohio"
	sh.Notes = append(slices.Clone(in.Notes), AuditNote{Slug: "inspector-may-condemn", Kind: "page_note", Warns: "condemn"})
	sh.Statements = append(slices.Clone(in.Statements),
		AuditStatement{Key: "x1", Body: "You can ask for help."}, AuditStatement{Key: "x2", Body: "You can ask for help."},
		AuditStatement{Key: "x3", Body: "You can ask for help."}, AuditStatement{Key: "x4", Body: "You can ask for help."},
		AuditStatement{Key: "x5", Concept: "retaliation-protection", Body: "You can " + long},
		AuditStatement{Key: "x6", Concept: "default-judgment", Stage: "The notice", Body: "You can " + long})
	codes := map[string]int{}
	hints := map[string]int{}
	for _, f := range PageAuditChecks(sh) {
		if f.Hint {
			hints[f.Code]++
		} else {
			codes[f.Code]++
		}
	}
	for _, code := range []string{"advice-not-used", "title-pattern", "over-extras", "question-stage", "statement-over-words"} {
		if codes[code] != 1 {
			t.Errorf("shape: %s: got %d, want 1 (fails %v, hints %v)", code, codes[code], codes, hints)
		}
	}
	if hints["statement-over-words"] != 1 {
		t.Errorf("shape: a long never-cut defense should be a hint (hints %v)", hints)
	}
	if got := len(PageShapeChecks(sh, PartScope)); got != 2 {
		t.Errorf("scope part: got %d findings, want 2 (title pattern, not a question)", got)
	}
	// The court papers tip never sits under the missed-court statement, and
	// sits under the answer statement when the page has one.
	in.Statements = append(in.Statements,
		AuditStatement{Key: "d", Concept: "default-judgment", Body: "If you miss the hearing, the court can rule for your landlord."},
		AuditStatement{Key: "e", Concept: "answer-the-case", Body: "File a written answer within 5 days."})
	for key, want := range map[string]int{"d": 1, "b": 1, "e": 0} {
		in.Notes[0].StatementKey = key
		n := 0
		for _, f := range PageAuditChecks(in) {
			if f.Code == "tip-misplaced" {
				n++
			}
		}
		if n != want {
			t.Errorf("court papers tip under %s: got %d tip-misplaced, want %d", key, n, want)
		}
	}
}
