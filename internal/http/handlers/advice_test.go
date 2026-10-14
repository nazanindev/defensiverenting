package handlers_test

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/nazanindev/defensiverenting/internal/http/handlers"
	"github.com/nazanindev/defensiverenting/internal/store"
	tmpl "github.com/nazanindev/defensiverenting/web/templates"
)

// ADR-016 A2/A3: the page says the risk once at the top; a tip sits under
// the statement it helps with; an unconfirmed tip and a retired entry do
// not show; a risk note shows even if its quote drifted, because the
// statements under it no longer carry their own warning.
func TestAdvice_rendersNotesOnceAndTipsUnderTheirStatement(t *testing.T) {
	now := time.Now()
	confirmed := []store.AdviceCitation{{SourceID: 9, URL: "https://example.gov/guide", Publisher: "Example AG", Kind: "gov_guidance", Quote: "q", CheckedAt: &now}}
	drifted := []store.AdviceCitation{{SourceID: 9, URL: "https://example.gov/guide", Publisher: "Example AG", Kind: "gov_guidance", Quote: "q", CheckedAt: &now, DriftAt: &now}}
	pb := store.PlaybookWithStatements{
		Playbook:     store.Playbook{ID: 1, Title: "Breaking a Lease Early", Language: "en"},
		Jurisdiction: store.Jurisdiction{ID: 2, Kind: "state", Name: "Pennsylvania", Slug: "pennsylvania"},
		Topic:        store.Topic{ID: 3, Slug: "breaking-lease", Name: "Breaking a Lease"},
		Statements: []store.CitedStatement{
			{ID: 10, Key: "k1", BodyMD: "You can end your lease if the home is not usable.", Citations: []store.CitationWithSource{{SourceID: 5, SourceURL: "https://example.gov/law", Publisher: "Law", SourceKind: "statute"}}},
			{ID: 11, Key: "k2", BodyMD: "Your landlord must return your deposit within 30 days.", Citations: []store.CitationWithSource{{SourceID: 5, SourceURL: "https://example.gov/law", Publisher: "Law", SourceKind: "statute"}}},
		},
		Advice: []store.Advice{
			{Slug: "disclaimer", Kind: "page_note", SiteVoice: true, BodyMD: "Breaking a lease early is legally complicated."},
			{Slug: "risk", Kind: "page_note", Warns: "owe", BodyMD: "Your landlord can sue you for the rent.", Citations: drifted},
			{Slug: "doc", Kind: "tip", StatementKey: "k1", BodyMD: "Write down each problem.", Citations: confirmed},
			{Slug: "unchecked", Kind: "tip", StatementKey: "k2", BodyMD: "An unchecked tip."},
			{Slug: "gone", Kind: "tip", StatementKey: "k2", BodyMD: "A retired tip.", Citations: confirmed, Retired: true},
		},
	}
	page := handlers.BuildPlaybookPage(context.Background(), pb, nil, nil, slog.Default())
	var sb strings.Builder
	if err := tmpl.Render(&sb, page); err != nil {
		t.Fatal(err)
	}
	body := sb.String()
	notes := strings.Index(body, `class="page-notes"`)
	main := strings.Index(body, `aria-label="Guide content"`)
	if notes < 0 || notes > main {
		t.Fatalf("page notes should sit above the statements; notes=%d main=%d", notes, main)
	}
	for _, want := range []string{"legally complicated", "can sue you for the rent", "Write down each problem."} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q", want)
		}
	}
	for _, not := range []string{"An unchecked tip.", "A retired tip."} {
		if strings.Contains(body, not) {
			t.Errorf("%q should not show", not)
		}
	}
	first := strings.Index(body, `id="s-k1"`)
	second := strings.Index(body, `id="s-k2"`)
	tip := strings.Index(body, "Write down each problem.")
	if first >= tip || tip >= second {
		t.Fatalf("the tip should sit under its own statement: s1=%d tip=%d s2=%d", first, tip, second)
	}
	if strings.Count(body, `class="statement-tip"`) != 1 {
		t.Fatal("exactly one tip should render")
	}
}

// A page note can sit under the one statement it is about, with its lead in
// bold and its source on a line of its own; placed under a statement the
// page no longer has, it falls back to the top rather than vanish.
func TestAdvice_placedNoteSitsUnderItsStatement(t *testing.T) {
	now := time.Now()
	confirmed := []store.AdviceCitation{{SourceID: 9, URL: "https://example.gov/guide", Publisher: "Example AG", Kind: "gov_guidance", Quote: "q", CheckedAt: &now}}
	law := []store.CitationWithSource{{SourceID: 5, SourceURL: "https://example.gov/law", Publisher: "Law", SourceKind: "statute"}}
	pb := store.PlaybookWithStatements{
		Playbook:     store.Playbook{ID: 1, Title: "Breaking a Lease Early", Language: "en"},
		Jurisdiction: store.Jurisdiction{ID: 2, Kind: "state", Name: "Pennsylvania", Slug: "pennsylvania"},
		Topic:        store.Topic{ID: 3, Slug: "breaking-lease", Name: "Breaking a Lease"},
		Statements: []store.CitedStatement{
			{ID: 10, Key: "k1", BodyMD: "You can end your lease if the home is not usable.", Citations: law},
			{ID: 11, Key: "k2", BodyMD: "Your landlord must return your deposit within 30 days.", Citations: law},
		},
		Advice: []store.Advice{
			{Slug: "risk", Kind: "page_note", Warns: "owe", Lead: "You can still owe rent.", BodyMD: "Your landlord can sue you for the rent.", Citations: confirmed},
			{Slug: "condemn", Kind: "page_note", Warns: "condemn", SiteVoice: true, StatementKey: "k1", BodyMD: "The inspector can condemn the home."},
			{Slug: "orphan", Kind: "page_note", Warns: "evict", SiteVoice: true, StatementKey: "gone", BodyMD: "An orphaned warning."},
		},
	}
	page := handlers.BuildPlaybookPage(context.Background(), pb, nil, nil, slog.Default())
	var sb strings.Builder
	if err := tmpl.Render(&sb, page); err != nil {
		t.Fatal(err)
	}
	body := sb.String()
	main := strings.Index(body, `aria-label="Guide content"`)
	first := strings.Index(body, `id="s-k1"`)
	second := strings.Index(body, `id="s-k2"`)
	condemn := strings.Index(body, "can condemn the home")
	if first >= condemn || condemn >= second {
		t.Fatalf("the placed note should sit under its statement: s1=%d note=%d s2=%d", first, condemn, second)
	}
	if o := strings.Index(body, "An orphaned warning."); o < 0 || o > main {
		t.Fatalf("a note whose statement is gone should show at the top: note=%d main=%d", o, main)
	}
	if !strings.Contains(body, `<strong class="note-lead">You can still owe rent.</strong>`) {
		t.Error("the note's lead should render in bold")
	}
	if !strings.Contains(body, `<div class="note-source">`) {
		t.Error("the note's source should sit on its own line")
	}
}
