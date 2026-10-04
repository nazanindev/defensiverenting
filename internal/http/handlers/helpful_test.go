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

func (f *fakeHelpful) RecordHelpful(_ context.Context, id int64, helpful bool) (bool, error) {
	if id != 7 {
		return false, nil
	}
	f.got = append(f.got, helpful)
	return true, nil
}

func TestHelpful_countsAValidAnswerOnly(t *testing.T) {
	db := &fakeHelpful{}
	h := Helpful(db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	post := func(body string) int {
		req := httptest.NewRequest(http.MethodPost, HelpfulPath, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		h(rec, req)
		if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
			t.Errorf("%q: Cache-Control = %q, want no-store", body, cc)
		}
		return rec.Code
	}

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
		if got := post(c.body); got != c.want {
			t.Errorf("%q: status %d, want %d", c.body, got, c.want)
		}
	}
	if len(db.got) != 2 || !db.got[0] || db.got[1] {
		t.Fatalf("counted %v, want [true false]", db.got)
	}
}
