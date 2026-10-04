package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nazanindev/defensiverenting/internal/store"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

// The Orgs page (ADR-029 D4): a sortable table of every org a page cites,
// with the key facts always in view and a row that opens to what we would
// say, the three answers, their contacts and the contact log. The editor's
// script sits pinned beside it and fills in the open org's details.

//go:embed script/org-contact-script.md
var orgScriptMD []byte

// orgScriptHTML renders the script once. Tables need GFM; the text is ours.
var orgScriptHTML = func() template.HTML {
	var buf bytes.Buffer
	if err := goldmark.New(goldmark.WithExtensions(extension.GFM)).Convert(orgScriptMD, &buf); err != nil {
		return template.HTML("<pre>" + template.HTMLEscapeString(string(orgScriptMD)) + "</pre>")
	}
	//nolint:gosec // The script is a file in this repo, not user input.
	return template.HTML(buf.String())
}()

// orgSortable is the allowlist of Orgs-page sort keys. "" is the work order.
var orgSortable = map[string]bool{
	"org": true, "places": true, "live": true, "clicks": true, "type": true,
	"list": true, "info": true, "followup": true, "last": true,
}

// orgListRank orders "May we list you?" answers from most to least to do.
var orgListRank = map[string]int{
	store.OrgMaybe: 0, store.OrgNotContacted: 1, store.OrgContacted: 2, store.OrgOK: 3, store.OrgLeaveOff: 4,
}

func sortOrgs(orgs []store.HelpOrg, key, dir string) {
	if key == "" {
		return // ListHelpOrgs already returns the work order
	}
	timeOr := func(t *time.Time) int64 {
		if t == nil {
			return 1 << 62
		}
		return t.Unix()
	}
	less := func(a, b store.HelpOrg) bool {
		switch key {
		case "org":
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		case "places":
			return len(a.Places) < len(b.Places)
		case "live":
			return a.LivePages < b.LivePages
		case "clicks":
			return a.Clicks30 < b.Clicks30
		case "type":
			return a.Type < b.Type
		case "list":
			return orgListRank[a.Status] < orgListRank[b.Status]
		case "info":
			return a.InfoCorrect < b.InfoCorrect
		case "followup":
			return timeOr(a.FollowUp) < timeOr(b.FollowUp)
		default: // last
			return timeOr(a.LastContact()) < timeOr(b.LastContact())
		}
	}
	sort.SliceStable(orgs, func(i, j int) bool {
		if dir == "desc" {
			return less(orgs[j], orgs[i])
		}
		return less(orgs[i], orgs[j])
	})
}

// orgsView is the Orgs page's sort state, for the column links.
type orgsView struct{ Sort, Dir string }

func (v orgsView) SortLink(col string) template.URL {
	dir := "asc"
	if v.Sort == col && v.Dir == "asc" {
		dir = "desc"
	}
	return template.URL("/orgs?sort=" + col + "&dir=" + dir)
}

func (v orgsView) Arrow(col string) string {
	if v.Sort != col {
		return ""
	}
	if v.Dir == "desc" {
		return "▼"
	}
	return "▲"
}

// orgFill is what the script sidebar puts in place of its brackets when a
// row is open.
type orgFill struct {
	Name       string   `json:"name"`
	Places     string   `json:"places"`
	Statements []string `json:"statements"`
}

func (s *srv) orgs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgs, err := s.pg.ListHelpOrgs(ctx)
	if err != nil {
		s.serverError(w, err)
		return
	}
	hiding, err := s.pg.HelpHiding(ctx)
	if err != nil {
		s.serverError(w, err)
		return
	}
	view := orgsView{Sort: r.URL.Query().Get("sort"), Dir: r.URL.Query().Get("dir")}
	if !orgSortable[view.Sort] {
		view.Sort = ""
	}
	if view.Dir != "desc" {
		view.Dir = "asc"
	}
	sortOrgs(orgs, view.Sort, view.Dir)

	now := time.Now()
	var waiting, listed int
	fills := map[string]string{}
	for _, o := range orgs {
		if o.Type == store.OrgContactFirst {
			if o.Status == store.OrgOK {
				listed++
			} else if o.Status != store.OrgLeaveOff {
				waiting++
			}
		}
		f := orgFill{Name: o.Name, Places: strings.Join(o.Places, ", ")}
		for _, st := range o.Statements {
			f.Statements = append(f.Statements, st.Body)
		}
		b, _ := json.Marshal(f)
		fills[o.Host] = string(b)
	}
	s.render(w, "orgs.html", map[string]any{
		"Actor":   actor(r),
		"Orgs":    orgs,
		"Fills":   fills,
		"Kinds":   store.HelpOrgChannelKinds,
		"View":    view,
		"Now":     now,
		"Today":   now.Format("2006-01-02"),
		"Hiding":  hiding,
		"Waiting": waiting,
		"Listed":  listed,
		"Script":  orgScriptHTML,
		"Msg":     r.URL.Query().Get("msg"),
		"Err":     r.URL.Query().Get("err"),
	})
}

