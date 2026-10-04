package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/nazanindev/defensiverenting/internal/store"
)

func TestOrgsTemplate_rendersEachKindOfOrg(t *testing.T) {
	tmpl := parseTemplates(t)
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	due := now.Add(-24 * time.Hour)
	later := now.Add(10 * 24 * time.Hour)
	orgs := []store.HelpOrg{
		{Host: "smalltenants.example", Name: "Small Tenant Union", Type: store.OrgContactFirst, Status: store.OrgMaybe,
			FollowUp: &due, Places: []string{"Austin"}, LivePages: 2,
			Contacts: []store.HelpOrgContact{{Method: "call", Reached: "Ana, director", Outcome: "maybe", Note: "Asking the board.", FollowUp: &due, LoggedBy: "editor", CreatedAt: now}}},
		{Host: "clinic.example", Name: "Clinic", Type: store.OrgContactFirst, Status: store.OrgNotContacted, LivePages: 1},
		{Host: "agreed.example", Name: "Agreed Org", Type: store.OrgContactFirst, Status: store.OrgOK, FollowUp: &later},
		{Host: "211.org", Name: "211", Type: store.OrgPublic, Status: store.OrgNotContacted, LivePages: 9},
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "orgs.html", map[string]any{
		"Actor": "editor", "Orgs": orgs, "Now": now, "Today": "2026-11-05",
		"Hiding": false, "Waiting": 2, "Listed": 1,
	}); err != nil {
		t.Fatal(err)
	}
	body := buf.String()
	for _, want := range []string{
		"Follow up today", "Asking the board.", "Ana, director", "Not contacted", "Said yes", "Public",
		"Hiding is not on yet", `id="smalltenants.example"`, "This org is built for public traffic",
		"This is a small local org: contact first", "Turn hiding on",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("orgs page lacks %q", want)
		}
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
