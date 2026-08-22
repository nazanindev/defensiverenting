// Package discover proposes candidate primary sources for a jurisdiction so the
// author can triage them into the citable sources table. Discovery only ever
// populates the author's research shelf — a candidate never becomes a statement
// or citation automatically, which preserves the citation guarantee (ADR-003).
//
// Phase 1 is registry-only (curated seeds). Future web-search providers
// implement the same Provider interface and drop into Run.
package discover

import (
	"net/url"
	"sort"
	"strings"
)

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

// ReferenceOnlyDomains are sites the pipeline must never treat as sources:
// lawyer marketing, legal content mills, and landlord-industry blogs. The
// standing policy (2026-08-21, after a law firm's blog reached three live
// pages as "editorial guidance"): these are research input only. The agent
// may read them via fetch_source to orient and to compare its own copy, but
// discovery never surfaces them, UpsertSource refuses to create rows for
// them, and save_draft_playbook rejects citations to them. The fix is always
// the same: find the primary law the page summarizes and cite that.
var ReferenceOnlyDomains = []string{
	"nolo.com", "justia.com", "findlaw.com", "avvo.com", "legalzoom.com",
	"rocketlawyer.com", "lawyers.com", "legalmatch.com", "superlawyers.com",
	"hg.org", "freeadvice.com", "enjuris.com", "upcounsel.com",
	"apartments.com", "apartmentlist.com", "zillow.com", "rent.com",
	"avail.co", "turbotenant.com", "rentprep.com", "ipropertymanagement.com",
	"doorloop.com", "steadily.com", "landlordology.com",
}

// ReferenceOnly reports whether rawURL is on a reference-only domain,
// matching subdomains too (blog.nolo.com) but never suffix lookalikes
// (notnolo.com).
func ReferenceOnly(rawURL string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Host == "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, d := range ReferenceOnlyDomains {
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}

// Run executes every provider for the jurisdiction, deduplicates by URL keeping
// the highest-confidence hit, and returns candidates sorted by confidence desc
// (URL as a stable tiebreaker). Reference-only domains are dropped before
// anything downstream can triage them into sources.
func Run(slug string, providers ...Provider) []Candidate {
	best := map[string]Candidate{}
	for _, p := range providers {
		for _, c := range p.Discover(slug) {
			if ReferenceOnly(c.URL) {
				continue
			}
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
