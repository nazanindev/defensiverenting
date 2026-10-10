package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
	"github.com/nazanindev/defensiverenting/internal/voice"
)

func adviceFirstKey(t *testing.T, pg *store.PG, id int64) string {
	t.Helper()
	pw, err := pg.AuthorGetPlaybook(context.Background(), id)
	if err != nil || len(pw.Statements) == 0 {
		t.Fatalf("load page %d: %v", id, err)
	}
	return pw.Statements[0].Key
}

// confirmAdvice stamps every quote behind the named entries as the source
// checker would after finding them live.
func confirmAdvice(t *testing.T, pg *store.PG, slugs ...string) {
	t.Helper()
	for _, s := range slugs {
		if _, err := pg.Pool().Exec(context.Background(), `
			UPDATE advice_citations SET checked_at = now(), drift_at = NULL
			WHERE advice_id = (SELECT id FROM advice WHERE slug = $1)`, s); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAdvice_registryIsSeededAndBacked(t *testing.T) {
	pg := testDB(t)
	list, err := pg.ListAdvice(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	bySlug := map[string]store.Advice{}
	for _, a := range list {
		bySlug[a.Slug] = a
	}
	for _, s := range []string{"breaking-lease-disclaimer", "leave-risk-owe", "read-your-lease", "write-down-the-problem", "photos-when-you-leave", "answer-court-papers", "show-your-proof"} {
		if _, ok := bySlug[s]; !ok {
			t.Errorf("registry is missing %q", s)
		}
	}
	if !bySlug["breaking-lease-disclaimer"].Backed() {
		t.Error("the disclaimer is the site's own voice and needs no quote")
	}
	for _, s := range []string{"read-your-lease", "write-down-the-problem", "photos-when-you-leave"} {
		if !bySlug[s].SiteVoice {
			t.Errorf("%s is a plain habit and needs no source", s)
		}
	}
	for _, s := range []string{"leave-risk-owe", "answer-court-papers", "show-your-proof"} {
		if bySlug[s].SiteVoice {
			t.Errorf("%s states a consequence and must be backed", s)
		}
	}
	if len(bySlug["show-your-proof"].Citations) != 0 {
		t.Error("show-your-proof has no source yet and must not pretend to")
	}
	if bySlug["leave-risk-owe"].Warns != "owe" {
		t.Error("the leave risk note must warn about owing rent")
	}
}

func TestAdvice_onlyGovernmentOrNonprofitBacksAdvice(t *testing.T) {
	pg := testDB(t)
	ctx := context.Background()
	src, err := pg.UpsertSource(ctx, store.UpsertSourceParams{URL: "https://example.gov/statute-" + t.Name(), Publisher: "Example", Kind: "statute"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = pg.Pool().Exec(ctx, `
		INSERT INTO advice_citations (advice_id, source_id, quote)
		VALUES ((SELECT id FROM advice WHERE slug = 'show-your-proof'), $1, 'a statute says so')`, src.ID)
	if err == nil {
		t.Fatal("a statute was allowed to back advice")
	}
}

func TestAdvice_pageReferencesAreChecked(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	draft := seedPlaybook(t, pg, jID, tID, "draft", "Advice draft")
	key := adviceFirstKey(t, pg, draft)

	cases := []struct {
		name string
		ref  store.AdviceRef
	}{
		{"unknown slug", store.AdviceRef{PlaybookID: draft, Slug: "no-such-advice"}},
		{"tip with no statement", store.AdviceRef{PlaybookID: draft, Slug: "read-your-lease"}},
		{"tip on a statement elsewhere", store.AdviceRef{PlaybookID: draft, Slug: "read-your-lease", StatementKey: "00000000-0000-0000-0000-000000000000"}},
		{"page note on a statement", store.AdviceRef{PlaybookID: draft, Slug: "leave-risk-owe", StatementKey: key}},
	}
	for _, c := range cases {
		if err := pg.SetPageAdvice(ctx, []store.AdviceRef{c.ref}, store.ActorReviewAgent); !errors.Is(err, store.ErrAdviceRef) {
			t.Errorf("%s: got %v, want ErrAdviceRef", c.name, err)
		}
	}

	live := seedPlaybook(t, pg, jID, tID, "published", "Advice live")
	if err := pg.SetPageAdvice(ctx, []store.AdviceRef{{PlaybookID: live, Slug: "leave-risk-owe"}}, store.ActorReviewAgent); !errors.Is(err, store.ErrAdviceRef) {
		t.Errorf("an agent attached advice to a live page: %v", err)
	}
}

func TestAdvice_gateHoldsUnconfirmedAdvice(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	draft := seedPlaybook(t, pg, jID, tID, "draft", "Gate draft")
	key := adviceFirstKey(t, pg, draft)
	if _, err := pg.Pool().Exec(ctx, `UPDATE advice_citations SET checked_at = NULL`); err != nil {
		t.Fatal(err)
	}
	refs := []store.AdviceRef{
		{PlaybookID: draft, Slug: "breaking-lease-disclaimer"},
		{PlaybookID: draft, Slug: "leave-risk-owe"},
		{PlaybookID: draft, Slug: "read-your-lease", StatementKey: key},
	}
	if err := pg.SetPageAdvice(ctx, refs, store.ActorReviewAgent); err != nil {
		t.Fatal(err)
	}
	codes := pageIssueCodes(t, pg, draft)
	unbacked := 0
	for _, c := range codes {
		if c == "advice-unbacked" {
			unbacked++
		}
	}
	if unbacked != 1 {
		t.Fatalf("issues = %v, want one advice-unbacked: the risk note (the disclaimer and the read-your-lease habit need no quote)", codes)
	}
	confirmAdvice(t, pg, "leave-risk-owe", "read-your-lease")
	if codes := pageIssueCodes(t, pg, draft); len(codes) != 0 {
		t.Fatalf("confirmed advice still blocks publish: %v", codes)
	}

	warns, err := pg.PageWarns(ctx, draft)
	if err != nil || !warns["owe"] {
		t.Fatalf("PageWarns = %v, %v; want owe", warns, err)
	}
	pw, err := pg.AuthorGetPlaybook(ctx, draft)
	if err != nil {
		t.Fatal(err)
	}
	if len(pw.Advice) != 3 || pw.Advice[0].Kind != "page_note" {
		t.Fatalf("page advice = %+v, want notes first then the tip", pw.Advice)
	}

	if err := pg.RemovePageAdvice(ctx, []store.AdviceRef{{PlaybookID: draft, Slug: "read-your-lease"}}); err != nil {
		t.Fatal(err)
	}
	if err := pg.RemovePageAdvice(ctx, []store.AdviceRef{{PlaybookID: draft, Slug: "read-your-lease"}}); !errors.Is(err, store.ErrAdviceRef) {
		t.Fatalf("removing a reference twice: %v", err)
	}
}

func TestAdvice_newDraftKeepsTheLivePagesAdvice(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	draft := seedPlaybook(t, pg, jID, tID, "draft", "First version")
	if err := pg.SetPageAdvice(ctx, []store.AdviceRef{{PlaybookID: draft, Slug: "leave-risk-owe"}}, store.ActorReviewAgent); err != nil {
		t.Fatal(err)
	}
	confirmAdvice(t, pg, "leave-risk-owe")
	if err := pg.AuthorPublishPlaybook(ctx, draft, "test"); err != nil {
		t.Fatalf("publish: %v", err)
	}
	next := seedPlaybook(t, pg, jID, tID, "draft", "Second version")
	if next == draft {
		t.Fatal("the revision replaced the live page")
	}
	warns, err := pg.PageWarns(ctx, next)
	if err != nil || !warns["owe"] {
		t.Fatalf("the new draft lost the live page's warning: %v %v", warns, err)
	}
}

func TestAdvice_unusedSourcesSpareAdviceSources(t *testing.T) {
	pg := testDB(t)
	unused, err := pg.ListUnusedSources(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range unused {
		if s.URL == "https://www.attorneygeneral.gov/wp-content/uploads/ConsumerTenant-Landlord-Guide.pdf" {
			t.Fatal("a source that backs advice was listed for deletion")
		}
	}
}

// Registry entries are site voice: they pass the same lint as statements.
func TestAdvice_entriesPassTheVoiceLint(t *testing.T) {
	pg := testDB(t)
	list, err := pg.ListAdvice(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range list {
		if v := voice.LintAll("en", map[string]string{"body_md": a.BodyMD}); len(v) > 0 {
			t.Errorf("%s: %v", a.Slug, v)
		}
	}
}

// A page uses its own place's source first, falls back to a national one,
// and never leans on another place's guide.
func TestAdvice_backingFallsBackToNationalNeverAnotherState(t *testing.T) {
	pg := testDB(t)
	ctx := context.Background()
	n := t.Name()
	freshSlugs(t, pg, "home-state-"+n, "adv-topic-"+n)
	freshSlugs(t, pg, "other-state-"+n, "adv-topic2-"+n)
	home, err := pg.UpsertJurisdiction(ctx, store.UpsertJurisdictionParams{Kind: "state", Name: "Home State", Slug: "home-state-" + n})
	if err != nil {
		t.Fatal(err)
	}
	other, err := pg.UpsertJurisdiction(ctx, store.UpsertJurisdictionParams{Kind: "state", Name: "Other State", Slug: "other-state-" + n})
	if err != nil {
		t.Fatal(err)
	}
	tp, err := pg.UpsertTopic(ctx, store.UpsertTopicParams{Slug: "adv-topic-" + n, Name: "Adv Topic"})
	if err != nil {
		t.Fatal(err)
	}
	page := seedPlaybook(t, pg, home.ID, tp.ID, "draft", "Fallback page")
	src := func(url string, j *int64) int64 {
		s, err := pg.UpsertSource(ctx, store.UpsertSourceParams{URL: url, Publisher: url, Kind: "gov_guidance", JurisdictionID: j})
		if err != nil {
			t.Fatal(err)
		}
		return s.ID
	}
	otherSrc := src("https://other.gov/guide-"+n, &other.ID)
	homeSrc := src("https://home.gov/guide-"+n, &home.ID)
	var us int64
	if err := pg.Pool().QueryRow(ctx, `SELECT id FROM jurisdictions WHERE kind = 'country' LIMIT 1`).Scan(&us); err != nil {
		u, err := pg.UpsertJurisdiction(ctx, store.UpsertJurisdictionParams{Kind: "country", Name: "United States", Slug: "united-states"})
		if err != nil {
			t.Fatal(err)
		}
		us = u.ID
	}
	natSrc := src("https://national.gov/guide-"+n, &us)
	if _, err := pg.UpsertSource(ctx, store.UpsertSourceParams{URL: "https://noplace.gov/guide-" + n, Publisher: "x", Kind: "gov_guidance"}); err != nil {
		t.Fatal(err)
	}
	if _, err := pg.Pool().Exec(ctx, `INSERT INTO advice_citations (advice_id, source_id, quote)
		SELECT a.id, s.id, 'q' FROM advice a, sources s WHERE a.slug = 'show-your-proof' AND s.url = $1`, "https://noplace.gov/guide-"+n); err == nil {
		t.Fatal("an advice source with no place was accepted")
	}

	if _, err := pg.Pool().Exec(ctx, `
		WITH a AS (SELECT id FROM advice WHERE slug = $1),
		     r AS (DELETE FROM playbook_advice WHERE advice_id IN (SELECT id FROM a))
		DELETE FROM advice WHERE id IN (SELECT id FROM a)`, "tip-"+n); err != nil {
		t.Fatal(err)
	}
	var adviceID int64
	if err := pg.Pool().QueryRow(ctx, `
		INSERT INTO advice (slug, kind, body_md) VALUES ($1, 'tip', 'Keep a copy.') RETURNING id`, "tip-"+n).Scan(&adviceID); err != nil {
		t.Fatal(err)
	}
	cite := func(source int64, pos int) {
		if _, err := pg.Pool().Exec(ctx, `
			INSERT INTO advice_citations (advice_id, source_id, quote, position, checked_at) VALUES ($1, $2, 'keep a copy', $3, now())`,
			adviceID, source, pos); err != nil {
			t.Fatal(err)
		}
	}
	cite(otherSrc, 0)
	key := adviceFirstKey(t, pg, page)
	if err := pg.SetPageAdvice(ctx, []store.AdviceRef{{PlaybookID: page, Slug: "tip-" + n, StatementKey: key}}, store.ActorReviewAgent); err != nil {
		t.Fatal(err)
	}
	if codes := pageIssueCodes(t, pg, page); len(codes) != 1 || codes[0] != "advice-unbacked" {
		t.Fatalf("another state's guide backed the tip: %v", codes)
	}

	cite(natSrc, 1)
	if codes := pageIssueCodes(t, pg, page); len(codes) != 0 {
		t.Fatalf("a national source should back the tip: %v", codes)
	}
	got, err := pg.PageAdvice(ctx, page)
	if err != nil || len(got) != 1 || len(got[0].Citations) != 1 || got[0].Citations[0].SourceID != natSrc {
		t.Fatalf("want only the national source, got %+v %v", got, err)
	}

	cite(homeSrc, 2)
	got, err = pg.PageAdvice(ctx, page)
	if err != nil || len(got[0].Citations) != 2 || got[0].Citations[0].SourceID != homeSrc {
		t.Fatalf("the page's own state should come first, then national: %+v %v", got, err)
	}
}
