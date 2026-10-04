package store_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// helpPage publishes a page in a fresh place with one statute statement and
// one statement citing a nonprofit at a host no other test uses. It returns
// the page id, both statement ids, the org's host, and a word only the
// nonprofit statement contains.
func helpPage(t *testing.T, pg *store.PG, jID, tID int64) (pageID, lawStmt, orgStmt int64, host, word string) {
	t.Helper()
	ctx := context.Background()
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	tag := hex.EncodeToString(b)
	host = "org-" + tag + ".example"
	word = "zq" + tag
	law, err := pg.UpsertSource(ctx, store.UpsertSourceParams{URL: "https://example.gov/law-" + tag, Publisher: "Example", Kind: "statute"})
	if err != nil {
		t.Fatal(err)
	}
	org, err := pg.UpsertSource(ctx, store.UpsertSourceParams{URL: "https://www." + host + "/help", Publisher: "Small Tenant Union", Kind: "nonprofit"})
	if err != nil {
		t.Fatal(err)
	}
	cite := func(id int64) []store.IngestCitationParams {
		return []store.IngestCitationParams{{SourceID: id, Locator: "§ 1", Quote: "verbatim", CheckedNow: true, CheckedBy: "test"}}
	}
	if err := pg.IngestPlaybook(ctx, store.IngestPlaybookParams{
		JurisdictionID: jID, TopicID: tID, Language: "en", Slug: "help-" + tag,
		Title: "Help", IntroMD: "intro", Status: "published", UpdatedBy: "test",
		Statements: []store.IngestStatementParams{
			{BodyMD: "The law says this.", Language: "en", Sources: cite(law.ID)},
			{BodyMD: "Call the Small Tenant Union " + word + ".", Language: "en", Sources: cite(org.ID)},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := pg.Pool().QueryRow(ctx,
		`SELECT id FROM playbooks WHERE jurisdiction_id=$1 AND topic_id=$2 AND status='published'`, jID, tID).Scan(&pageID); err != nil {
		t.Fatal(err)
	}
	pw, err := pg.AuthorGetPlaybook(ctx, pageID)
	if err != nil {
		t.Fatal(err)
	}
	return pageID, pw.Statements[0].ID, pw.Statements[1].ID, host, word
}

// setHiding flips the switch directly, past SetHelpHiding's D8 check, and
// puts it back when the test ends.
func setHiding(t *testing.T, pg *store.PG, on bool) {
	t.Helper()
	ctx := context.Background()
	was, err := pg.HelpHiding(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pg.Pool().Exec(ctx, `UPDATE help_hiding SET hiding = $1`, on); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pg.Pool().Exec(context.Background(), `UPDATE help_hiding SET hiding = $1`, was) })
}

func TestHelpOrgs_heldUntilTheOrgSaysYes(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	_, law, org, host, _ := helpPage(t, pg, jID, tID)

	held := func() string {
		t.Helper()
		m, err := pg.StatementsHeldBy(ctx, []int64{law, org})
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := m[law]; ok {
			t.Fatal("a statute statement was held")
		}
		return m[org]
	}

	// No row yet: an org nobody has looked at is held.
	if got := held(); got != host {
		t.Fatalf("unknown org: held by %q, want %q (www. stripped)", got, host)
	}
	if err := pg.SyncHelpOrgs(ctx); err != nil {
		t.Fatal(err)
	}
	log := func(outcome string) {
		t.Helper()
		if err := pg.LogHelpOrgContact(ctx, host, store.HelpOrgContact{Method: "call", Outcome: outcome, LoggedBy: "test"}); err != nil {
			t.Fatal(err)
		}
	}
	for _, step := range []struct {
		outcome string
		held    bool
	}{
		{"no_answer", true},
		{"maybe", true},
		{"yes", false},
		{"no", true},
	} {
		log(step.outcome)
		if got := held() != ""; got != step.held {
			t.Fatalf("after %s: held = %v, want %v", step.outcome, got, step.held)
		}
	}

	// A public org is never held, whatever its status.
	if err := pg.SetHelpOrgType(ctx, host, store.OrgPublic, "test"); err != nil {
		t.Fatal(err)
	}
	if got := held(); got != "" {
		t.Fatalf("public org held by %q", got)
	}

	if err := pg.LogHelpOrgContact(ctx, host, store.HelpOrgContact{Method: "fax", Outcome: "yes", LoggedBy: "test"}); err == nil {
		t.Fatal("an unknown method was logged")
	}
	if err := pg.LogHelpOrgContact(ctx, host, store.HelpOrgContact{Method: "call", Outcome: "probably", LoggedBy: "test"}); err == nil {
		t.Fatal("an unknown outcome was logged")
	}
}

// Hidden means held and the switch is on. Search follows the same rule.
func TestHelpOrgs_hiddenOnlyWhileTheSwitchIsOn(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	_, _, org, _, word := helpPage(t, pg, jID, tID)

	found := func() bool {
		t.Helper()
		res, err := pg.Search(ctx, word, nil, "en")
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range res {
			if r.StatementID != nil && *r.StatementID == org {
				return true
			}
		}
		return false
	}

	setHiding(t, pg, false)
	if m, err := pg.HiddenStatements(ctx, []int64{org}); err != nil || len(m) != 0 {
		t.Fatalf("switch off: hidden = %v, err %v", m, err)
	}
	if !found() {
		t.Fatal("switch off: search lost the statement")
	}

	setHiding(t, pg, true)
	if m, err := pg.HiddenStatements(ctx, []int64{org}); err != nil || len(m) != 1 {
		t.Fatalf("switch on: hidden = %v, err %v", m, err)
	}
	if found() {
		t.Fatal("switch on: search still shows a hidden statement")
	}
}

// D8: hiding will not turn on while a live Local Help page lacks public help.
func TestHelpOrgs_switchRefusedWhileLocalHelpIsShort(t *testing.T) {
	pg, jID, _ := revisionFixture(t)
	ctx := context.Background()
	lh, err := pg.UpsertTopic(ctx, store.UpsertTopicParams{Slug: store.LocalHelpTopic, Name: "Local Help"})
	if err != nil {
		t.Fatal(err)
	}
	page, _, _, _, _ := helpPage(t, pg, jID, lh.ID)
	setHiding(t, pg, false)

	if n, err := pg.LocalHelpPublicCount(ctx, page); err != nil || n != 1 {
		t.Fatalf("public statements = %d (err %v), want 1: the statute counts, the union does not", n, err)
	}
	codes := pageIssueCodes(t, pg, page)
	if !strings.Contains(strings.Join(codes, ","), "local-help-public") {
		t.Fatalf("issues = %v, want local-help-public on a Local Help page with one public statement", codes)
	}
	err = pg.SetHelpHiding(ctx, true)
	if err == nil || !strings.Contains(err.Error(), "Rev City") {
		t.Fatalf("switch on with a short Local Help page: err = %v", err)
	}
	if on, _ := pg.HelpHiding(ctx); on {
		t.Fatal("the switch turned on anyway")
	}
}

func TestHelpOrgs_sourceHost(t *testing.T) {
	pg := testDB(t)
	cases := map[string]string{
		"https://www.TexasLawHelp.org/article/x":    "texaslawhelp.org",
		"http://assets.washingtonlawhelp.org/a.pdf": "assets.washingtonlawhelp.org",
		"https://211.org:443/?q=1":                  "211.org",
		"https://lsc.gov#top":                       "lsc.gov",
	}
	for url, want := range cases {
		var got string
		if err := pg.Pool().QueryRow(context.Background(), `SELECT source_host($1)`, url).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("source_host(%q) = %q, want %q", url, got, want)
		}
	}
}
