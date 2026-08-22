package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParsePortalUsers(t *testing.T) {
	users, err := parsePortalUsers("Nazanin:secret1, Cameron:sec:ret2,")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []portalUser{{name: "Nazanin", pass: "secret1"}, {name: "Cameron", pass: "sec:ret2"}}
	if len(users) != len(want) {
		t.Fatalf("got %d users, want %d", len(users), len(want))
	}
	for i, u := range users {
		if u != want[i] {
			t.Errorf("user %d = %+v, want %+v", i, u, want[i])
		}
	}
	if users, err := parsePortalUsers(""); err != nil || len(users) != 0 {
		t.Errorf("empty env should parse to no users, got %v, %v", users, err)
	}
	// A pair missing its password must be an error, not a silent skip: a
	// login that failed to parse is indistinguishable from a wrong password.
	if _, err := parsePortalUsers("Nazanin"); err == nil {
		t.Error("entry without a colon should be an error")
	}
	if _, err := parsePortalUsers("Nazanin:"); err == nil {
		t.Error("entry with an empty password should be an error")
	}
}

// The middleware must both gate access and identify who got in: the name it
// puts in the context is what every write stamps into updated_by/checked_by,
// so the wrong name here would misattribute edits silently.
func TestBasicAuthIdentifiesActor(t *testing.T) {
	users := []portalUser{{name: "Nazanin", pass: "pw-n"}, {name: "Cameron", pass: "pw-c"}}
	handler := basicAuth(users, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(actor(r)))
	}))

	cases := []struct {
		name, user, pass string
		wantStatus       int
		wantBody         string
	}{
		{"first user", "Nazanin", "pw-n", http.StatusOK, "Nazanin"},
		{"second user", "Cameron", "pw-c", http.StatusOK, "Cameron"},
		{"wrong password", "Nazanin", "pw-c", http.StatusUnauthorized, ""},
		{"unknown user", "intruder", "pw-n", http.StatusUnauthorized, ""},
		{"no credentials", "", "", http.StatusUnauthorized, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.user != "" || tc.pass != "" {
				req.SetBasicAuth(tc.user, tc.pass)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantStatus == http.StatusOK && rec.Body.String() != tc.wantBody {
				t.Errorf("actor = %q, want %q", rec.Body.String(), tc.wantBody)
			}
		})
	}
}
