package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	mailpkg "github.com/nazanindev/defensiverenting/internal/mail"
	"github.com/nazanindev/defensiverenting/internal/store"
	tmpl "github.com/nazanindev/defensiverenting/web/templates"
)

// Reader accounts (ADR-017): magic-link sign-in, email address only.
//
// Nothing here touches how browse pages render. Those pages are shared-cache
// and must stay identical for every reader; the signed-in state reaches the
// browser only through the uncached JSON at /api/me, which the scope script
// reads after load. Every route in this file sends Cache-Control: no-store.

// accountStore is the slice of the store this feature needs.
type accountStore interface {
	CreateLoginToken(ctx context.Context, tokenHash, email string, expires time.Time) error
	CountRecentLoginTokens(ctx context.Context, email string, since time.Time) (int, error)
	ConsumeLoginToken(ctx context.Context, tokenHash string, now time.Time) (int64, error)
	CreateSession(ctx context.Context, tokenHash string, userID int64, expires time.Time) error
	GetAccountBySession(ctx context.Context, tokenHash string, now time.Time) (store.Account, error)
	DeleteSession(ctx context.Context, tokenHash string) error
	SetUserLocation(ctx context.Context, userID int64, slug string) error
}

const (
	// sessionCookie carries the session secret. HttpOnly: scripts never see it.
	sessionCookie = "rl_session"
	// signedInCookie is a JS-readable flag with no secret in it, so the scope
	// script knows whether /api/me is worth a request. Set and cleared
	// alongside sessionCookie.
	signedInCookie = "rl_in"

	linkTTL    = 15 * time.Minute
	sessionTTL = 90 * 24 * time.Hour

	// A sign-in link is the only email this server sends, which makes the
	// form a way to make us mail anyone. Cap per address and per client.
	maxLinksPerEmailPerHour  = 5
	maxLinksPerClientPerHour = 20
)

// AccountConfig is what the account routes need from the deployment.
type AccountConfig struct {
	SiteURL string
	Mailer  mailpkg.Mailer
	// SecureCookies is off only in development, where there is no TLS.
	SecureCookies bool
	// Now is the clock; tests set it.
	Now func() time.Time
}

// Account mounts the account routes on r. Mount it outside the cached browse
// group: every response here is personal.
func Account(r chi.Router, db accountStore, logger *slog.Logger, cfg AccountConfig) {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	h := &accountHandler{db: db, logger: logger, cfg: cfg, clients: map[string][]time.Time{}}
	r.Group(func(r chi.Router) {
		r.Use(noStore)
		r.Get("/account", h.page)
		r.Post("/account/signin", h.signIn)
		r.Get("/account/verify", h.verify)
		r.Post("/account/signout", h.signOut)
		r.Get("/api/me", h.me)
		r.Put("/api/me/location", h.setLocation)
	})
}

func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, no-store")
		next.ServeHTTP(w, r)
	})
}

type accountHandler struct {
	db     accountStore
	logger *slog.Logger
	cfg    AccountConfig

	mu      sync.Mutex
	clients map[string][]time.Time // sign-in attempts per client IP, last hour
}

// current resolves the session cookie, if any, to an account. A missing or
// bad cookie is simply "not signed in"; a store failure is logged and treated
// the same way, since the page has a sensible signed-out state and the
// alternative is a 500 on every browse page load with a stale cookie.
func (h *accountHandler) current(r *http.Request) (store.Account, bool) {
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return store.Account{}, false
	}
	a, err := h.db.GetAccountBySession(r.Context(), hashToken(c.Value), h.cfg.Now())
	if errors.Is(err, store.ErrNotFound) {
		return store.Account{}, false
	}
	if err != nil {
		h.logger.ErrorContext(r.Context(), "resolve session", slog.Any("err", err))
		return store.Account{}, false
	}
	return a, true
}

// page serves /account: the sign-in form, or the account when signed in.
func (h *accountHandler) page(w http.ResponseWriter, r *http.Request) {
	p := tmpl.AccountPage{Notice: accountNotice(r.URL.Query().Get("notice"))}
	if a, ok := h.current(r); ok {
		p.SignedIn = true
		p.Email = a.Email
		p.LocationName = a.Location.Name
		p.LocationSlug = a.Location.Slug
	}
	render(w, r, http.StatusOK, p)
}

