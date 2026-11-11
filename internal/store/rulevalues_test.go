package store_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// ruleValueFixture is a fresh state with one statute source a statement
// cites, so a rule value has something real to point at.
func ruleValueFixture(t *testing.T) (*store.PG, store.Jurisdiction, store.Source) {
	t.Helper()
	pg, j := rulesFixture(t)
	ctx := context.Background()
	if err := ingestTagged(t, pg, j.ID, "rent-increase", "published", "playbook", [2]string{"increase-notice-period", "How much notice you get"}); err != nil {
		t.Fatal(err)
	}
	src, err := pg.UpsertSource(ctx, store.UpsertSourceParams{
		URL: "https://example.gov/rules-" + t.Name(), Publisher: "Example", Kind: "statute",
	})
	if err != nil {
		t.Fatal(err)
	}
	return pg, j, src
}

func TestRuleValue_fileListReplaceRemove(t *testing.T) {
	pg, j, src := ruleValueFixture(t)
	ctx := context.Background()
	t.Cleanup(func() { _, _ = pg.RemoveRuleValue(ctx, j.Slug, "increase-notice-period", "") })

	p := store.FileRuleValueParams{
		JurisdictionSlug: j.Slug, ConceptSlug: "increase-notice-period",
		Kind: "days", Value: 30, SourceURL: src.URL, Locator: "§ 1", Quote: "thirty days", By: "test",
	}
	if err := pg.FileRuleValue(ctx, p); err != nil {
		t.Fatalf("file: %v", err)
	}
	rows, err := pg.ListRuleValues(ctx, j.Slug, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Value != 30 || rows[0].Kind != "days" || rows[0].SourceURL != src.URL || rows[0].CheckedBy != "test" {
		t.Fatalf("listed %+v", rows)
	}
	if rows[0].ValueMax != nil {
		t.Errorf("single value should have no max, got %v", *rows[0].ValueMax)
	}

	// A second condition is a second row; the same key replaces.
	p2 := p
	p2.Condition = "increase over 10 percent"
	p2.Value, p2.Quote = 60, "sixty days"
	if err := pg.FileRuleValue(ctx, p2); err != nil {
		t.Fatalf("file second condition: %v", err)
	}
	t.Cleanup(func() { _, _ = pg.RemoveRuleValue(ctx, j.Slug, "increase-notice-period", p2.Condition) })
	if err := pg.MarkRuleValueCitation(ctx, rows[0].ID, false, "gone"); err != nil {
		t.Fatal(err)
	}
	max := 45.0
	p.Value, p.ValueMax = 30, &max
	if err := pg.FileRuleValue(ctx, p); err != nil {
		t.Fatalf("refile: %v", err)
	}
	rows, err = pg.ListRuleValues(ctx, j.Slug, "increase-notice-period")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rows))
	}
	if rows[0].Condition != "" || rows[0].ValueMax == nil || *rows[0].ValueMax != 45 {
		t.Errorf("refiled row = %+v", rows[0])
	}
	if rows[0].DriftAt != nil || rows[0].DriftNote != "" {
		t.Errorf("a refile clears the checker's drift mark, got %v %q", rows[0].DriftAt, rows[0].DriftNote)
	}
	if rows[1].Condition != p2.Condition || rows[1].Value != 60 {
		t.Errorf("second row = %+v", rows[1])
	}

	ok, err := pg.RemoveRuleValue(ctx, j.Slug, "increase-notice-period", p2.Condition)
	if err != nil || !ok {
		t.Fatalf("remove: ok=%v err=%v", ok, err)
	}
	ok, err = pg.RemoveRuleValue(ctx, j.Slug, "increase-notice-period", p2.Condition)
	if err != nil || ok {
		t.Fatalf("second remove: ok=%v err=%v", ok, err)
	}
}

