package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
)

// HelpfulPath takes the answer to "Did this page help?" at the foot of a page.
const HelpfulPath = "/helpful"

type helpfulStore interface {
	RecordHelpful(ctx context.Context, playbookID int64, helpful bool, client string) (bool, error)
}

// Helpful counts one yes or no for a published page. The page's script posts
// p=<playbook id>&a=yes|no. Only a post from our own page counts, and the
// store counts each reader once per page per day (see store.RecordHelpful);
// the reader's address is used for that and never stored. The reply carries
// no body, because the script shows its own thanks whatever happens: a reader
// who answered should not see an error about our counter.
func Helpful(db helpfulStore, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if !fromOurPage(r) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<10)
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		id, err := strconv.ParseInt(r.PostForm.Get("p"), 10, 64)
		answer := r.PostForm.Get("a")
		if err != nil || id <= 0 || (answer != "yes" && answer != "no") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		ok, err := db.RecordHelpful(r.Context(), id, answer == "yes", clientKey(r))
		if err != nil {
			logger.ErrorContext(r.Context(), "record helpful", slog.Any("err", err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// fromOurPage accepts only a browser posting from one of our pages. Unlike
// sameOrigin it refuses a request that carries neither header: nothing but
// our own page script has a reason to answer.
func fromOurPage(r *http.Request) bool {
	switch r.Header.Get("Sec-Fetch-Site") {
	case "same-origin":
		return true
	case "":
		return r.Header.Get("Origin") != "" && sameOrigin(r)
	default:
		return false
	}
}
