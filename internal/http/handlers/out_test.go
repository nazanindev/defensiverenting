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

type fakeClicks struct{ got []int64 }

func (f *fakeClicks) RecordClick(_ context.Context, id int64, client string) (bool, error) {
	if client == "" {
		return false, nil
	}
	f.got = append(f.got, id)
	return true, nil
}

func TestOut_countsOnlyFromOurPage(t *testing.T) {
	db := &fakeClicks{}
	h := Out(db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	post := func(body string, headers map[string]string) int {
		req := httptest.NewRequest(http.MethodPost, OutPath, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		h(rec, req)
		return rec.Code
	}
	ours := map[string]string{"Sec-Fetch-Site": "same-origin"}

	if got := post("s=12", ours); got != http.StatusNoContent {
		t.Errorf("valid click: %d", got)
	}
	if got := post("s=abc", ours); got != http.StatusBadRequest {
		t.Errorf("bad id: %d", got)
	}
	if got := post("s=12", map[string]string{"Sec-Fetch-Site": "cross-site"}); got != http.StatusForbidden {
		t.Errorf("cross-site: %d", got)
	}
	if got := post("s=12", nil); got != http.StatusForbidden {
		t.Errorf("no browser headers: %d", got)
	}
	if len(db.got) != 1 || db.got[0] != 12 {
		t.Fatalf("counted %v, want [12]", db.got)
	}
}
