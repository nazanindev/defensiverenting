package store

import "context"

// Store is the data-access interface for the tenant-rights service.
// All methods are synchronous; the caller supplies context for cancellation/timeout.
type Store interface {
	// Browse
	ListCityJurisdictions(ctx context.Context) ([]Jurisdiction, error)
	GetJurisdictionBySlug(ctx context.Context, slug string) (Jurisdiction, error)
	ListTopicsByJurisdiction(ctx context.Context, jurisdictionID int64, language string) ([]Topic, error)
	GetPlaybook(ctx context.Context, jurisdictionSlug, topicSlug, language string) (PlaybookWithStatements, error)

	// Search
	Search(ctx context.Context, query string, jurisdictionID *int64, language string) ([]SearchResult, error)

	// SEO
	ListSitemapURLs(ctx context.Context) ([]SitemapEntry, error)

	// Health
	Ping(ctx context.Context) error

	// Ingest
	UpsertJurisdiction(ctx context.Context, j UpsertJurisdictionParams) (Jurisdiction, error)
	UpsertSource(ctx context.Context, s UpsertSourceParams) (Source, error)
	UpsertTopic(ctx context.Context, t UpsertTopicParams) (Topic, error)
	IngestPlaybook(ctx context.Context, p IngestPlaybookParams) error
	GetEditorialSource(ctx context.Context) (Source, error)
}

type UpsertJurisdictionParams struct {
	ParentID *int64
	Kind     string
	Name     string
	Slug     string
}

type UpsertSourceParams struct {
	URL            string
	Publisher      string
	JurisdictionID *int64
	Kind           string
}

type UpsertTopicParams struct {
	Slug string
	Name string
}

type IngestPlaybookParams struct {
	JurisdictionID int64
	TopicID        int64
	Language       string
	Slug           string
	Title          string
	IntroMD        string
	Statements     []IngestStatementParams
}

type IngestStatementParams struct {
	BodyMD   string
	Language string
	Sources  []IngestCitationParams
}

// IngestCitationParams pairs a source row (already upserted) with its locator.
type IngestCitationParams struct {
	SourceID int64
	Locator  string
}