func TestRuleValue_invariants(t *testing.T) {
	pg, j, src := ruleValueFixture(t)
	ctx := context.Background()
	good := store.FileRuleValueParams{
		JurisdictionSlug: j.Slug, ConceptSlug: "increase-notice-period",
		Kind: "days", Value: 30, SourceURL: src.URL, Quote: "thirty days", By: "test",
	}
	cases := map[string]func(p *store.FileRuleValueParams){
		"unknown kind":      func(p *store.FileRuleValueParams) { p.Kind = "weeks" },
		"negative value":    func(p *store.FileRuleValueParams) { p.Value = -1 },
		"backwards range":   func(p *store.FileRuleValueParams) { m := 10.0; p.ValueMax = &m },
		"empty quote":       func(p *store.FileRuleValueParams) { p.Quote = "  " },
		"nobody filed":      func(p *store.FileRuleValueParams) { p.By = "" },
		"unknown source":    func(p *store.FileRuleValueParams) { p.SourceURL = "https://example.gov/nowhere-" + t.Name() },
		"unknown concept":   func(p *store.FileRuleValueParams) { p.ConceptSlug = "no-such-concept" },
		"unknown place":     func(p *store.FileRuleValueParams) { p.JurisdictionSlug = "no-such-place" },
		"editorial source":  func(p *store.FileRuleValueParams) { p.SourceURL = editorialURL(t, pg) },
	}
	for name, mutate := range cases {
		p := good
		mutate(&p)
		if err := pg.FileRuleValue(ctx, p); err == nil {
			t.Errorf("%s: filed, want refusal", name)
			_, _ = pg.RemoveRuleValue(ctx, j.Slug, "increase-notice-period", "")
		}
	}
	p := good
	p.JurisdictionSlug = "no-such-place"
	if err := pg.FileRuleValue(ctx, p); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("unknown place: want ErrNotFound, got %v", err)
	}
}

func editorialURL(t *testing.T, pg *store.PG) string {
	t.Helper()
	var url string
	if err := pg.Pool().QueryRow(context.Background(), `SELECT url FROM sources WHERE kind = 'editorial' LIMIT 1`).Scan(&url); err != nil {
		t.Fatalf("editorial source: %v", err)
	}
	return url
}

func TestRuleValue_checkRowsAndInput(t *testing.T) {
	pg, j, src := ruleValueFixture(t)
	ctx := context.Background()
	t.Cleanup(func() { _, _ = pg.RemoveRuleValue(ctx, j.Slug, "increase-notice-period", "") })
	if err := pg.FileRuleValue(ctx, store.FileRuleValueParams{
		JurisdictionSlug: j.Slug, ConceptSlug: "increase-notice-period",
		Kind: "days", Value: 30, SourceURL: src.URL, Quote: "thirty days", By: "test",
	}); err != nil {
		t.Fatal(err)
	}
	rows, err := pg.ListRuleValueCitationsForCheck(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var mine *store.RuleValueCheckRow
	for i := range rows {
		if rows[i].Place == j.Slug {
			mine = &rows[i]
		}
	}
	if mine == nil || mine.URL != src.URL || mine.Quote != "thirty days" || mine.Concept != "increase-notice-period" {
		t.Fatalf("check row = %+v", mine)
	}
	if err := pg.MarkRuleValueCitation(ctx, mine.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	listed, err := pg.ListRuleValues(ctx, j.Slug, "")
	if err != nil || len(listed) != 1 || listed[0].SourceCheckedAt == nil {
		t.Fatalf("after mark found: %+v err=%v", listed, err)
	}

	in, err := pg.StatementsForConcept(ctx, "increase-notice-period")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range in {
		if s.JurisdictionSlug != j.Slug {
			continue
		}
		found = true
		if !strings.Contains(s.BodyMD, "rent-increase") || len(s.Citations) != 1 || s.Citations[0].URL != src.URL || s.Citations[0].Quote != "verbatim" {
			t.Errorf("input row = %+v", s)
		}
	}
	if !found {
		t.Errorf("fixture statement missing from StatementsForConcept")
	}
}
