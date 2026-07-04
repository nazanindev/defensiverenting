// Package discover proposes candidate primary sources for a jurisdiction so the
// author can triage them into the citable sources table. Discovery only ever
// populates the author's research shelf — a candidate never becomes a statement
// or citation automatically, which preserves the citation guarantee (ADR-003).
//
// Phase 1 is registry-only (curated seeds). Future web-search providers
// implement the same Provider interface and drop into Run.
package discover

import "sort"

// Candidate is a proposed source surfaced for author review.
type Candidate struct {
	URL        string
	Publisher  string
	Title      string
	KindGuess  string  // mirrors the sources.kind enum
	Rationale  string  // one line: why this source is relevant
	Confidence float64 // 0..1, ranks the review queue
	Via        string  // how it was found: "registry" (Phase 1) | "search" (future)
}

// Provider surfaces candidate sources for a jurisdiction identified by its slug.
type Provider interface {
	Discover(slug string) []Candidate
}

// DefaultProviders is the registry-backed provider set used in Phase 1.
func DefaultProviders() []Provider {
	return []Provider{seedProvider{}}
}

// Run executes every provider for the jurisdiction, deduplicates by URL keeping
// the highest-confidence hit, and returns candidates sorted by confidence desc
// (URL as a stable tiebreaker).
func Run(slug string, providers ...Provider) []Candidate {
	best := map[string]Candidate{}
	for _, p := range providers {
		for _, c := range p.Discover(slug) {
			if existing, ok := best[c.URL]; !ok || c.Confidence > existing.Confidence {
				best[c.URL] = c
			}
		}
	}
	out := make([]Candidate, 0, len(best))
	for _, c := range best {
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Confidence != out[j].Confidence {
			return out[i].Confidence > out[j].Confidence
		}
		return out[i].URL < out[j].URL
	})
	return out
}
