package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// Only a pending replacement on a draft, still on the page, and not a drift
// finding or a note, may be applied in bulk.
func TestBulkApplicable(t *testing.T) {
	ok := store.ProposalRow{Proposal: store.Proposal{Status: "pending", Reason: "agent-pass:voice-readability", Proposed: &store.ProposedStatement{BodyMD: "x"}}, TargetStatus: "draft", Position: 2}
	if !bulkApplicable(ok) {
		t.Fatal("a draft replacement is not applicable in bulk")
	}
	for name, mut := range map[string]func(*store.ProposalRow){
		"live page":      func(r *store.ProposalRow) { r.TargetStatus = "published" },
		"off the page":   func(r *store.ProposalRow) { r.Position = 0 },
		"no replacement": func(r *store.ProposalRow) { r.Proposed = nil },
		"drift":          func(r *store.ProposalRow) { r.Reason = store.ReasonSourceDrift },
		"decided":        func(r *store.ProposalRow) { r.Status = "rejected" },
	} {
		r := ok
		mut(&r)
		if bulkApplicable(r) {
			t.Errorf("%s: applicable in bulk", name)
		}
	}
	rules := bulkRules([]queueItem{{ProposalRow: ok}, {ProposalRow: ok}})
	if len(rules) != 1 || rules[0].Count != 2 || rules[0].Name != "voice-readability" {
		t.Errorf("rules = %+v", rules)
	}
}

func TestQueueTemplateBulkAndPinnedActions(t *testing.T) {
	render := func(extra map[string]any) string {
		data := map[string]any{"Actor": "Nazanin", "Status": "pending", "Items": nil, "Open": int64(0), "Sources": nil, "Count": 0, "Checking": false}
		for k, v := range extra {
			data[k] = v
		}
		var buf bytes.Buffer
		if err := parseTemplates(t).ExecuteTemplate(&buf, "queue.html", data); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}
	html := render(map[string]any{"Rules": []bulkRule{{Reason: "agent-pass:voice-readability", Name: "voice-readability", Count: 575}}})
	for _, want := range []string{`action="/queue/bulk"`, `value="agent-pass:voice-readability"`, "575 on drafts", "Apply all", ".item[open] .actions { position: fixed"} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q", want)
		}
	}
	html = render(map[string]any{"Rules": []bulkRule{{Reason: "r", Name: "r", Count: 1}}, "Bulk": &bulkStatus{Reason: "r", Done: 3, Total: 9}})
	if !strings.Contains(html, "3 of 9 done") || strings.Contains(html, "Apply all") {
		t.Error("a running bulk apply should show progress and no second button")
	}
}
