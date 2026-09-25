package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// Bulk apply: every pending replacement filed under one reason, applied
// together on draft pages. Each one goes through the same applyApproval a
// single Apply does — live quote checks, the publish gate, the reviewer's
// name — so a bulk apply is many ordinary applies, not a shortcut past them.
// Live pages are left out: a change there reaches renters at once, so it is
// read one item at a time. Drift findings are left out too, because the
// reviewer has to read what the source says now.

// bulkRule is one line at the top of the queue.
type bulkRule struct {
	Reason string
	Name   string // the reason without its "agent-pass:" family
	Count  int
}

// bulkApplicable reports whether an item may be applied without being read
// on its own.
func bulkApplicable(row store.ProposalRow) bool {
	return row.Status == "pending" && row.Proposed != nil && row.OnPage() &&
		row.TargetStatus == "draft" && row.Reason != store.ReasonSourceDrift &&
		row.Reason != store.ReasonReviewerFlag
}

// bulkRules groups the applicable items by reason, largest first.
func bulkRules(items []queueItem) []bulkRule {
	n := map[string]int{}
	for _, it := range items {
		if bulkApplicable(it.ProposalRow) {
			n[it.Reason]++
		}
	}
	out := make([]bulkRule, 0, len(n))
	for r, c := range n {
		_, name, ok := strings.Cut(r, ":")
		if !ok {
			name = r
		}
		out = append(out, bulkRule{Reason: r, Name: name, Count: c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Reason < out[j].Reason
	})
	return out
}

// bulkRun is the one bulk apply in flight, if any, and the errors of the
// last one, keyed by proposal id so a failed item shows why in the queue.
// Held in memory: a restart forgets the errors, and the items stay pending.
type bulkRun struct {
	mu      sync.Mutex
	running bool
	reason  string
	done    int
	total   int
	failed  map[int64]string
}

type bulkStatus struct {
	Reason      string
	Done, Total int
}

func (b *bulkRun) status() *bulkStatus {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.running {
		return nil
	}
	return &bulkStatus{Reason: b.reason, Done: b.done, Total: b.total}
}

func (b *bulkRun) failure(id int64) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.failed[id]
}

func (s *srv) bulkApprove(w http.ResponseWriter, r *http.Request) {
	reason := r.FormValue("reason")
	rows, err := s.pg.ListProposalsByReason(r.Context(), "pending", "")
	if err != nil {
		s.serverError(w, err)
		return
	}
	var ids []int64
	for _, row := range rows {
		if row.Reason == reason && bulkApplicable(row) {
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
		back("", "Nothing under "+reason+" can be applied in bulk.")
		return
	}
	b := s.bulk
	b.mu.Lock()
	if b.running {
		b.mu.Unlock()
		back("", "A bulk apply is already running.")
		return
	}
	b.running, b.reason, b.done, b.total, b.failed = true, reason, 0, len(ids), map[int64]string{}
	b.mu.Unlock()

	by := actor(r)
	go func() {
		ctx := context.Background()
		failed := 0
		for _, id := range ids {
			// Read each one fresh: an earlier apply on the same page can
			// move the statement or leave it gone.
			p, err := s.pg.GetProposal(ctx, id)
			if err == nil && !bulkApplicable(p) {
				err = fmt.Errorf("no longer applicable in bulk (page %s, statement on page: %v)", p.TargetStatus, p.OnPage())
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
		s.log.Info("bulk apply done", slog.String("reason", reason), slog.Int("total", len(ids)), slog.Int("failed", failed))
	}()
	back(fmt.Sprintf("Applying %d under %s. Refresh to see progress; any that fail stay in the list with the reason.", len(ids), reason), "")
}
