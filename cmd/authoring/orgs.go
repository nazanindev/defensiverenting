package main

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// The Orgs page (ADR-029 D4): one row per org a page cites, with its type,
// its contact status, and every email and call logged against it. The editor
// contacts orgs outside the site and records each attempt here; a
// contact-first org shows on the live site only once an attempt says yes.

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
	now := time.Now()
	var waiting, listed int
	for _, o := range orgs {
		if o.Type != store.OrgContactFirst {
			continue
		}
		if o.Status == store.OrgOK {
			listed++
		} else if o.Status != store.OrgLeaveOff {
			waiting++
		}
	}
	s.render(w, "orgs.html", map[string]any{
		"Actor":   actor(r),
		"Orgs":    orgs,
		"Now":     now,
		"Today":   now.Format("2006-01-02"),
		"Hiding":  hiding,
		"Waiting": waiting,
		"Listed":  listed,
		"Msg":     r.URL.Query().Get("msg"),
		"Err":     r.URL.Query().Get("err"),
	})
}

// orgsBack returns to the org's row with a message.
func orgsBack(w http.ResponseWriter, r *http.Request, host, key, msg string) {
	http.Redirect(w, r, "/orgs?"+key+"="+url.QueryEscape(msg)+"#"+url.PathEscape(host), http.StatusSeeOther)
}

func (s *srv) orgContact(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	host := r.PostForm.Get("host")
	c := store.HelpOrgContact{
		Method:   r.PostForm.Get("method"),
		Reached:  r.PostForm.Get("reached"),
		Outcome:  r.PostForm.Get("outcome"),
		Note:     r.PostForm.Get("note"),
		LoggedBy: actor(r),
	}
	if d := strings.TrimSpace(r.PostForm.Get("follow_up")); d != "" {
		t, err := time.Parse("2006-01-02", d)
		if err != nil {
			orgsBack(w, r, host, "err", "The follow-up date is not a date.")
			return
		}
		c.FollowUp = &t
	}
	if c.LoggedBy == "" {
		c.LoggedBy = "portal"
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
	by := actor(r)
	if by == "" {
		by = "portal"
	}
	err := s.pg.SetHelpOrgType(r.Context(), host, typ, by)
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
