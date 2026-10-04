package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
)

// OutPath takes the page script's report of a click on a citation or a tap on
// a phone number (ADR-029 D6). The link itself goes straight to the source, so
// a reader always sees where it leads; the report travels beside it.
const OutPath = "/out"

type clickStore interface {
	RecordClick(ctx context.Context, sourceID int64, client string) (bool, error)
}

// Out counts one click on a source. Same rules as Helpful: only from our own
// page, once per reader per source per day, nothing about the reader kept.
func Out(db clickStore, logger *slog.Logger) http.HandlerFunc {
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
		id, err := strconv.ParseInt(r.PostForm.Get("s"), 10, 64)
		if err != nil || id <= 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if _, err := db.RecordClick(r.Context(), id, clientKey(r)); err != nil {
			logger.ErrorContext(r.Context(), "record click", slog.Any("err", err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
