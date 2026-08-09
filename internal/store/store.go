package store

import (
	"context"

	"github.com/nazanindev/defensiverenting/internal/discover"
)

// Store is the data-access interface for the tenant-rights service.
// All methods are synchronous; the caller supplies context for cancellation/timeout.
type Store interface {
	// Browse
	ListCityJurisdictions(ctx context.Context) ([]Jurisdiction, error)
	ListAuthorableJurisdictions(ctx context.Context) ([]Jurisdiction, error)
	ListPublishedCityJurisdictions(ctx context.Context) ([]Jurisdiction, error)
	GetJurisdictionBySlug(ctx context.Context, slug string) (Jurisdiction, error)
	ListTopicsByJurisdiction(ctx context.Context, jurisdictionID int64, language string) ([]Topic, error)
	GetTopicBySlug(ctx context.Context, slug string) (Topic, error)
	// ListTopicRegistry returns the whole canonical topic set, not just topics
	// already published somewhere. Drafting a new city must see the vocabulary
	// it is expected to reuse.
	ListTopicRegistry(ctx context.Context) ([]Topic, error)
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

	// Source discovery
	InsertCandidates(ctx context.Context, jurisdictionID int64, cands []discover.Candidate) (int, error)
	ListCandidates(ctx context.Context, jurisdictionID int64, status string) ([]SourceCandidate, error)
	GetCandidate(ctx context.Context, id int64) (SourceCandidate, error)
	SetCandidateStatus(ctx context.Context, id int64, status string, sourceID *int64) error
	CandidateCounts(ctx context.Context) ([]CandidateCountRow, error)

	// Source monitoring
	ListCitationsForCheck(ctx context.Context) ([]CitationCheckRow, error)
	CountUncheckableCitations(ctx context.Context) (int, error)
	MarkSourceReviewed(ctx context.Context, id int64, changed bool) error
	ListFlaggedSources(ctx context.Context) ([]Source, error)
	DismissSourceFlag(ctx context.Context, id int64) error

	// Authoring
	AuthorListPlaybooks(ctx context.Context) ([]AuthorPlaybookRow, error)
	AuthorGetPlaybook(ctx context.Context, id int64) (PlaybookWithStatements, error)
	AuthorUpdatePlaybook(ctx context.Context, p AuthorUpdatePlaybookParams) error
	AuthorPublishPlaybook(ctx context.Context, id int64) error
	AuthorDeletePlaybook(ctx context.Context, id int64) error
}

type AuthorUpdatePlaybookParams struct {
	ID             int64
	JurisdictionID int64
	TopicID        int64
	Language       string
	Slug           string
	Title          string
	IntroMD        string
	PageKind       string
	Statements     []IngestStatementParams
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
	PageKind       string // "playbook"|"directory"|"faq"|"checklist"; defaults to "playbook"
	Statements     []IngestStatementParams
	Status         string // "draft" | "published"; defaults to "published" if empty
}

type IngestStatementParams struct {
	BodyMD   string
	Language string
	Sources  []IngestCitationParams
}

// IngestCitationParams pairs a source row (already upserted) with its locator
// and the verbatim source line (quote) backing the citation.
type IngestCitationParams struct {
	SourceID int64
	Locator  string
	Quote    string
}
