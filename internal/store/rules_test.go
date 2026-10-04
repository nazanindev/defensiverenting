package store_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// rulesFixture is a fresh state with the real registry topics, so the stage
// lists and concept homes are the ones migration 000048 seeded.
func rulesFixture(t *testing.T) (*store.PG, store.Jurisdiction) {
	t.Helper()
	pg := testDB(t)
	slug := "rules-state-" + strings.ToLower(t.Name())
	freshSlugs(t, pg, slug)
	j, err := pg.UpsertJurisdiction(context.Background(), store.UpsertJurisdictionParams{
		Kind: "state", Name: "Rulesland", Slug: slug,
	})
	if err != nil {
		t.Fatal(err)
	}
	return pg, j
}

func topicID(t *testing.T, pg *store.PG, slug string) int64 {
	t.Helper()
	tp, err := pg.GetTopicBySlug(context.Background(), slug)
	if err != nil {
		t.Fatalf("topic %s: %v", slug, err)
	}
	return tp.ID
}

// ingestTagged writes a one-page playbook whose statements carry the given
// concept tags and stages (pairs of concept, stage).
func ingestTagged(t *testing.T, pg *store.PG, jID int64, topic, status, kind string, tags ...[2]string) error {
	t.Helper()
	ctx := context.Background()
	src, err := pg.UpsertSource(ctx, store.UpsertSourceParams{
		URL: "https://example.gov/rules-" + t.Name(), Publisher: "Example", Kind: "statute",
	})
	if err != nil {
		t.Fatal(err)
	}
	var stmts []store.IngestStatementParams
	for i, tg := range tags {
		stmts = append(stmts, store.IngestStatementParams{
			BodyMD: "Rulesland law says thing " + string(rune('A'+i)) + " " + topic + ".", Language: "en",
			ConceptSlug: tg[0], Stage: tg[1],
			Sources: []store.IngestCitationParams{{SourceID: src.ID, Locator: "§ 1", Quote: "verbatim", CheckedNow: true, CheckedBy: "test"}},
		})
	}
	return pg.IngestPlaybook(ctx, store.IngestPlaybookParams{
		JurisdictionID: jID, TopicID: topicID(t, pg, topic), Language: "en", Slug: topic,
		Title: topic + " in Rulesland", IntroMD: "intro", Status: status, PageKind: kind,
		UpdatedBy: "test", Statements: stmts,
	})
}

func pageID(t *testing.T, pg *store.PG, jID int64, topic, status string) int64 {
	t.Helper()
	var id int64
	if err := pg.Pool().QueryRow(context.Background(),
		`SELECT id FROM playbooks WHERE jurisdiction_id=$1 AND topic_id=$2 AND language='en' AND status=$3`,
		jID, topicID(t, pg, topic), status).Scan(&id); err != nil {
		t.Fatalf("find %s %s: %v", topic, status, err)
	}
	return id
}

