package main

import (
	"context"
	"net/http"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// The coverage page: the two matrices that answer "who is missing what" —
// core topics by city, and statement concepts by place (ADR-011 D3) — with
// the counts that frame them. It used to sit on the dashboard above the page
// list, where its width pushed the list a screen down for everyone who came
// to review a draft, so it has its own page.
func (s *srv) coverage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	coverage, err := s.pg.AuthorCoverage(ctx)
	if err != nil {
		s.serverError(w, err)
		return
	}
	conceptCoverage, conceptPlaces, err := s.pg.ConceptCoverage(ctx)
	if err != nil {
		s.serverError(w, err)
		return
	}
	coreTopics, err := s.pg.ListCoreTopics(ctx)
	if err != nil {
		s.serverError(w, err)
		return
	}
	playbooks, err := s.pg.AuthorListPlaybooks(ctx)
	if err != nil {
		s.serverError(w, err)
		return
	}
	draftIssues, err := s.pg.AuthorDraftIssues(ctx)
	if err != nil {
		s.serverError(w, err)
		return
	}
	proposals, err := s.pendingProposalCount(ctx)
	if err != nil {
		s.serverError(w, err)
		return
	}
	// Drafts in a deferred language (ADR-015 D3) have no reviewer, so they
	// are not "to review" here any more than on the dashboard.
	drafts := 0
	for _, p := range playbooks {
		if p.Status == "draft" && store.LanguageActive(p.Language) {
			drafts++
		}
	}
	s.render(w, "coverage.html", map[string]any{
		"Actor":           actor(r),
		"Coverage":        coverage,
		"ConceptCoverage": conceptCoverage,
		"ConceptPlaces":   conceptPlaces,
		"CoreTopics":      coreTopics,
		"DraftCount":      drafts,
		"IssueCount":      len(draftIssues),
		"Proposals":       proposals,
	})
}

// pendingProposalCount is what "N proposals waiting" means everywhere it is
// shown: statement proposals and source proposals together, since one queue
// page holds both.
func (s *srv) pendingProposalCount(ctx context.Context) (int, error) {
	stmts, err := s.pg.CountPendingProposals(ctx)
	if err != nil {
		return 0, err
	}
	srcs, err := s.pg.CountPendingSourceProposals(ctx)
	if err != nil {
		return 0, err
	}
	return stmts + srcs, nil
}
