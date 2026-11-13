package store_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// Eviction's shape holds all four parts (migration 000093), and a topic
// nobody has filled says so.
func TestTopicShapes(t *testing.T) {
	pg := testDB(t)
	ctx := context.Background()
	shapes, err := pg.TopicShapes(ctx, "")
	if err != nil {
		t.Fatalf("TopicShapes: %v", err)
	}
	for _, s := range shapes {
		for _, q := range s.Questions {
			if !slices.Contains(s.Stages, q.Stage) {
				t.Errorf("%s: %s sits under %q, not a stage of the topic", s.Slug, q.Slug, q.Stage)
			}
		}
	}

	one, err := pg.TopicShapes(ctx, "eviction-defense")
	if err != nil || len(one) != 1 {
		t.Fatalf("eviction-defense: %v, %d shapes", err, len(one))
	}
	ev := one[0]
	if m := ev.Missing(); len(m) != 0 {
		t.Errorf("eviction is missing %v", m)
	}
	if n := len(ev.Questions); n != 8 {
		t.Errorf("eviction has %d must-answer questions, want 8", n)
	}
	if n := len(ev.Size.Extras); n != 4 {
		t.Errorf("eviction has %d extras, want 4", n)
	}
	if n := len(ev.Advice); n != 6 {
		t.Errorf("eviction uses %d tips and notes, want 6", n)
	}
	for _, q := range ev.Questions {
		switch q.Slug {
		case "pay-and-stay":
			if q.Answer != "days_where_set" {
				t.Errorf("pay-and-stay is answered by %q, want days_where_set", q.Answer)
			}
		case "notice-to-quit":
			if len(q.Situations) != 3 || !q.Situations[0].Base {
				t.Errorf("notice-to-quit situations: %+v", q.Situations)
			}
		}
	}

	bl, err := pg.TopicShapes(ctx, "breaking-lease")
	if err != nil || len(bl) != 1 {
		t.Fatalf("breaking-lease: %v", err)
	}
	if m := bl[0].Missing(); len(m) != 0 {
		t.Errorf("breaking a lease is missing %v", m)
	}
	if n := len(bl[0].Advice); n != 11 {
		t.Errorf("breaking a lease uses %d tips and notes, want 11", n)
	}
	if bl[0].Scope.IntroOpens == "" {
		t.Error("breaking a lease has no intro rule")
	}

	other, err := pg.TopicShapes(ctx, "landlord-entry")
	if err != nil || len(other) != 1 {
		t.Fatalf("landlord-entry: %v", err)
	}
	if n := len(other[0].Missing()); n != 4 {
		t.Errorf("an unfilled topic is missing %d parts, want 4", n)
	}
	if _, err := pg.TopicShapes(ctx, "no-such-topic"); err == nil {
		t.Error("want an error for an unknown topic")
	}
}

func TestTopicShapeTitle(t *testing.T) {
	s := store.TopicShape{Scope: store.ShapeScope{TitlePattern: "Facing Eviction in {Place}: What Can I Do?"}}
	if got := s.Title("Ohio"); got != "Facing Eviction in Ohio: What Can I Do?" {
		t.Errorf("state title: %q", got)
	}
	if got := s.Title(""); got != "Facing Eviction: What Can I Do?" {
		t.Errorf("nationwide title: %q", got)
	}
}

// A days question answered only in words is a hole; a days_where_set
// question answered in words is not.
func TestTableCellGap(t *testing.T) {
	days := store.ShapeQuestion{Answer: "days"}
	whereSet := store.ShapeQuestion{Answer: "days_where_set"}
	cases := []struct {
		state string
		q     store.ShapeQuestion
		want  bool
	}{
		{store.CellNumber, days, false},
		{store.CellWords, days, true},
		{store.CellWords, whereSet, false},
		{store.CellNoLaw, days, false},
		{store.CellNotChecked, whereSet, true},
	}
	for _, c := range cases {
		if got := (store.TableCell{State: c.state}).Gap(c.q); got != c.want {
			t.Errorf("%s on a %s question: gap %v, want %v", c.state, c.q.Answer, got, c.want)
		}
	}
}

