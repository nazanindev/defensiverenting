package handlers

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeHelpful struct {
	got []bool
}

func (f *fakeHelpful) RecordHelpful(_ context.Context, id int64, helpful bool, client string) (bool, error) {
	if id != 7 || client == "" {
		return false, nil
	}
	f.got = append(f.got, helpful)
	return true, nil
}

func TestHelpful_countsAValidAnswerOnly(t *testing.T) {
	db := &fakeHelpful{}
	h := Helpful(db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	post := func(body string, headers map[string]string) int {
		req := httptest.NewRequest(http.MethodPost, HelpfulPath, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		h(rec, req)
		if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
			t.Errorf("%q: Cache-Control = %q, want no-store", body, cc)
		}
		return rec.Code
	}
	ours := map[string]string{"Sec-Fetch-Site": "same-origin"}

	cases := []struct {
		body string
		want int
	}{
		{"p=7&a=yes", http.StatusNoContent},
		{"p=7&a=no", http.StatusNoContent},
		{"p=7&a=maybe", http.StatusBadRequest},
		{"p=abc&a=yes", http.StatusBadRequest},
		{"p=-1&a=yes", http.StatusBadRequest},
		{"p=8&a=yes", http.StatusNotFound},
	}
	for _, c := range cases {
		if got := post(c.body, ours); got != c.want {
			t.Errorf("%q: status %d, want %d", c.body, got, c.want)
		}
	}

	// Anything that is not our own page is refused before it is read.
	for _, hdr := range []map[string]string{
		{},
		{"Sec-Fetch-Site": "cross-site"},
		{"Origin": "https://evil.example"},
	} {
		if got := post("p=7&a=yes", hdr); got != http.StatusForbidden {
			t.Errorf("headers %v: status %d, want 403", hdr, got)
		}
	}

	if len(db.got) != 2 || !db.got[0] || db.got[1] {
		t.Fatalf("counted %v, want [true false]", db.got)
	}
}
