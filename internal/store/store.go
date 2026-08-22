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
	// ListPublishedHubJurisdictions is the hub-page inventory: every
	// jurisdiction of any kind with a published playbook of its own.
	ListPublishedHubJurisdictions(ctx context.Context) ([]Jurisdiction, error)
	GetJurisdictionBySlug(ctx context.Context, slug string) (Jurisdiction, error)
	// GetNearestTopicJurisdiction resolves a location to the closest guide up
	// its ancestor chain: city, else state, else country.
	GetNearestTopicJurisdiction(ctx context.Context, jurisdictionID, topicID int64, language string) (Jurisdiction, error)
	ListTopicsByJurisdiction(ctx context.Context, jurisdictionID int64, language string) ([]Topic, error)
	// ListTopicsByJurisdictionRecursive is the coverage set that same walk
	// resolves for a location: its own topics plus every ancestor's.
	ListTopicsByJurisdictionRecursive(ctx context.Context, jurisdictionID int64, language string) ([]Topic, error)
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
	CitationQuoteExists(ctx context.Context, url, quote string) (bool, error)
	MarkSourceReviewed(ctx context.Context, id int64, changed bool) error
	// MarkQuotesChecked stamps checked_at on every citation of this source
	// whose quote is in quotes — the ones a check run confirmed are still
	// present at the URL. Quotes that went missing are not stamped: their last
	// confirmation stays at the run that last actually saw them.
	MarkQuotesChecked(ctx context.Context, sourceID int64, quotes []string) error
	ListFlaggedSources(ctx context.Context) ([]Source, error)
	ListUnusedSources(ctx context.Context) ([]Source, error)
	DismissSourceFlag(ctx context.Context, id int64) error

	// Concepts (ADR-011) and the reference layer built on them (ADR-012)
	ListConcepts(ctx context.Context) ([]Concept, error)
	ConceptCoverage(ctx context.Context) ([]ConceptCoverageRow, error)
	ConceptHubTopics(ctx context.Context, language string) (map[string]string, error)
	ListTerms(ctx context.Context, language string) ([]Term, error)
	GetConceptPage(ctx context.Context, slug, language string) (ConceptPageData, error)
	ListSourceUsage(ctx context.Context) ([]SourceUsage, error)

	// Authoring
	AuthorListPlaybooks(ctx context.Context) ([]AuthorPlaybookRow, error)
	AuthorCoverage(ctx context.Context) ([]CoverageRow, error)
	AuthorGetPlaybook(ctx context.Context, id int64) (PlaybookWithStatements, error)
	AuthorFindDraft(ctx context.Context, jurisdictionID, topicID int64, language string) (int64, error)
	AuthorUpdatePlaybook(ctx context.Context, p AuthorUpdatePlaybookParams) error
	AuthorPublishPlaybook(ctx context.Context, id int64, actor string) error
	AuthorUnpublishPlaybook(ctx context.Context, id int64, retireExistingDraft bool, actor string) error
	AuthorDeletePlaybook(ctx context.Context, id int64) error
	// Page issues (ADR-013): the invariants publishing enforces, surfaced on
	// drafts so the dashboard can mark what is not yet publishable.
	AuthorPlaybookIssues(ctx context.Context, id int64) ([]PageIssue, error)
	AuthorDraftIssues(ctx context.Context) (map[int64][]PageIssue, error)
}

// Actor names for the non-human write paths, recorded in updated_by and
// checked_by beside the people's first names. Constants so the same path
// cannot stamp two spellings of itself.
const (
	// ActorDraftingAgent marks writes from the AI drafting pipeline — the MCP
	// save_draft_playbook tool and the portal's generate button both save
	// through it.
	ActorDraftingAgent = "drafting agent"
	// ActorSourceCheck marks quote confirmations stamped by the automated
	// check-sources run, as opposed to a person saving or attesting.
	ActorSourceCheck = "source check"
)

type AuthorUpdatePlaybookParams struct {
	ID             int64
	JurisdictionID int64
	TopicID        int64
	Language       string
	Slug           string
	Title          string
	IntroMD        string
	PageKind       string
	AuthorNotes    string
	// UpdatedBy is who is saving: the signed-in person's first name, or an
	// Actor* constant. Overwrites the previous value — "last edited by", not
	// a history.
	UpdatedBy  string
	Statements []IngestStatementParams
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
	// UpdatedBy is who is saving; see AuthorUpdatePlaybookParams.UpdatedBy.
	UpdatedBy string
	// AllowIncomplete captures a person's part-finished draft (ADR-013):
	// statements may lack citations and statute locators go unvalidated, with
	// the gaps surfacing as page issues that block publishing instead of the
	// save. Only valid with Status "draft". The drafting agent and the seeding
	// tools leave it false — their output has no excuse to be incomplete.
	AllowIncomplete bool
}

type IngestStatementParams struct {
	BodyMD   string
	Language string
	// ConceptSlug tags the statement with a registry concept (ADR-011); ""
	// leaves it untagged. An unknown slug fails the save — the registry is
	// closed, and silently dropping a tag would hide the mistake.
	ConceptSlug string
	// TopicRefSlug marks the statement as a whole-topic summary (ADR-011 D7),
	// pointing at a registry topic. Mutually exclusive with ConceptSlug; the
	// save fails when both are set rather than picking one silently.
	TopicRefSlug string
	Sources      []IngestCitationParams
}

// IngestCitationParams pairs a source row (already upserted) with its locator
// and the verbatim source line (quote) backing the citation.
type IngestCitationParams struct {
	SourceID int64
	Locator  string
	Quote    string
	// ManuallyVerified marks a citation a reviewer attested to by hand because
	// the automated fetch could not reach the source at all.
	ManuallyVerified bool
	// CheckedNow says this save confirmed the quote against the source: the
	// drafting guardrail matched it against fetched text, the authoring form
	// fetched the source live, or a reviewer attested to it by hand. When
	// false the insert inherits the newest checked_at already stored for the
	// same (source, quote) — a stored identical quote got there through a
	// verified path, but the verification happened then, not now.
	CheckedNow bool
	// CheckedBy is who confirmed the quote when CheckedNow is set: the person
	// saving, or an Actor* constant. When CheckedNow is false it is ignored —
	// the insert inherits checked_by along with checked_at, naming whoever
	// actually confirmed the quote back then.
	CheckedBy string
}
