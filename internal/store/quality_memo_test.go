package store

import "testing"

func TestQualityMemoReusesUnchangedPages(t *testing.T) {
	in := &QualityInput{Language: "en", PageKind: "playbook", Warns: map[string]bool{},
		Statements: []QualityStatement{{Body: "You must pay rent within 5 days.", Citations: []QualityCitation{{SourceKind: "nonprofit", URL: "https://x", Quote: "q", Checked: true}}}}}
	var m qualityMemo
	first := m.checks(1, in)
	if len(first) == 0 {
		t.Fatal("expected a failing check")
	}
	again := m.checks(1, in)
	if &first[0] != &again[0] {
		t.Error("unchanged page was checked again")
	}
	in.Statements[0].Citations[0].SourceKind = "statute"
	changed := m.checks(1, in)
	if len(changed) == len(first) && len(first) > 0 && &first[0] == &changed[0] {
		t.Error("changed page reused stale checks")
	}
	m.keep(map[int64]*QualityInput{})
	if len(m.m) != 0 {
		t.Error("keep left a page that is no longer on the map")
	}
}