// The redirect after each step carries a code, never text, for the same
// reason the forms Worker does: the query string is reader-controlled.
var accountNotices = map[string]string{
	"sent":      "Check your email for a sign-in link. It works for 15 minutes.",
	"badlink":   "That sign-in link has expired or was already used. Ask for a new one below.",
	"bademail":  "That email address does not look right. Check it and try again.",
	"signedout": "You are signed out.",
}

func accountNotice(code string) string {
	if code == "" {
		return ""
	}
	if msg, ok := accountNotices[code]; ok {
		return msg
	}
	return ""
}

// signIn handles the email form: mint a link, mail it, say "check your email"
// whether or not anything was sent. The response never reveals whether an
// address has an account, and a rate-limited address gets the same page.
func (h *accountHandler) signIn(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	email, ok := normalizeEmail(r.FormValue("email"))
	if !ok {
		http.Redirect(w, r, "/account?notice=bademail", http.StatusSeeOther)
		return
	}
	now := h.cfg.Now()
	if !h.allowClient(clientKey(r), now) {
		http.Redirect(w, r, "/account?notice=sent", http.StatusSeeOther)
		return
	}
	n, err := h.db.CountRecentLoginTokens(r.Context(), email, now.Add(-time.Hour))
	if err != nil {
		h.logger.ErrorContext(r.Context(), "count login tokens", slog.Any("err", err))
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}
	if n >= maxLinksPerEmailPerHour {
		h.logger.WarnContext(r.Context(), "sign-in link rate limited", slog.String("email", email))
		http.Redirect(w, r, "/account?notice=sent", http.StatusSeeOther)
		return
	}

	token := newToken()
	if err := h.db.CreateLoginToken(r.Context(), hashToken(token), email, now.Add(linkTTL)); err != nil {
		h.logger.ErrorContext(r.Context(), "create login token", slog.Any("err", err))
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}
	link := strings.TrimRight(h.cfg.SiteURL, "/") + "/account/verify?t=" + token
	text := "Here is your RenterLaw sign-in link:\n\n" + link + "\n\n" +
		"It works once, for 15 minutes. If you did not ask for it, ignore this email; nothing happens unless the link is opened.\n"
	if err := h.cfg.Mailer.Send(r.Context(), email, "Your RenterLaw sign-in link", text); err != nil {
		h.logger.ErrorContext(r.Context(), "send sign-in link", slog.Any("err", err))
		http.Error(w, "we could not send the email; please try again", http.StatusBadGateway)
		return
	}
	http.Redirect(w, r, "/account?notice=sent", http.StatusSeeOther)
}

// verify handles the link itself. A good token becomes a session cookie; a
// bad one goes back to the form with one message for every kind of bad.
func (h *accountHandler) verify(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("t")
	if token == "" {
		http.Redirect(w, r, "/account?notice=badlink", http.StatusSeeOther)
		return
	}
	now := h.cfg.Now()
	userID, err := h.db.ConsumeLoginToken(r.Context(), hashToken(token), now)
	if errors.Is(err, store.ErrNotFound) {
		http.Redirect(w, r, "/account?notice=badlink", http.StatusSeeOther)
		return
	}
	if err != nil {
		h.logger.ErrorContext(r.Context(), "consume login token", slog.Any("err", err))
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}
	session := newToken()
	if err := h.db.CreateSession(r.Context(), hashToken(session), userID, now.Add(sessionTTL)); err != nil {
		h.logger.ErrorContext(r.Context(), "create session", slog.Any("err", err))
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}
	h.setSessionCookies(w, session, now.Add(sessionTTL))
	http.Redirect(w, r, "/account", http.StatusSeeOther)
}

func (h *accountHandler) signOut(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
		if err := h.db.DeleteSession(r.Context(), hashToken(c.Value)); err != nil {
			h.logger.ErrorContext(r.Context(), "delete session", slog.Any("err", err))
		}
	}
	h.clearSessionCookies(w)
	http.Redirect(w, r, "/account?notice=signedout", http.StatusSeeOther)
}

