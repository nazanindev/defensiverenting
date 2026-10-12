package main

import (
	"context"
	"net/http"
	"sort"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// Two maps share one shape: places down, core topics across, one colored
// cell per pair, and a click goes through to the pages behind the cell.
//
// The coverage map (/coverage) colors a cell by what exists: nothing, a
// draft, a published page. The quality map (/quality) colors a cell by the
// worst ADR-027 check failing on the pages in it. Nazanin, 2026-10-11: a
// master matrix to spot where coverage is lacking, then click through for
// specifics; quality is its own map, not a tint on the coverage one.

// The coverage page.
func (s *srv) coverage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	coverage, err := s.pg.AuthorCoverage(ctx)
	if err != nil {
		s.serverError(w, err)
		return
	}
	coreTopics, err := s.pg.ListCoreTopics(ctx)
	if err != nil {
		s.serverError(w, err)
		return
	}
	// One number: topics a state or the country has nothing on. Cities are
	// not counted: an empty city cell is not a gap (ADR-028 D2).
	missing := 0
	var places, cities []store.CoverageRow
	for _, row := range coverage {
		if row.JurisdictionKind == "city" {
			cities = append(cities, row)
			continue
		}
		places = append(places, row)
		for _, t := range coreTopics {
			if row.Status[t.Slug].State == "" {
				missing++
			}
		}
	}
	s.render(w, "coverage.html", map[string]any{
		"Actor":      actor(r),
		"Places":     places,
		"Cities":     cities,
		"CoreTopics": coreTopics,
		"Missing":    missing,
	})
}

// qualityCell is one place and topic on the quality map.
type qualityCell struct {
	// Worst is the worst severity across the cell's pages; 0 when every
	// page passes, -1 when the cell has no page.
	Worst int
	// Draft says the worst page is a draft, where an agent can still fix it.
	Draft bool
	Pages int
}

// qualityRow is one place on the quality map.
type qualityRow struct {
	Name  string
	Slug  string
	Kind  string
	Cells map[string]qualityCell
}

// qualityCheckCount is one check and how many pages fail it, for the list
// under the map.
type qualityCheckCount struct {
	Code  string
	Name  string
	Pages int
}

// The quality page. Without a filter it is the map; with ?place, ?topic or
// ?check it is the list of pages behind one cell, row, column or check,
// each with its failing checks in plain words.
func (s *srv) quality(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pages, err := s.pg.AuthorQuality(ctx)
	if err != nil {
		s.serverError(w, err)
		return
	}
	coreTopics, err := s.pg.ListCoreTopics(ctx)
	if err != nil {
		s.serverError(w, err)
		return
	}
	topicName := map[string]string{}
	for _, t := range coreTopics {
		topicName[t.Slug] = t.Name
	}

	place, topic, check := r.FormValue("place"), r.FormValue("topic"), r.FormValue("check")
	if _, ok := store.CheckNames[check]; !ok {
		check = ""
	}
	if _, ok := topicName[topic]; !ok {
		topic = ""
	}

	failing := 0
	var rows []qualityRow
	rowIndex := map[string]int{}
	byCheck := map[string]int{}
	placeName := ""
	var listed []store.QualityPage
	for _, p := range pages {
		if p.Worst() > 0 {
			failing++
		}
		seen := map[string]bool{}
		for _, c := range p.Checks {
			if !seen[c.Code] {
				seen[c.Code] = true
				byCheck[c.Code]++
			}
		}
		i, ok := rowIndex[p.JurisdictionSlug]
		if !ok {
			i = len(rows)
			rowIndex[p.JurisdictionSlug] = i
			rows = append(rows, qualityRow{Name: p.JurisdictionName, Slug: p.JurisdictionSlug, Kind: p.JurisdictionKind, Cells: map[string]qualityCell{}})
		}
		cell, ok := rows[i].Cells[p.TopicSlug]
		if !ok {
			cell = qualityCell{Worst: -1}
		}
		cell.Pages++
		if worst := p.Worst(); worst > cell.Worst {
			cell.Worst = worst
			cell.Draft = p.Status == "draft"
		}
		rows[i].Cells[p.TopicSlug] = cell

		if p.JurisdictionSlug == place {
			placeName = p.JurisdictionName
		}
		if (place == "" || p.JurisdictionSlug == place) &&
			(topic == "" || p.TopicSlug == topic) &&
			(check == "" || seen[check]) {
			listed = append(listed, p)
		}
	}
	if place != "" && placeName == "" {
		place = ""
	}
	filtered := place != "" || topic != "" || check != ""
	if filtered {
		// Worst pages first, so the top of the list is the next fix.
		sort.SliceStable(listed, func(i, j int) bool {
			if listed[i].Worst() != listed[j].Worst() {
				return listed[i].Worst() > listed[j].Worst()
			}
			return len(listed[i].Checks) > len(listed[j].Checks)
		})
	}

	var checks []qualityCheckCount
	for code, n := range byCheck {
		checks = append(checks, qualityCheckCount{Code: code, Name: store.CheckNames[code], Pages: n})
	}
	sort.Slice(checks, func(i, j int) bool {
		if checks[i].Pages != checks[j].Pages {
			return checks[i].Pages > checks[j].Pages
		}
		return checks[i].Code < checks[j].Code
	})

	var places, cities []qualityRow
	for _, row := range rows {
		// A cell with no page must read as such, not as a zero value that
		// looks like a pass.
		for _, t := range coreTopics {
			if _, ok := row.Cells[t.Slug]; !ok {
				row.Cells[t.Slug] = qualityCell{Worst: -1}
			}
		}
		if row.Kind == "city" {
			cities = append(cities, row)
		} else {
			places = append(places, row)
		}
	}
	s.render(w, "quality.html", map[string]any{
		"Actor":      actor(r),
		"Places":     places,
		"Cities":     cities,
		"CoreTopics": coreTopics,
		"TopicName":  topicName,
		"Failing":    failing,
		"Judged":     len(pages),
		"Checks":     checks,
		"Filtered":   filtered,
		"Place":      place,
		"PlaceName":  placeName,
		"Topic":      topic,
		"Check":      check,
		"CheckName":  store.CheckNames[check],
		"Listed":     listed,
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
