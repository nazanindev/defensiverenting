package handlers_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/nazanindev/defensiverenting/internal/http/handlers"
	"github.com/nazanindev/defensiverenting/internal/store"
)

// accountStub is an in-memory stand-in for the account tables. Tokens are
// keyed by hash, exactly as the real store sees them.
type accountStub struct {
	tokens   map[string]stubToken
	sessions map[string]int64
	users    map[string]int64 // email -> id
	location map[int64]string // user id -> slug
	places   map[string]store.Jurisdiction
	nextID   int64
}

type stubToken struct {
	email   string
	expires time.Time
	used    bool
}

func newAccountStub() *accountStub {
	return &accountStub{
		tokens: map[string]stubToken{}, sessions: map[string]int64{}, users: map[string]int64{},
		location: map[int64]string{},
		places: map[string]store.Jurisdiction{
			"boston": {ID: 1, Kind: "city", Name: "Boston", Slug: "boston"},
		},
	}
}

func (s *accountStub) CreateLoginToken(_ context.Context, hash, email string, expires time.Time) error {
	s.tokens[hash] = stubToken{email: email, expires: expires}
	return nil
}

func (s *accountStub) CountRecentLoginTokens(_ context.Context, email string, _ time.Time) (int, error) {
	n := 0
	for _, t := range s.tokens {
		if t.email == email {
			n++
		}
	}
	return n, nil
}

func (s *accountStub) ConsumeLoginToken(_ context.Context, hash string, now time.Time) (int64, error) {
	t, ok := s.tokens[hash]
	if !ok || t.used || !t.expires.After(now) {
		return 0, store.ErrNotFound
	}
	t.used = true
	s.tokens[hash] = t
	id, ok := s.users[t.email]
	if !ok {
		s.nextID++
		id = s.nextID
		s.users[t.email] = id
	}
	return id, nil
}

func (s *accountStub) CreateSession(_ context.Context, hash string, userID int64, _ time.Time) error {
	s.sessions[hash] = userID
	return nil
}

func (s *accountStub) GetAccountBySession(_ context.Context, hash string, _ time.Time) (store.Account, error) {
	id, ok := s.sessions[hash]
	if !ok {
		return store.Account{}, store.ErrNotFound
	}
	a := store.Account{UserID: id}
	for email, uid := range s.users {
		if uid == id {
			a.Email = email
		}
	}
	if slug := s.location[id]; slug != "" {
		a.Location = s.places[slug]
	}
	return a, nil
}

func (s *accountStub) DeleteSession(_ context.Context, hash string) error {
	delete(s.sessions, hash)
	return nil
}

func (s *accountStub) SetUserLocation(_ context.Context, userID int64, slug string) error {
	if slug != "" {
		if _, ok := s.places[slug]; !ok {
			return store.ErrNotFound
		}
	}
	s.location[userID] = slug
	return nil
}

type stubMailer struct{ to, text string }

func (m *stubMailer) Send(_ context.Context, to, _ string, text string) error {
	m.to, m.text = to, text
	return nil
}

type accountFixture struct {
	router http.Handler
	store  *accountStub
	mail   *stubMailer
	now    time.Time
}

func newAccountFixture(t *testing.T) *accountFixture {
	t.Helper()
	f := &accountFixture{store: newAccountStub(), mail: &stubMailer{}, now: time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)}
	r := chi.NewRouter()
	handlers.Account(r, f.store, logger(), handlers.AccountConfig{
		SiteURL: "https://renterlaw.test",
		Mailer:  f.mail,
		Now:     func() time.Time { return f.now },
	})
	f.router = r
	return f
}

func (f *accountFixture) do(req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	return rec
}

func (f *accountFixture) postForm(path string, form string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	return f.do(req)
}

var linkRe = regexp.MustCompile(`https://renterlaw\.test/account/verify\?t=[A-Za-z0-9_-]+`)

// signIn walks the whole flow and returns the session cookie.
func (f *accountFixture) signIn(t *testing.T, email string) *http.Cookie {
	t.Helper()
	rec := f.postForm("/account/signin", "email="+email)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/account?notice=sent" {
		t.Fatalf("signin: %d -> %q", rec.Code, rec.Header().Get("Location"))
	}
	link := linkRe.FindString(f.mail.text)
	if link == "" {
		t.Fatalf("no sign-in link in mail: %q", f.mail.text)
	}
	rec = f.do(httptest.NewRequest(http.MethodGet, strings.TrimPrefix(link, "https://renterlaw.test"), nil))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/account" {
		t.Fatalf("verify: %d -> %q", rec.Code, rec.Header().Get("Location"))
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == "rl_session" {
			if !c.HttpOnly {
				t.Error("session cookie is not HttpOnly")
			}
			return c
		}
	}
	t.Fatal("verify set no session cookie")
	return nil
}

