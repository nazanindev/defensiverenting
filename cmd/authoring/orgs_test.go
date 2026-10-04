package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/nazanindev/defensiverenting/internal/store"
)

func orgsFixture(now time.Time) []store.HelpOrg {
	due := now.Add(-24 * time.Hour)
	later := now.Add(10 * 24 * time.Hour)
	return []store.HelpOrg{
		{Host: "smalltenants.example", Name: "Small Tenant Union", Type: store.OrgContactFirst, Status: store.OrgMaybe,
			FollowUp: &due, Places: []string{"Austin", "Texas", "Dallas", "El Paso"}, LivePages: 2, Clicks30: 14,
			InfoCorrect: store.InfoNeedsChanges, InfoChanges: "New intake number.",
			Channels: []store.HelpOrgChannel{
				{ID: 1, Kind: "phone", Value: "512-555-0100", Label: "Intake line", Preferred: true},
				{ID: 2, Kind: "walk_in", Value: "Tue 1 to 4 pm"},
			},
			Statements: []store.HelpOrgStatement{{Place: "Austin", Topic: "Local Help", Body: "Small Tenant Union helps renters in Austin."}},
			Contacts:   []store.HelpOrgContact{{Method: "call", Reached: "Ana, director", Outcome: "maybe", Note: "Asking the board.", FollowUp: &due, LoggedBy: "editor", CreatedAt: now}}},
		{Host: "clinic.example", Name: "Clinic", Type: store.OrgContactFirst, Status: store.OrgNotContacted, LivePages: 1, InfoCorrect: store.InfoNotAsked},
		{Host: "agreed.example", Name: "Agreed Org", Type: store.OrgContactFirst, Status: store.OrgOK, FollowUp: &later, InfoCorrect: store.InfoYes},
		{Host: "211.org", Name: "211", Type: store.OrgPublic, Status: store.OrgNotContacted, LivePages: 9, InfoCorrect: store.InfoNotAsked},
	}
}

func TestOrgsTemplate_rendersTheTableAndScript(t *testing.T) {
	tmpl := parseTemplates(t)
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	orgs := orgsFixture(now)
	fills := map[string]string{}
	for _, o := range orgs {
		fills[o.Host] = `{"name":"` + o.Name + `"}`
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "orgs.html", map[string]any{
		"Actor": "editor", "Orgs": orgs, "Fills": fills, "Kinds": store.HelpOrgChannelKinds,
		"View": orgsView{Sort: "clicks", Dir: "desc"}, "Now": now, "Today": "2026-11-05",
		"Hiding": false, "Waiting": 2, "Listed": 1, "Script": orgScriptHTML,
	}); err != nil {
		t.Fatal(err)
	}
	body := buf.String()
	for _, want := range []string{
		// column titles, sortable, the sorted one marked
		"May we list you?", "Info correct?", "Their contacts", "Clicks 30d", `class="is-sorted"`, "▼",
		// always-visible metrics
		"Needs changes", "Intake line", "512-555-0100", "+4 in all", "Today", "never", "not needed",
		// the opened row
		"What we would say", "Small Tenant Union helps renters in Austin.", "1. May we list you?",
		"2. Is the information correct?", "3. Their contacts", "Walk-in", "Asking the board.",
		"New intake number.", "This org is built for public traffic", "Turn hiding on",
		// the pinned script
		"Open in new tab", "Show script", `class="page script-closed"`, "Contacting an org before we list it", "[paste the statements, exactly as they appear]",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("orgs page lacks %q", want)
		}
	}
}

func TestOrgScriptPage_renders(t *testing.T) {
	tmpl := parseTemplates(t)
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "orgscript.html", map[string]any{"Script": orgScriptHTML}); err != nil {
		t.Fatal(err)
	}
	// GFM renders the logging table as a table, not pipes.
	if !strings.Contains(buf.String(), "<table>") || !strings.Contains(buf.String(), "within two business days") {
		t.Fatal("script page lacks the rendered script")
	}
}

func TestSortHelpOrgs_dueFirstPublicLast(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)
	orgs := []store.HelpOrg{
		{Name: "public", Type: store.OrgPublic, LivePages: 50},
		{Name: "said no", Type: store.OrgContactFirst, Status: store.OrgLeaveOff},
		{Name: "listed", Type: store.OrgContactFirst, Status: store.OrgOK},
		{Name: "waiting small", Type: store.OrgContactFirst, Status: store.OrgNotContacted, LivePages: 1},
		{Name: "waiting big", Type: store.OrgContactFirst, Status: store.OrgNotContacted, LivePages: 6},
		{Name: "due", Type: store.OrgContactFirst, Status: store.OrgMaybe, FollowUp: &past},
	}
	store.SortHelpOrgs(orgs, now)
	var got []string
	for _, o := range orgs {
		got = append(got, o.Name)
	}
	want := "due,waiting big,waiting small,listed,said no,public"
	if strings.Join(got, ",") != want {
		t.Fatalf("order = %s, want %s", strings.Join(got, ","), want)
	}
}

func TestSortOrgs_byColumn(t *testing.T) {
	orgs := orgsFixture(time.Now())
	sortOrgs(orgs, "clicks", "desc")
	if orgs[0].Name != "Small Tenant Union" {
		t.Fatalf("clicks desc: first = %s", orgs[0].Name)
	}
	sortOrgs(orgs, "live", "desc")
	if orgs[0].Name != "211" {
		t.Fatalf("live desc: first = %s", orgs[0].Name)
	}
	sortOrgs(orgs, "list", "asc")
	if orgs[0].Status != store.OrgMaybe {
		t.Fatalf("list asc: first status = %s, want maybe", orgs[0].Status)
	}
}
