package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// Every item with an Apply button can be ticked: a pending replacement for a
// statement still on a draft or live page, quote moves included.
func TestBulkApplicable(t *testing.T) {
	ok := store.ProposalRow{Proposal: store.Proposal{Status: "pending", Reason: "agent-pass:voice-readability", Proposed: &store.ProposedStatement{BodyMD: "x"}}, TargetStatus: "draft", Position: 2}
	if !bulkApplicable(ok) {
		t.Fatal("a draft replacement cannot be ticked")
	}
	for name, mut := range map[string]func(*store.ProposalRow){
		"retired page":   func(r *store.ProposalRow) { r.TargetStatus = "superseded" },
		"off the page":   func(r *store.ProposalRow) { r.Position = 0 },
		"no replacement": func(r *store.ProposalRow) { r.Proposed = nil },
		"note":           func(r *store.ProposalRow) { r.Reason = store.ReasonReviewerFlag },
		"decided":        func(r *store.ProposalRow) { r.Status = "rejected" },
	} {
		r := ok
		mut(&r)
		if bulkApplicable(r) {
			t.Errorf("%s: can be ticked", name)
		}
	}
	live := ok
	live.TargetStatus = "published"
	if !bulkApplicable(live) {
		t.Error("a live-page replacement cannot be ticked")
	}
	moved := ok
	moved.Reason = store.ReasonSourceDrift
	moved.Evidence = []byte(`{"old_quote":"unfair, deceptive, or abusive","new_quote":"unfair or deceptive"}`)
	if !bulkApplicable(moved) {
		t.Error("a quote move with a replacement cannot be ticked")
	}
}

// No per-reason Apply all lines: a running bulk apply shows progress only.
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
	html := render(nil)
	if strings.Contains(html, "Apply all") || !strings.Contains(html, ".item[open] .actions { position: fixed") {
		t.Error("the queue should have pinned actions and no Apply all line")
	}
	html = render(map[string]any{"Bulk": &bulkStatus{Done: 3, Total: 9}})
	if !strings.Contains(html, "3 of 9 done") {
		t.Error("a running bulk apply should show progress")
	}
}

// A replacement on a draft or live page gets a tick box tied to the Apply
// selected form, a quote move included; a note gets none.
func TestQueueTemplatePickBoxes(t *testing.T) {
	row := store.ProposalRow{Proposal: store.Proposal{ID: 41, Status: "pending", Reason: "agent-pass:voice-readability", Proposed: &store.ProposedStatement{BodyMD: "x"}}, TargetStatus: "draft", Position: 1}
	live := row
	live.ID, live.TargetStatus = 42, "published"
	drift := row
	drift.ID, drift.Reason = 43, store.ReasonSourceDrift
	note := row
	note.ID, note.Reason, note.Proposed = 44, store.ReasonReviewerFlag, nil
	items := []queueItem{newQueueItem(row), newQueueItem(live), newQueueItem(drift), newQueueItem(note)}
	for i := range items {
		items[i].Pickable = bulkApplicable(items[i].ProposalRow)
	}
	var buf bytes.Buffer
	err := parseTemplates(t).ExecuteTemplate(&buf, "queue.html", map[string]any{
		"Actor": "Nazanin", "Status": "pending", "Items": items, "Open": int64(41), "Sources": nil, "Count": 4, "Checking": false, "Pickable": 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	html := buf.String()
	if !strings.Contains(html, `form="picked" name="id" value="41"`) {
		t.Error("the draft item has no tick box")
	}
	if !strings.Contains(html, `name="id" value="42"`) {
		t.Error("the live-page item has no tick box")
	}
	if !strings.Contains(html, `name="id" value="43"`) {
		t.Error("the quote move has no tick box")
	}
	if strings.Contains(html, `name="id" value="44"`) {
		t.Error("the note offers a tick box")
	}
	if !strings.Contains(html, `data-own="approve41"`) {
		t.Error("the open item's Apply cannot switch to the selection")
	}
	if !strings.Contains(html, `id="pickall"`) || !strings.Contains(html, "Tick all 3") {
		t.Error("no tick-all box")
	}
	if !strings.Contains(html, `id="picked" method="post" action="/queue/bulk"`) {
		t.Error("no Apply selected form")
	}
}
