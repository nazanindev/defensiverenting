package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"sync"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// Bulk apply: the items ticked on the queue page, applied together. Each one
// goes through the same applyApproval a single Apply does (live quote
// checks, the reviewer's name, and on a live page the gate that refuses a
// save adding a problem), so a bulk apply is many ordinary applies, not a
// shortcut past them. Live pages are included by the reviewer's decision
// (2026-09-26). Ticks are the only bulk path (2026-10-03): the per-reason
// "Apply all" lines were dropped because nothing showed which items they
// covered, and a tick is on exactly the item it applies.

// bulkApplicable reports whether an item can be ticked: it has an Apply
// button, that is a pending replacement for a statement still on a draft or
// live page. A drift finding with a replacement quote is tickable like any
// other; one with no replacement, and a reviewer note, have no Apply.
func bulkApplicable(row store.ProposalRow) bool {
	return row.Status == "pending" && row.Proposed != nil && row.OnPage() &&
		(row.TargetStatus == "draft" || row.TargetStatus == "published") && row.Reason != store.ReasonReviewerFlag
}

// bulkRun is the one bulk apply in flight, if any, and the errors of the
// last one, keyed by proposal id so a failed item shows why in the queue.
// Held in memory: a restart forgets the errors, and the items stay pending.
type bulkRun struct {
	mu      sync.Mutex
	running bool
	done    int
	total   int
	failed  map[int64]string
}

type bulkStatus struct {
	Done, Total int
}

func (b *bulkRun) status() *bulkStatus {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.running {
		return nil
	}
	return &bulkStatus{Done: b.done, Total: b.total}
}

func (b *bulkRun) failure(id int64) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.failed[id]
}

func (s *srv) bulkApprove(w http.ResponseWriter, r *http.Request) {
	picked := map[int64]bool{}
	if err := r.ParseForm(); err == nil {
		for _, v := range r.Form["id"] {
			if id, err := strconv.ParseInt(v, 10, 64); err == nil {
				picked[id] = true
			}
		}
	}
	rows, err := s.pg.ListProposalsByReason(r.Context(), "pending", "")
	if err != nil {
		s.serverError(w, err)
		return
	}
	var ids []int64
	for _, row := range rows {
		if picked[row.ID] && bulkApplicable(row) {
			ids = append(ids, row.ID)
		}
	}
	back := func(msg, errMsg string) {
		q := url.Values{}
		if msg != "" {
			q.Set("msg", msg)
		}
		if errMsg != "" {
			q.Set("err", errMsg)
		}
		http.Redirect(w, r, "/queue?"+q.Encode(), http.StatusSeeOther)
	}
	if len(ids) == 0 {
		back("", "Nothing ticked can be applied.")
		return
	}
	b := s.bulk
	b.mu.Lock()
	if b.running {
		b.mu.Unlock()
		back("", "A bulk apply is already running.")
		return
	}
	b.running, b.done, b.total, b.failed = true, 0, len(ids), map[int64]string{}
	b.mu.Unlock()

	by := actor(r)
	// The run outlives the request that started it.
	ctx := context.WithoutCancel(r.Context())
	go func() {
		failed := 0
		for _, id := range ids {
			// Read each one fresh: an earlier apply on the same page can
			// move the statement or leave it gone.
			p, err := s.pg.GetProposal(ctx, id)
			if err == nil && !bulkApplicable(p) {
				err = fmt.Errorf("no longer applicable (page %s, statement on page: %v)", p.TargetStatus, p.OnPage())
			}
			if err == nil {
				err = s.applyApproval(ctx, p, "", by)
			}
			b.mu.Lock()
			b.done++
			if err != nil {
				failed++
				b.failed[id] = err.Error()
			}
			b.mu.Unlock()
		}
		b.mu.Lock()
		b.running = false
		b.mu.Unlock()
		s.log.Info("bulk apply done", slog.Int("total", len(ids)), slog.Int("failed", failed))
	}()
	back(fmt.Sprintf("Applying %d selected. Refresh to see progress; any that fail stay in the list with the reason.", len(ids)), "")
}