// meResponse is the whole of what a page learns about the reader.
type meResponse struct {
	SignedIn     bool   `json:"signed_in"`
	Location     string `json:"location,omitempty"`
	LocationName string `json:"location_name,omitempty"`
}

func (h *accountHandler) me(w http.ResponseWriter, r *http.Request) {
	a, ok := h.current(r)
	writeJSON(w, meResponse{SignedIn: ok, Location: a.Location.Slug, LocationName: a.Location.Name})
}

// setLocation stores the picker's choice. Body: {"location": "boston"}; an
// empty slug clears it. The JSON content type plus the same-origin check is
// the CSRF guard: a cross-site form cannot send either.
func (h *accountHandler) setLocation(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) || !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	a, ok := h.current(r)
	if !ok {
		http.Error(w, "not signed in", http.StatusUnauthorized)
		return
	}
	var body struct {
		Location string `json:"location"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	err := h.db.SetUserLocation(r.Context(), a.UserID, strings.TrimSpace(body.Location))
	if errors.Is(err, store.ErrNotFound) {
		http.Error(w, "unknown location", http.StatusBadRequest)
		return
	}
	if err != nil {
		h.logger.ErrorContext(r.Context(), "set user location", slog.Any("err", err))
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- helpers ---------------------------------------------------------------

// Secure is a config value (off only in development, which has no TLS), and
// rl_in is deliberately readable by scripts: it holds no secret. gosec cannot
// see either from the literal.
func (h *accountHandler) setSessionCookies(w http.ResponseWriter, session string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{ // #nosec G124
		Name: sessionCookie, Value: session, Path: "/", Expires: expires,
		HttpOnly: true, Secure: h.cfg.SecureCookies, SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{ // #nosec G124
		Name: signedInCookie, Value: "1", Path: "/", Expires: expires,
		Secure: h.cfg.SecureCookies, SameSite: http.SameSiteLaxMode,
	})
}

func (h *accountHandler) clearSessionCookies(w http.ResponseWriter) {
	for _, name := range []string{sessionCookie, signedInCookie} {
		http.SetCookie(w, &http.Cookie{ // #nosec G124
			Name: name, Value: "", Path: "/", MaxAge: -1,
			HttpOnly: name == sessionCookie, Secure: h.cfg.SecureCookies, SameSite: http.SameSiteLaxMode,
		})
	}
}

// allowClient is the per-client half of the rate limit: an in-memory sliding
// window keyed by IP. It resets on restart, which is fine for a cap whose
// only job is to stop one machine mailing hundreds of strangers.
func (h *accountHandler) allowClient(key string, now time.Time) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	cutoff := now.Add(-time.Hour)
	kept := h.clients[key][:0]
	for _, t := range h.clients[key] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= maxLinksPerClientPerHour {
		h.clients[key] = kept
		return false
	}
	h.clients[key] = append(kept, now)
	// Drop empty keys so the map does not grow with every visitor ever.
	for k, v := range h.clients {
		if len(v) == 0 || !v[len(v)-1].After(cutoff) {
			delete(h.clients, k)
		}
	}
	return true
}

func clientKey(r *http.Request) string {
	if ip := r.Header.Get("Fly-Client-IP"); ip != "" {
		return ip
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i > 0 {
		host = host[:i]
	}
	return host
}

// sameOrigin rejects state changes that did not come from our own pages.
// Browsers send Sec-Fetch-Site on every request; older ones send Origin on
// POSTs. Neither header can be set by a cross-site form.
func sameOrigin(r *http.Request) bool {
	switch r.Header.Get("Sec-Fetch-Site") {
	case "same-origin", "none":
		return true
	case "cross-site", "same-site":
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		// No Origin and no Sec-Fetch-Site: a non-browser client. Cookies are
		// the only credential, and a non-browser has to hold one deliberately.
		return true
	}
	return strings.EqualFold(strings.TrimPrefix(strings.TrimPrefix(origin, "https://"), "http://"), r.Host)
}

func normalizeEmail(raw string) (string, bool) {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" || len(s) > 254 {
		return "", false
	}
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address != s {
		return "", false
	}
	return s, true
}

func newToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}
