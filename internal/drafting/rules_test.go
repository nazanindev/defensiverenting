package drafting

import (
	"context"
	"strings"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// rulesStore extends the fake with a rules topic, stage lists, concept homes,
// and answers already given on another page (ADR-028).
type rulesStore struct {
	fakeStore
	answered map[string]string // concept slug -> title of the page answering it
}

func (f *rulesStore) GetTopicBySlug(_ context.Context, slug string) (store.Topic, error) {
	switch slug {
	case "security-deposit-rules":
		return store.Topic{ID: 70, Slug: slug, Name: "Security Deposit Rules", RulesFor: 7}, nil
	case "move-in-checklist":
		return store.Topic{ID: 71, Slug: slug, NationalOnly: true}, nil
	}
	return store.Topic{ID: 7, Slug: slug, Name: "Security Deposits", Stages: []string{"What the law says", "Ask for your deposit back"}}, nil
}

func (f *rulesStore) ListConcepts(_ context.Context) ([]store.Concept, error) {
	return []store.Concept{
		{ID: 1, Slug: "deposit-return-deadline", TopicID: 7, TopicSlug: "security-deposits"},
		{ID: 4, Slug: "deposit-cap", TopicID: 7, TopicSlug: "security-deposits"},
		{ID: 3, Slug: "entry-notice-period", TopicID: 9, TopicSlug: "landlord-entry"},
	}, nil
}

func (f *rulesStore) ConceptAnsweredElsewhere(_ context.Context, _, _ int64, concept, _ string) (string, error) {
	return f.answered[concept], nil
}

func (f *rulesStore) ListTopicRegistry(_ context.Context) ([]store.Topic, error) {
	return []store.Topic{{ID: 7, Slug: "security-deposits"}, {ID: 70, Slug: "security-deposit-rules", RulesFor: 7}}, nil
}

func rulesToolbelt(t *testing.T, fs *rulesStore) *Toolbelt {
	t.Helper()
	tb := newTestToolbelt(&fs.fakeStore, map[string]string{
		depositURL: `<p>A lessor shall, within thirty days after the termination of the tenancy, return the security deposit.</p>`,
	})
	tb.db = fs
	mustFetch(t, tb, depositURL)
	return tb
}

func tagged(concept, stage string) StatementInput {
	st := stmt("your landlord must return your deposit within 30 days.", depositURL, "within thirty days after the termination of the tenancy")
	st.Concept, st.Stage = concept, stage
	return st
}

func save(tb *Toolbelt, topic, kind string, stmts ...StatementInput) error {
	_, err := tb.SaveDraft(context.Background(), SaveDraftInput{
		JurisdictionSlug: "boston", TopicSlug: topic, Title: "Boston deposits", IntroMD: "Boston rules.",
		PageKind: kind, Statements: stmts,
	})
	return err
}

// A statement does not have to name its place (ADR-028 D11, amended
// 2026-10-04): the page title and headings carry it, and repeating it in
// every statement made pages hard to read.
func TestSaveDraft_StatementNeedNotNamePlace(t *testing.T) {
	fs := &rulesStore{}
	tb := rulesToolbelt(t, fs)
	bare := StatementInput{
		BodyMD:    "Your landlord must return your deposit within 30 days.",
		Citations: []CitationInput{{URL: depositURL, Kind: "statute", Locator: "§ 15B", Quote: "within thirty days after the termination of the tenancy"}},
	}
	if err := save(tb, "security-deposits", "", bare); err != nil {
		t.Fatalf("a statement without its place name was refused: %v", err)
	}
}

func TestSaveDraft_Stages(t *testing.T) {
	fs := &rulesStore{}
	tb := rulesToolbelt(t, fs)
	if err := save(tb, "security-deposits", "", tagged("", "What the law says")); err != nil {
		t.Fatalf("a listed stage was refused: %v", err)
	}
	if got := fs.ingested.Statements[0].Stage; got != "What the law says" {
		t.Fatalf("stage not passed to the save: %q", got)
	}
	err := save(tb, "security-deposits", "", tagged("", "Your rights"))
	if err == nil || !strings.Contains(err.Error(), "What the law says | Ask for your deposit back") {
		t.Fatalf("an invented stage saved, or the rejection hid the choices: %v", err)
	}
}

func TestSaveDraft_RulesPage(t *testing.T) {
	fs := &rulesStore{answered: map[string]string{"deposit-return-deadline": "Boston deposits: what can I do?"}}
	tb := rulesToolbelt(t, fs)

	if err := save(tb, "security-deposit-rules", "", tagged("deposit-cap", "")); err == nil || !strings.Contains(err.Error(), `page_kind=\"rules\"`) && !strings.Contains(err.Error(), `page_kind="rules"`) {
		t.Fatalf("a rules topic saved as a playbook: %v", err)
	}
	if err := save(tb, "security-deposits", "rules", tagged("deposit-cap", "")); err == nil {
		t.Fatal("a situation topic saved with the rules layout")
	}
	if err := save(tb, "security-deposit-rules", "rules", tagged("", "")); err == nil {
		t.Fatal("an untagged statement on a rules page")
	}
	if err := save(tb, "security-deposit-rules", "rules", tagged("entry-notice-period", "")); err == nil {
		t.Fatal("a concept of another topic on a rules page")
	}
	if err := save(tb, "security-deposit-rules", "rules", tagged("deposit-cap", ""), tagged("deposit-cap", "")); err == nil {
		t.Fatal("two answers to one concept")
	}
	err := save(tb, "security-deposit-rules", "rules", tagged("deposit-return-deadline", ""))
	if err == nil || !strings.Contains(err.Error(), "already answered") {
		t.Fatalf("a concept the situation page answers was drafted twice: %v", err)
	}
	if err := save(tb, "security-deposit-rules", "rules", tagged("deposit-cap", "")); err != nil {
		t.Fatalf("a gap answer was refused: %v", err)
	}
	if fs.ingested.PageKind != "rules" {
		t.Fatalf("saved as %q", fs.ingested.PageKind)
	}

	if err := save(tb, "move-in-checklist", "", tagged("", "")); err == nil || !strings.Contains(err.Error(), "national only") {
		t.Fatalf("a national-only topic saved on a city: %v", err)
	}
}