// orgScript is the script alone, for its own tab.
func (s *srv) orgScript(w http.ResponseWriter, r *http.Request) {
	s.render(w, "orgscript.html", map[string]any{"Script": orgScriptHTML})
}

// orgsBack returns to the org's row, open, with a message.
func orgsBack(w http.ResponseWriter, r *http.Request, host, key, msg string) {
	back := "/orgs?" + key + "=" + url.QueryEscape(msg)
	if ref, err := url.Parse(r.Referer()); err == nil && ref.Path == "/orgs" {
		if q := ref.Query(); orgSortable[q.Get("sort")] {
			back += "&sort=" + url.QueryEscape(q.Get("sort")) + "&dir=" + url.QueryEscape(q.Get("dir"))
		}
	}
	http.Redirect(w, r, back+"#"+url.PathEscape(host), http.StatusSeeOther)
}

func editor(r *http.Request) string {
	if a := actor(r); a != "" {
		return a
	}
	return "portal"
}

func (s *srv) orgContact(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	host := r.PostForm.Get("host")
	c := store.HelpOrgContact{
		Method:      r.PostForm.Get("method"),
		Reached:     r.PostForm.Get("reached"),
		Outcome:     r.PostForm.Get("outcome"),
		Note:        r.PostForm.Get("note"),
		InfoCorrect: r.PostForm.Get("info_correct"),
		InfoChanges: r.PostForm.Get("info_changes"),
		LoggedBy:    editor(r),
	}
	if c.InfoCorrect == "unchanged" {
		c.InfoCorrect = ""
	}
	if d := strings.TrimSpace(r.PostForm.Get("follow_up")); d != "" {
		t, err := time.Parse("2006-01-02", d)
		if err != nil {
			orgsBack(w, r, host, "err", "The follow-up date is not a date.")
			return
		}
		c.FollowUp = &t
	}
	err := s.pg.LogHelpOrgContact(r.Context(), host, c)
	if errors.Is(err, store.ErrNoHelpOrg) {
		http.Error(w, "no such org", http.StatusNotFound)
		return
	}
	if err != nil {
		orgsBack(w, r, host, "err", err.Error())
		return
	}
	orgsBack(w, r, host, "msg", "Logged.")
}

func (s *srv) orgChannel(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	host := r.PostForm.Get("host")
	err := s.pg.AddHelpOrgChannel(r.Context(), host, store.HelpOrgChannel{
		Kind:      r.PostForm.Get("kind"),
		Value:     r.PostForm.Get("value"),
		Label:     r.PostForm.Get("label"),
		Preferred: r.PostForm.Get("preferred") == "on",
		AddedBy:   editor(r),
	})
	if errors.Is(err, store.ErrNoHelpOrg) {
		http.Error(w, "no such org", http.StatusNotFound)
		return
	}
	if err != nil {
		orgsBack(w, r, host, "err", err.Error())
		return
	}
	orgsBack(w, r, host, "msg", "Contact added.")
}

func (s *srv) orgChannelRemove(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	host := r.PostForm.Get("host")
	id, err := strconv.ParseInt(r.PostForm.Get("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := s.pg.RemoveHelpOrgChannel(r.Context(), host, id); err != nil {
		orgsBack(w, r, host, "err", err.Error())
		return
	}
	orgsBack(w, r, host, "msg", "Contact removed.")
}

// orgHiding turns hiding on or off (ADR-029 D5). Turning it on is refused
// while any live Local Help page lacks public help; the refusal names them.
func (s *srv) orgHiding(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	on := r.PostForm.Get("hiding") == "on"
	if err := s.pg.SetHelpHiding(r.Context(), on); err != nil {
		http.Redirect(w, r, "/orgs?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	s.log.Info("help hiding changed", "on", on, "by", actor(r))
	msg := "Hiding is on. Readers no longer see orgs that have not said yes."
	if !on {
		msg = "Hiding is off. Readers see every org again."
	}
	http.Redirect(w, r, "/orgs?msg="+url.QueryEscape(msg), http.StatusSeeOther)
}

func (s *srv) orgType(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	host, typ := r.PostForm.Get("host"), r.PostForm.Get("type")
	err := s.pg.SetHelpOrgType(r.Context(), host, typ, editor(r))
	if errors.Is(err, store.ErrNoHelpOrg) {
		http.Error(w, "no such org", http.StatusNotFound)
		return
	}
	if err != nil {
		orgsBack(w, r, host, "err", err.Error())
		return
	}
	orgsBack(w, r, host, "msg", "Type changed.")
}
