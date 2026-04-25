// cmd/ingest parses content/*.md files and loads them into Postgres.
// Run: ingest -content ./content -db $DATABASE_URL
//
// Exit codes: 0 = success, 1 = configuration or parse error, 2 = DB error.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	dbpkg "github.com/nazanin212/bostontenantsrights/db"
	"github.com/nazanin212/bostontenantsrights/internal/content"
	"github.com/nazanin212/bostontenantsrights/internal/store"
)

func main() {
	contentDir := flag.String("content", "./content", "root directory of content markdown files")
	dsn := flag.String("db", os.Getenv("DATABASE_URL"), "Postgres DSN")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if *dsn == "" {
		logger.Error("DATABASE_URL or -db flag is required")
		os.Exit(1)
	}

	ctx := context.Background()
	pg, err := store.New(ctx, *dsn)
	if err != nil {
		logger.Error("connect to database", slog.Any("err", err))
		os.Exit(2)
	}
	defer pg.Close()

	if err := dbpkg.Migrate(ctx, pg.Pool()); err != nil {
		logger.Error("migrate", slog.Any("err", err))
		os.Exit(2)
	}

	editorialSrc, err := pg.GetEditorialSource(ctx)
	if err != nil {
		logger.Error("get editorial source — ensure migration ran", slog.Any("err", err))
		os.Exit(2)
	}

	files, err := filepath.Glob(filepath.Join(*contentDir, "**", "*.md"))
	if err != nil {
		logger.Error("glob content files", slog.Any("err", err))
		os.Exit(1)
	}
	// Also match top-level *.md
	topLevel, _ := filepath.Glob(filepath.Join(*contentDir, "*.md"))
	files = append(files, topLevel...)

	if len(files) == 0 {
		logger.Warn("no .md files found", slog.String("dir", *contentDir))
		os.Exit(0)
	}

	var errCount int
	for _, path := range files {
		if err := ingestFile(ctx, pg, path, editorialSrc.ID, logger); err != nil {
			logger.Error("ingest file", slog.String("path", path), slog.Any("err", err))
			errCount++
		}
	}

	if errCount > 0 {
		logger.Error("ingest completed with errors", slog.Int("errors", errCount))
		os.Exit(1)
	}
	logger.Info("ingest complete", slog.Int("files", len(files)))
}

func ingestFile(ctx context.Context, pg *store.PG, path string, editorialSourceID int64, logger *slog.Logger) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	pb, err := content.Parse(data)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	lang := pb.Language
	if lang == "" {
		lang = "en"
	}

	// Upsert jurisdiction (the migration seeds US/MA/Boston; this handles new cities)
	j, err := pg.UpsertJurisdiction(ctx, store.UpsertJurisdictionParams{
		Kind: "city",
		Name: toTitle(pb.Jurisdiction),
		Slug: pb.Jurisdiction,
	})
	if err != nil {
		return fmt.Errorf("upsert jurisdiction %s: %w", pb.Jurisdiction, err)
	}

	// Build source-id → store.Source map
	sourceMap := make(map[string]store.Source, len(pb.Sources)+1)
	for _, s := range pb.Sources {
		var jID *int64
		if s.Jurisdiction != "" {
			// Prefer looking up the seeded jurisdiction rather than upserting with wrong kind.
			sj, err := pg.GetJurisdictionBySlug(ctx, s.Jurisdiction)
			if err == nil {
				jID = &sj.ID
			}
		}
		src, err := pg.UpsertSource(ctx, store.UpsertSourceParams{
			URL:            s.URL,
			Publisher:      s.Publisher,
			JurisdictionID: jID,
			Kind:           s.Kind,
		})
		if err != nil {
			return fmt.Errorf("upsert source %s: %w", s.ID, err)
		}
		sourceMap[s.ID] = src
	}
	// editorial always resolves to the seeded editorial source
	sourceMap["editorial"] = store.Source{ID: editorialSourceID}

	// Upsert topic
	topicName := toTitle(pb.Topic)
	topic, err := pg.UpsertTopic(ctx, store.UpsertTopicParams{
		Slug: pb.Topic,
		Name: topicName,
	})
	if err != nil {
		return fmt.Errorf("upsert topic %s: %w", pb.Topic, err)
	}

	// Resolve statements to ingest params
	statements := make([]store.IngestStatementParams, 0, len(pb.Statements))
	for i, s := range pb.Statements {
		citations := make([]store.IngestCitationParams, 0, len(s.Citations))
		for _, c := range s.Citations {
			src, ok := sourceMap[c.SourceID]
			if !ok {
				return fmt.Errorf("statement %d references unknown source %q", i, c.SourceID)
			}
			locator := c.Locator
			if locator == "" {
				// Fall back to the source-level default locator
				for _, fs := range pb.Sources {
					if fs.ID == c.SourceID {
						locator = fs.Locator
						break
					}
				}
			}
			citations = append(citations, store.IngestCitationParams{
				SourceID: src.ID,
				Locator:  locator,
			})
		}
		statements = append(statements, store.IngestStatementParams{
			BodyMD:   s.Body,
			Language: lang,
			Sources:  citations,
		})
	}

	if err := pg.IngestPlaybook(ctx, store.IngestPlaybookParams{
		JurisdictionID: j.ID,
		TopicID:        topic.ID,
		Language:       lang,
		Slug:           pb.Topic,
		Title:          pb.Title,
		IntroMD:        pb.Intro,
		Statements:     statements,
	}); err != nil {
		return fmt.Errorf("ingest playbook: %w", err)
	}

	logger.Info("ingested", slog.String("file", path), slog.Int("statements", len(statements)))
	return nil
}

func toTitle(slug string) string {
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if len(p) == 0 {
			continue
		}
		runes := []rune(p)
		runes[0] = unicode.ToUpper(runes[0])
		parts[i] = string(runes)
	}
	return strings.Join(parts, " ")
}