// The table reads the shape: eviction has six columns, and a condition
// outside a question's situations is refused.
func TestTopicTableAndSituations(t *testing.T) {
	pg, j, src := ruleValueFixture(t)
	ctx := context.Background()
	tb, err := pg.TopicTable(ctx, "eviction-defense")
	if err != nil {
		t.Fatalf("TopicTable: %v", err)
	}
	if n := len(tb.Questions); n != 6 {
		t.Errorf("eviction table has %d columns, want 6", n)
	}
	for _, r := range tb.Rows {
		if len(r.Cells) != len(tb.Questions) {
			t.Fatalf("%s has %d cells for %d questions", r.Place, len(r.Cells), len(tb.Questions))
		}
	}
	bl, err := pg.TopicTable(ctx, "breaking-lease")
	if err != nil || len(bl.Questions) != 1 {
		t.Errorf("breaking-lease table: %v, %d columns, want 1", err, len(bl.Questions))
	}
	if _, err := pg.TopicTable(ctx, "landlord-entry"); err == nil {
		t.Error("want an error for a topic with no shape")
	}

	err = pg.FileRuleValue(ctx, store.FileRuleValueParams{
		JurisdictionSlug: j.Slug, ConceptSlug: "notice-to-quit", Condition: "a situation nobody listed",
		Kind: "days", Value: 3, SourceURL: src.URL, Quote: "three days", By: "test",
	})
	if err == nil || !strings.Contains(err.Error(), "situations") {
		t.Errorf("a condition outside the list: got %v, want a situations error", err)
	}
	t.Cleanup(func() { _, _ = pg.RemoveRuleValue(ctx, j.Slug, "notice-to-quit", "lease violation you can fix") })
	err = pg.FileRuleValue(ctx, store.FileRuleValueParams{
		JurisdictionSlug: j.Slug, ConceptSlug: "notice-to-quit", Condition: "lease violation you can fix",
		Kind: "days", Value: 3, SourceURL: src.URL, Quote: "three days", By: "test",
	})
	if err != nil {
		t.Errorf("a listed situation was refused: %v", err)
	}
}

// The quality map judges a page against its topic's shape: a must-answer
// question the page leaves out fails, a days question with no stored
// number fails, and a stored number clears it.
func TestQualityMustAnswer(t *testing.T) {
	pg, j, src := ruleValueFixture(t)
	ctx := context.Background()
	if err := ingestTagged(t, pg, j.ID, "eviction-defense", "draft", "playbook",
		[2]string{"late-rent-notice", "The notice"}, [2]string{"eviction-court-process", "Going to court"}); err != nil {
		t.Fatal(err)
	}
	id := pageID(t, pg, j.ID, "eviction-defense", "draft")
	holes := func() (missing, noNumber int) {
		t.Helper()
		pages, err := pg.AuthorQuality(ctx)
		if err != nil {
			t.Fatalf("AuthorQuality: %v", err)
		}
		for _, p := range pages {
			if p.ID != id {
				continue
			}
			for _, c := range p.Checks {
				if c.Code != "must-answer" {
					continue
				}
				if c.Stmt == 0 {
					missing++
				} else {
					noNumber++
				}
			}
		}
		return missing, noNumber
	}
	// Eight must-answer questions: two are on the page, six are not. Of the
	// two, late rent is answered in days and has no number; what happens in
	// court is words only.
	if m, n := holes(); m != 6 || n != 1 {
		t.Errorf("before a number is filed: %d missing, %d without a number; want 6 and 1", m, n)
	}
	t.Cleanup(func() { _, _ = pg.RemoveRuleValue(ctx, j.Slug, "late-rent-notice", "") })
	if err := pg.FileRuleValue(ctx, store.FileRuleValueParams{
		JurisdictionSlug: j.Slug, ConceptSlug: "late-rent-notice", Kind: "days", Value: 3,
		SourceURL: src.URL, Quote: "three days", By: "test",
	}); err != nil {
		t.Fatal(err)
	}
	if m, n := holes(); m != 6 || n != 0 {
		t.Errorf("after a number is filed: %d missing, %d without a number; want 6 and 0", m, n)
	}
}
