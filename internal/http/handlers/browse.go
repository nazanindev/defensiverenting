package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/nazanin212/bostontenantsrights/internal/store"
	"github.com/nazanin212/bostontenantsrights/internal/content"
	tmpl "github.com/nazanin212/bostontenantsrights/web/templates"
)

type browseStore interface {
	ListCityJurisdictions(ctx context.Context) ([]store.Jurisdiction, error)
	GetJurisdictionBySlug(ctx context.Context, slug string) (store.Jurisdiction, error)
	ListTopicsByJurisdiction(ctx context.Context, id int64, lang string) ([]store.Topic, error)
	GetPlaybook(ctx context.Context, jurisdictionSlug, topicSlug, language string) (store.PlaybookWithStatements, error)
}

// Browse wires the three browse routes onto a chi.Router.
func Browse(r chi.Router, db browseStore, logger *slog.Logger) {
	r.Get("/", index(db, logger))
	r.Get("/j/{jurisdiction}", jurisdictionIndex(db, logger))
	r.Get("/j/{jurisdiction}/{topic}", playbook(db, logger))
}

func index(db browseStore, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jurisdictions, err := db.ListCityJurisdictions(r.Context())
		if err != nil {
			logger.ErrorContext(r.Context(), "list jurisdictions", slog.Any("err", err))
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		render(w, r, http.StatusOK, tmpl.IndexPage{Jurisdictions: jurisdictions})
	}
}

func jurisdictionIndex(db browseStore, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "jurisdiction")
		j, err := db.GetJurisdictionBySlug(r.Context(), slug)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				http.NotFound(w, r)
				return
			}
			logger.ErrorContext(r.Context(), "get jurisdiction", slog.Any("err", err))
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		topics, err := db.ListTopicsByJurisdiction(r.Context(), j.ID, "en")
		if err != nil {
			logger.ErrorContext(r.Context(), "list topics", slog.Any("err", err))
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		render(w, r, http.StatusOK, tmpl.JurisdictionPage{Jurisdiction: j, Topics: topics})
	}
}

func playbook(db browseStore, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jSlug := chi.URLParam(r, "jurisdiction")
		tSlug := chi.URLParam(r, "topic")

		pb, err := db.GetPlaybook(r.Context(), jSlug, tSlug, "en")
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				http.NotFound(w, r)
				return
			}
			logger.ErrorContext(r.Context(), "get playbook", slog.Any("err", err))
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		// Render markdown intro to HTML
		introHTML := content.RenderMarkdown(pb.IntroMD)

		// Render each statement's body markdown and validate citation invariant
		statements := make([]tmpl.RenderedStatement, 0, len(pb.Statements))
		for _, s := range pb.Statements {
			if len(s.Citations) == 0 {
				// Log the violation and skip rather than panic in production.
				// Development builds should catch this via tests.
				logger.ErrorContext(r.Context(), "statement missing citations — invariant violated",
					slog.Int64("statement_id", s.ID))
				continue
			}
			chips := make([]tmpl.CitationChip, 0, len(s.Citations))
			for _, c := range s.Citations {
				chips = append(chips, tmpl.CitationChip{
					URL:       c.SourceURL + anchorFragment(c.Locator),
					Label:     c.Publisher,
					Locator:   c.Locator,
					SourceKind: c.SourceKind,
				})
			}
			statements = append(statements, tmpl.RenderedStatement{
				BodyHTML:  content.RenderMarkdown(s.BodyMD),
				Citations: chips,
			})
		}

		render(w, r, http.StatusOK, tmpl.PlaybookPage{
			Playbook:   pb.Playbook,
			Jurisdiction: pb.Jurisdiction,
			Topic:      pb.Topic,
			IntroHTML:  introHTML,
			Statements: statements,
		})
	}
}

// anchorFragment returns a URL fragment for a locator if non-empty.
func anchorFragment(locator string) string {
	if locator == "" {
		return ""
	}
	return "#" + locator
}