// The link is the credential. It must go only to the typed address, be
// hashed at rest, and work exactly once.
func TestAccount_magicLinkSignsInOnce(t *testing.T) {
	f := newAccountFixture(t)
	c := f.signIn(t, "Renter%40Example.org")

	if f.mail.to != "renter@example.org" {
		t.Errorf("mail sent to %q, want lower-cased address", f.mail.to)
	}
	link := linkRe.FindString(f.mail.text)
	raw := strings.TrimPrefix(link, "https://renterlaw.test/account/verify?t=")
	if _, stored := f.store.tokens[raw]; stored {
		t.Error("raw token stored; expected only its hash")
	}

	// Second use of the same link fails.
	rec := f.do(httptest.NewRequest(http.MethodGet, strings.TrimPrefix(link, "https://renterlaw.test"), nil))
	if rec.Header().Get("Location") != "/account?notice=badlink" {
		t.Errorf("reused link -> %q, want badlink", rec.Header().Get("Location"))
	}

	// The session resolves to the account.
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.AddCookie(c)
	rec = f.do(req)
	var me struct {
		SignedIn bool `json:"signed_in"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&me)
	if !me.SignedIn {
		t.Errorf("/api/me after sign-in = %s", rec.Body.String())
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Errorf("/api/me Cache-Control = %q, must never be shared-cached", cc)
	}
}

func TestAccount_expiredLinkIsRejected(t *testing.T) {
	f := newAccountFixture(t)
	f.postForm("/account/signin", "email=a%40example.org")
	link := strings.TrimPrefix(linkRe.FindString(f.mail.text), "https://renterlaw.test")
	f.now = f.now.Add(16 * time.Minute)
	rec := f.do(httptest.NewRequest(http.MethodGet, link, nil))
	if rec.Header().Get("Location") != "/account?notice=badlink" {
		t.Errorf("expired link -> %q", rec.Header().Get("Location"))
	}
}

// Location round-trips through the API, is validated against real places,
// and needs both a session and a same-origin JSON request.
func TestAccount_locationPersists(t *testing.T) {
	f := newAccountFixture(t)
	c := f.signIn(t, "a%40example.org")

	put := func(body string, withCookie, sameOrigin bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPut, "/api/me/location", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if sameOrigin {
			req.Header.Set("Sec-Fetch-Site", "same-origin")
		} else {
			req.Header.Set("Sec-Fetch-Site", "cross-site")
		}
		if withCookie {
			req.AddCookie(c)
		}
		return f.do(req)
	}

	if rec := put(`{"location":"boston"}`, false, true); rec.Code != http.StatusUnauthorized {
		t.Errorf("anonymous PUT = %d, want 401", rec.Code)
	}
	if rec := put(`{"location":"boston"}`, true, false); rec.Code != http.StatusForbidden {
		t.Errorf("cross-site PUT = %d, want 403", rec.Code)
	}
	if rec := put(`{"location":"atlantis"}`, true, true); rec.Code != http.StatusBadRequest {
		t.Errorf("unknown place PUT = %d, want 400", rec.Code)
	}
	if rec := put(`{"location":"boston"}`, true, true); rec.Code != http.StatusNoContent {
		t.Errorf("PUT = %d, want 204: %s", rec.Code, rec.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.AddCookie(c)
	body, _ := io.ReadAll(f.do(req).Body)
	if !strings.Contains(string(body), `"location":"boston"`) {
		t.Errorf("/api/me = %s", body)
	}

	req = httptest.NewRequest(http.MethodGet, "/account", nil)
	req.AddCookie(c)
	rec := f.do(req)
	if page := rec.Body.String(); !strings.Contains(page, "Boston") || !strings.Contains(page, "a@example.org") {
		t.Errorf("/account page missing email or place")
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Errorf("/account Cache-Control = %q", cc)
	}
}

func TestAccount_signOutClearsSession(t *testing.T) {
	f := newAccountFixture(t)
	c := f.signIn(t, "a%40example.org")
	rec := f.postForm("/account/signout", "", c)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("signout = %d", rec.Code)
	}
	cleared := 0
	for _, ck := range rec.Result().Cookies() {
		if (ck.Name == "rl_session" || ck.Name == "rl_in") && ck.MaxAge < 0 {
			cleared++
		}
	}
	if cleared != 2 {
		t.Errorf("cleared %d cookies, want both", cleared)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.AddCookie(c)
	if body, _ := io.ReadAll(f.do(req).Body); !strings.Contains(string(body), `"signed_in":false`) {
		t.Errorf("session still valid after sign-out: %s", body)
	}
}

// The form is the only way to make this server send email. It must refuse
// bad addresses and stop mailing an address that is being hammered, without
// telling the sender which happened.
func TestAccount_signInLimitsAndValidates(t *testing.T) {
	f := newAccountFixture(t)
	if rec := f.postForm("/account/signin", "email=not-an-address"); rec.Header().Get("Location") != "/account?notice=bademail" {
		t.Errorf("bad address -> %q", rec.Header().Get("Location"))
	}
	if rec := f.postForm("/account/signin", "email=%22Renter%22+%3Ca%40example.org%3E"); rec.Header().Get("Location") != "/account?notice=bademail" {
		t.Errorf("display-name form accepted: %q", rec.Header().Get("Location"))
	}
	for i := 0; i < 5; i++ {
		f.postForm("/account/signin", "email=a%40example.org")
	}
	f.mail.text = ""
	rec := f.postForm("/account/signin", "email=a%40example.org")
	if rec.Header().Get("Location") != "/account?notice=sent" {
		t.Errorf("rate-limited attempt -> %q, must look like success", rec.Header().Get("Location"))
	}
	if f.mail.text != "" {
		t.Error("sixth link in an hour was mailed")
	}

	req := httptest.NewRequest(http.MethodPost, "/account/signin", strings.NewReader("email=b%40example.org"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	if rec := f.do(req); rec.Code != http.StatusForbidden {
		t.Errorf("cross-site signin = %d, want 403", rec.Code)
	}
}