func TestStage_savedCarriedAndChecked(t *testing.T) {
	pg, j := rulesFixture(t)
	ctx := context.Background()
	if err := ingestTagged(t, pg, j.ID, "security-deposits", "draft", "",
		[2]string{"deposit-return-deadline", "What the law says"},
		[2]string{"", "Ask for your deposit back"}); err != nil {
		t.Fatal(err)
	}
	id := pageID(t, pg, j.ID, "security-deposits", "draft")
	pw, err := pg.AuthorGetPlaybook(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if pw.Statements[0].Stage != "What the law says" || pw.Statements[1].Stage != "Ask for your deposit back" {
		t.Fatalf("stages = %q, %q", pw.Statements[0].Stage, pw.Statements[1].Stage)
	}
	if len(pw.Topic.Stages) != 4 {
		t.Fatalf("topic stages = %v", pw.Topic.Stages)
	}

	// A save that never heard of stages (the authoring form, an approval)
	// keeps them, matched by key.
	var stmts []store.IngestStatementParams
	for _, s := range pw.Statements {
		stmts = append(stmts, store.IngestStatementParams{Key: s.Key, BodyMD: s.BodyMD + " Edited.", Language: "en", ConceptSlug: s.ConceptSlug,
			Sources: []store.IngestCitationParams{{SourceID: s.Citations[0].SourceID, Locator: "§ 1", Quote: "verbatim"}}})
	}
	if err := pg.AuthorUpdatePlaybook(ctx, store.AuthorUpdatePlaybookParams{
		ID: id, JurisdictionID: j.ID, TopicID: pw.Playbook.TopicID, Language: "en", Slug: pw.Playbook.Slug,
		Title: pw.Playbook.Title, IntroMD: pw.Playbook.IntroMD, PageKind: "playbook", UpdatedBy: "Nazanin", Statements: stmts,
	}); err != nil {
		t.Fatal(err)
	}
	pw, _ = pg.AuthorGetPlaybook(ctx, id)
	if pw.Statements[0].Stage != "What the law says" || pw.Statements[1].Stage != "Ask for your deposit back" {
		t.Fatalf("a stage-blind save wiped the stages: %q, %q", pw.Statements[0].Stage, pw.Statements[1].Stage)
	}

	// Nobody invents a heading.
	err = ingestTagged(t, pg, j.ID, "security-deposits", "draft", "", [2]string{"", "Know your rights"})
	if err == nil || !strings.Contains(err.Error(), "not one of this topic's stages") {
		t.Fatalf("an invented stage saved: %v", err)
	}
}

func TestRulesAnswers_lookupByTag(t *testing.T) {
	pg, j := rulesFixture(t)
	ctx := context.Background()
	// The situation page answers the return deadline (published) and the
	// cap (draft only).
	if err := ingestTagged(t, pg, j.ID, "security-deposits", "published", "",
		[2]string{"deposit-return-deadline", ""}); err != nil {
		t.Fatal(err)
	}
	if err := ingestTagged(t, pg, j.ID, "security-deposits", "draft", "",
		[2]string{"deposit-return-deadline", ""}, [2]string{"deposit-cap", ""}); err != nil {
		t.Fatal(err)
	}
	// Searched, no law found.
	if err := pg.FileCoverageRecord(ctx, store.FileCoverageParams{
		JurisdictionSlug: j.Slug, ConceptSlug: "deposit-escrow-interest",
		SourcesChecked: []string{"https://example.gov/code"}, By: store.ActorReviewAgent,
	}); err != nil {
		t.Fatal(err)
	}

	byConcept := func(answers []store.RulesAnswer) map[string]store.RulesAnswer {
		m := map[string]store.RulesAnswer{}
		for _, a := range answers {
			m[a.Concept.Slug] = a
		}
		return m
	}
	public, err := pg.RulesAnswers(ctx, j.ID, "security-deposit-rules", "en", false)
	if err != nil {
		t.Fatal(err)
	}
	if public[0].Concept.Slug != "deposit-cap" || public[0].Concept.Question == "" {
		t.Fatalf("first answer = %+v, want deposit-cap with its question", public[0].Concept)
	}
	m := byConcept(public)
	if a := m["deposit-return-deadline"]; a.Statement == nil || a.Status != "published" || a.TopicSlug != "security-deposits" || len(a.Statement.Citations) == 0 {
		t.Fatalf("return deadline not borrowed from the published situation page: %+v", a)
	}
	if a := m["deposit-cap"]; a.Statement != nil || !a.Gap() {
		t.Fatalf("the public page showed a draft-only answer: %+v", a)
	}
	if a := m["deposit-escrow-interest"]; a.NoLaw == nil || a.Gap() {
		t.Fatalf("coverage record missing: %+v", a)
	}

	// Drafts count when working out gaps: nothing gets drafted twice.
	gaps, err := pg.RulesGaps(ctx, j.Slug)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range gaps {
		if g.RulesTopicSlug != "security-deposit-rules" {
			continue
		}
		for _, c := range g.Gaps() {
			if c.Slug == "deposit-cap" || c.Slug == "deposit-return-deadline" || c.Slug == "deposit-escrow-interest" {
				t.Errorf("%s listed as a gap", c.Slug)
			}
		}
		if len(g.Gaps()) == 0 {
			t.Error("no gaps at all for a page with 3 of 15 concepts")
		}
	}

	// A national concept is never a state's gap.
	for _, g := range gaps {
		for _, c := range g.Gaps() {
			if c.Slug == "subsidized-rent-change" {
				t.Error("a national concept listed as a state gap")
			}
		}
	}
}

func TestCoverageRecord_invariants(t *testing.T) {
	pg, j := rulesFixture(t)
	ctx := context.Background()
	if err := ingestTagged(t, pg, j.ID, "security-deposits", "draft", "", [2]string{"deposit-cap", ""}); err != nil {
		t.Fatal(err)
	}
	err := pg.FileCoverageRecord(ctx, store.FileCoverageParams{
		JurisdictionSlug: j.Slug, ConceptSlug: "deposit-cap", SourcesChecked: []string{"https://example.gov"}, By: "test",
	})
	if !errors.Is(err, store.ErrConceptAnswered) {
		t.Fatalf("a record beside a statement: %v", err)
	}
	if err := pg.FileCoverageRecord(ctx, store.FileCoverageParams{
		JurisdictionSlug: j.Slug, ConceptSlug: "deposit-receipt", By: "test",
	}); err == nil {
		t.Fatal("a record with no place searched")
	}
	if err := pg.FileCoverageRecord(ctx, store.FileCoverageParams{
		JurisdictionSlug: j.Slug, ConceptSlug: "subsidized-rent-change", SourcesChecked: []string{"https://example.gov"}, By: "test",
	}); err == nil {
		t.Fatal("a state record for a national concept")
	}

	title, err := pg.ConceptAnsweredElsewhere(ctx, j.ID, topicID(t, pg, "security-deposit-rules"), "deposit-cap", "en")
	if err != nil || title == "" {
		t.Fatalf("answered elsewhere = %q, %v", title, err)
	}
	if title, _ := pg.ConceptAnsweredElsewhere(ctx, j.ID, topicID(t, pg, "security-deposit-rules"), "deposit-receipt", "en"); title != "" {
		t.Fatalf("deposit-receipt answered on %q", title)
	}
}

// A rules topic is always laid out as rules, and no other topic may use the
// layout (ADR-028 D4), on every save path.
func TestRulesLayout_followsTheTopic(t *testing.T) {
	pg, j := rulesFixture(t)
	if err := ingestTagged(t, pg, j.ID, "security-deposit-rules", "draft", "playbook", [2]string{"deposit-cap", ""}); err != nil {
		t.Fatal(err)
	}
	pw, err := pg.AuthorGetPlaybook(context.Background(), pageID(t, pg, j.ID, "security-deposit-rules", "draft"))
	if err != nil {
		t.Fatal(err)
	}
	if pw.Playbook.PageKind != "rules" {
		t.Fatalf("a rules topic saved as %q", pw.Playbook.PageKind)
	}
	if err := ingestTagged(t, pg, j.ID, "security-deposits", "draft", "rules", [2]string{"deposit-cap", ""}); err == nil {
		t.Fatal("a situation topic saved with the rules layout")
	}
}

// Advice tagged so a checklist links to the rule everywhere is not the
// rule: a statement citing only editorial guidance answers no concept
// question (ADR-028 D1).
func TestEditorialOnly_answersNoConcept(t *testing.T) {
	pg, j := rulesFixture(t)
	ctx := context.Background()
	ed, err := pg.GetEditorialSource(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := pg.IngestPlaybook(ctx, store.IngestPlaybookParams{
		JurisdictionID: j.ID, TopicID: topicID(t, pg, "security-deposits"), Language: "en", Slug: "security-deposits",
		Title: "Deposits in Rulesland", IntroMD: "intro", Status: "published", UpdatedBy: "test",
		Statements: []store.IngestStatementParams{{
			BodyMD: "In Rulesland, ask for a receipt when you pay a deposit.", Language: "en", ConceptSlug: "deposit-receipt",
			Sources: []store.IngestCitationParams{{SourceID: ed.ID}},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	answers, err := pg.RulesAnswers(ctx, j.ID, "security-deposit-rules", "en", true)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range answers {
		if a.Concept.Slug == "deposit-receipt" && !a.Gap() {
			t.Fatal("editorial advice answered the deposit-receipt question")
		}
	}
	if err := pg.FileCoverageRecord(ctx, store.FileCoverageParams{
		JurisdictionSlug: j.Slug, ConceptSlug: "deposit-receipt", SourcesChecked: []string{"https://example.gov"}, By: "test",
	}); err != nil {
		t.Fatalf("editorial advice blocked a no-law record: %v", err)
	}
}

func TestStage_splitFollowersKeepTheLeadsStage(t *testing.T) {
	pg, j := rulesFixture(t)
	ctx := context.Background()
	if err := ingestTagged(t, pg, j.ID, "security-deposits", "draft", "",
		[2]string{"", "What the law says"},
		[2]string{"", "Ask for your deposit back"}); err != nil {
		t.Fatal(err)
	}
	id := pageID(t, pg, j.ID, "security-deposits", "draft")
	pw, err := pg.AuthorGetPlaybook(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	src := pw.Statements[0].Citations[0].SourceID
	pid, err := pg.FileProposal(ctx, store.FileProposalParams{StatementKey: pw.Statements[0].Key, PlaybookID: id, Reason: "agent-pass:triage", ProposedBy: "triage agent",
		Proposed: &store.ProposedStatement{BodyMD: "Rulesland law says the first half.", Citations: []store.ProposedCitation{{URL: "https://example.gov/rules-" + t.Name(), Quote: "verbatim"}}}})
	if err != nil {
		t.Fatal(err)
	}
	cite := []store.IngestCitationParams{{SourceID: src, Locator: "§ 1", Quote: "verbatim"}}
	if err := pg.ApproveProposal(ctx, store.ApproveProposalParams{ID: pid, By: store.ActorReviewAgent,
		Statement: store.IngestStatementParams{BodyMD: "Rulesland law says the first half.", Sources: cite},
		Followers: []store.IngestStatementParams{{BodyMD: "Rulesland law says the second half.", Sources: cite}}}); err != nil {
		t.Fatal(err)
	}
	pw, _ = pg.AuthorGetPlaybook(ctx, id)
	var got []string
	for _, s := range pw.Statements {
		got = append(got, s.Stage)
	}
	if strings.Join(got, "|") != "What the law says|What the law says|Ask for your deposit back" {
		t.Fatalf("stages after split = %q", got)
	}
}
