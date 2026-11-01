package main

import (
	"net/http"
	"strings"
	"time"

	"github.com/nazanindev/defensiverenting/internal/sitecopy"
	"github.com/nazanindev/defensiverenting/internal/store"
)

// Site copy screens. The about, editorial standards, privacy and support
// pages are text the editor should be able to fix without a code change. The
// list shows each page and who last edited it; the edit screen is three
// fields and one Save. A save runs sitecopy.Check (voice rules + spelling)
// and is refused, with the reasons listed, when the text breaks one. What is
// saved is live on the next page load; the public cache holds for 5 minutes.

// copyRow is one page on the list.
type copyRow struct {
	sitecopy.Info
	Title     string
	UpdatedBy string
	UpdatedAt time.Time
	Edited    bool
}

func (s *srv) copyList(w http.ResponseWriter, r *http.Request) {
	saved, err := s.pg.ListSitePages(r.Context())
	if err != nil {
		s.serverError(w, err)
		return
	}
	var rows []copyRow
	for _, info := range sitecopy.Pages() {
		row := copyRow{Info: info, Title: sitecopy.Default(info.Slug).Title}
		if p, ok := saved[info.Slug]; ok {
			row.Title, row.UpdatedBy, row.UpdatedAt, row.Edited = p.Title, p.UpdatedBy, p.UpdatedAt, true
		}
		rows = append(rows, row)
	}
	s.render(w, "copy.html", map[string]any{"Rows": rows, "SiteURL": s.siteURL})
}

// copyForm is the edit screen's data: the page as it will be saved, plus
// what to say about it.
type copyForm struct {
	sitecopy.Info
	Page      sitecopy.Page
	Problems  []string
	Saved     bool
	UpdatedBy string
	UpdatedAt time.Time
	SiteURL   string
}

func (s *srv) copyEdit(w http.ResponseWriter, r *http.Request) {
	info, ok := sitecopy.Lookup(r.PathValue("slug"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	f := copyForm{Info: info, Page: sitecopy.Default(info.Slug), Saved: r.URL.Query().Get("saved") == "1", SiteURL: s.siteURL}
	if p, found, err := s.pg.GetSitePage(r.Context(), info.Slug); err != nil {
		s.serverError(w, err)
		return
	} else if found {
		f.Page = sitecopy.Page{Slug: info.Slug, Title: p.Title, Description: p.Description, Body: p.Body}
		f.UpdatedBy, f.UpdatedAt = p.UpdatedBy, p.UpdatedAt
	}
	s.render(w, "copyedit.html", f)
}

func (s *srv) copySave(w http.ResponseWriter, r *http.Request) {
	info, ok := sitecopy.Lookup(r.PathValue("slug"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	page := sitecopy.Page{
		Slug:        info.Slug,
		Title:       strings.TrimSpace(r.FormValue("title")),
		Description: strings.TrimSpace(r.FormValue("description")),
		// Browsers post CRLF; keep the stored text LF so diffs and history
		// read cleanly.
		Body: strings.TrimSpace(strings.ReplaceAll(r.FormValue("body"), "\r\n", "\n")),
	}
	if problems := sitecopy.Check(page); len(problems) > 0 {
		w.WriteHeader(http.StatusUnprocessableEntity)
		s.render(w, "copyedit.html", copyForm{Info: info, Page: page, Problems: problems, SiteURL: s.siteURL})
		return
	}
	err := s.pg.SaveSitePage(r.Context(), store.SitePage{
		Slug: page.Slug, Title: page.Title, Description: page.Description, Body: page.Body, UpdatedBy: actor(r),
	})
	if err != nil {
		s.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/copy/"+info.Slug+"?saved=1", http.StatusSeeOther)
}
