package store_test

import (
	"context"
	"strings"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// Each question is its own field: the listing answer, whether our
// information is right, and any number of typed contacts.
func TestHelpOrgs_answersAndContacts(t *testing.T) {
	pg, jID, tID := revisionFixture(t)
	ctx := context.Background()
	_, _, _, host, word := helpPage(t, pg, jID, tID)
	if err := pg.SyncHelpOrgs(ctx); err != nil {
		t.Fatal(err)
	}

	if err := pg.LogHelpOrgContact(ctx, host, store.HelpOrgContact{
		Method: "call", Outcome: "maybe", InfoCorrect: store.InfoNeedsChanges, LoggedBy: "test",
	}); err == nil {
		t.Fatal("needs changes was accepted without saying what")
	}
	if err := pg.LogHelpOrgContact(ctx, host, store.HelpOrgContact{
		Method: "call", Outcome: "maybe", InfoCorrect: store.InfoNeedsChanges, InfoChanges: "New intake number.", LoggedBy: "test",
	}); err != nil {
		t.Fatal(err)
	}
	// A later attempt that leaves the answer alone keeps it.
	if err := pg.LogHelpOrgContact(ctx, host, store.HelpOrgContact{Method: "email", Outcome: "yes", LoggedBy: "test"}); err != nil {
		t.Fatal(err)
	}
	for _, c := range []store.HelpOrgChannel{
		{Kind: "phone", Value: "512-555-0100", Label: "Intake line", Preferred: true, AddedBy: "test"},
		{Kind: "walk_in", Value: "Tue 1 to 4 pm", AddedBy: "test"},
	} {
		if err := pg.AddHelpOrgChannel(ctx, host, c); err != nil {
			t.Fatal(err)
		}
	}
	if err := pg.AddHelpOrgChannel(ctx, host, store.HelpOrgChannel{Kind: "pigeon", Value: "x", AddedBy: "test"}); err == nil {
		t.Fatal("an unknown contact type was accepted")
	}

	orgs, err := pg.ListHelpOrgs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var o *store.HelpOrg
	for i := range orgs {
		if orgs[i].Host == host {
			o = &orgs[i]
		}
	}
	if o == nil {
		t.Fatal("org missing")
	}
	if o.Status != store.OrgOK || o.InfoCorrect != store.InfoNeedsChanges || o.InfoChanges != "New intake number." {
		t.Fatalf("answers = %s / %s / %q", o.Status, o.InfoCorrect, o.InfoChanges)
	}
	if len(o.Channels) != 2 || !o.Channels[0].Preferred || len(o.Preferred()) != 1 {
		t.Fatalf("channels = %+v", o.Channels)
	}
	if len(o.Statements) != 1 || !strings.Contains(o.Statements[0].Body, word) {
		t.Fatalf("statements = %+v", o.Statements)
	}
	if o.LastContact() == nil || len(o.Contacts) != 2 {
		t.Fatalf("contacts = %d", len(o.Contacts))
	}
	if err := pg.RemoveHelpOrgChannel(ctx, host, o.Channels[1].ID); err != nil {
		t.Fatal(err)
	}
}
