package store

import "time"

type Jurisdiction struct {
	ID       int64
	ParentID *int64
	Kind     string // country|state|city
	Name     string
	Slug     string
	// ParentSlug is the parent's slug, denormalised because every city URL
	// contains its state (/j/{state}/{city}/{topic}). Empty for the country.
	ParentSlug string
	// ParentName is the parent's display name, denormalised alongside ParentSlug
	// so a list of cities can be grouped under state headings without a lookup
	// per row. Empty for the country.
	ParentName string
}

// Path is the public URL for this jurisdiction. Cities sit under their state,
// which is how tenant law is actually layered: state law with local ordinances
// on top. See docs/ADRs/ADR-005 D2.
//
// Every URL on the site is built from this and TopicPath, so the shape lives in
// exactly one place. It used to be spelled out at seventeen call sites across
// handlers and templates, which is how the authoring form ended up previewing a
// slug format that had not existed since the 2026-08-01 cleanup.
//
// A city with no parent falls back to a flat path rather than emitting "/j//x".
// The hierarchy repair means that should not happen; this keeps a data problem
// from becoming a broken link.
func (j Jurisdiction) Path() string {
	if j.Kind == "city" && j.ParentSlug != "" {
		return "/j/" + j.ParentSlug + "/" + j.Slug
	}
	return "/j/" + j.Slug
}

// TopicPath is the public URL for one topic within this jurisdiction.
func (j Jurisdiction) TopicPath(topicSlug string) string {
	return j.Path() + "/" + topicSlug
}

type Source struct {
	ID             int64
	URL            string
	Publisher      string
	JurisdictionID *int64
	Kind           string // statute|regulation|gov_guidance|nonprofit|editorial
	RetrievedAt    *time.Time
	ContentHash    *string
	FlaggedAt      *time.Time // set when a cited quote no longer appears at the source
}

// CitationCheckRow pairs a source with one verbatim quote cited from it, for the
// source-change checker to confirm the quote still appears at the URL.
type CitationCheckRow struct {
	SourceID  int64
	URL       string
	Publisher string
	Quote     string
}

type Statement struct {
	ID             int64
	JurisdictionID int64
	Language       string
	BodyMD         string
	LastReviewedAt *time.Time
	CreatedAt      time.Time
}

type Citation struct {
	StatementID int64
	SourceID    int64
	Locator     string
	Quote       string // verbatim line from the source backing this citation
}

type CitationWithSource struct {
	SourceID   int64
	Locator    string
	Quote      string // verbatim line from the source backing this citation
	SourceURL  string
	Publisher  string
	SourceKind string // statute|regulation|gov_guidance|nonprofit|editorial
}

type Topic struct {
	ID   int64
	Slug string
	Name string
	// IsCore marks a topic every new city is seeded with. Non-core topics are
	// added to a city only where its law justifies one. Populated by the
	// registry queries; zero elsewhere.
	IsCore bool
}

type Playbook struct {
	ID             int64
	JurisdictionID int64
	TopicID        int64
	Language       string
	Slug           string
	Title          string
	IntroMD        string
	Status         string
	PageKind       string // playbook|directory|faq|checklist
	LastReviewedAt *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	PublishedAt    *time.Time
}

// StatementRow is a row returned by the GetPlaybookStatements query.
type StatementRow struct {
	StatementID     int64
	BodyMD          string
	Position        int
	SourceID        int64
	Locator         string
	SourceURL       string
	SourcePublisher string
	SourceKind      string
}

// PlaybookWithStatements is a fully-assembled playbook ready to render.
// CitedStatements is always non-empty when fetched through the store (invariant).
type PlaybookWithStatements struct {
	Playbook
	Jurisdiction Jurisdiction
	Topic        Topic
	Statements   []CitedStatement
}

// CitedStatement is an atomic claim paired with its citation chips.
// Citations must be non-empty — the store constructor enforces this.
type CitedStatement struct {
	ID        int64
	BodyMD    string
	Citations []CitationWithSource
}

type SearchResult struct {
	Type             string // "statement"|"playbook"
	StatementID      *int64
	PlaybookSlug     string
	PlaybookTitle    string
	Snippet          string
	Rank             float64
	JurisdictionSlug string
	// JurisdictionParentSlug carries the state so a result can link to the
	// hierarchical URL without a second query per row.
	JurisdictionParentSlug string
	TopicSlug              string
}

// Path is the URL of the page this result points at.
func (r SearchResult) Path() string {
	j := Jurisdiction{Kind: "city", Slug: r.JurisdictionSlug, ParentSlug: r.JurisdictionParentSlug}
	return j.TopicPath(r.TopicSlug)
}

// SitemapEntry is a single playbook URL row used to build sitemap.xml.
type SitemapEntry struct {
	JurisdictionSlug       string
	JurisdictionParentSlug string
	JurisdictionKind       string
	TopicSlug              string
	LastMod                *time.Time
}

// Path is the URL to emit for this entry. The sitemap must agree with the
// canonical tag on the page, so both are built from Jurisdiction.TopicPath.
func (e SitemapEntry) Path() string {
	j := Jurisdiction{Kind: e.JurisdictionKind, Slug: e.JurisdictionSlug, ParentSlug: e.JurisdictionParentSlug}
	return j.TopicPath(e.TopicSlug)
}

// SourceCandidate is a proposed source awaiting author triage in the discovery
// queue. Approving one creates a real sources row (SourceID is then set); it is
// never turned into a statement automatically.
type SourceCandidate struct {
	ID             int64
	JurisdictionID *int64
	URL            string
	Publisher      string
	Title          string
	KindGuess      string // mirrors Source.Kind
	Rationale      string
	Confidence     float64
	DiscoveredVia  string
	Status         string // pending|approved|rejected|snoozed
	SourceID       *int64
	CreatedAt      time.Time
	ReviewedAt     *time.Time
}

// CandidateCountRow is a per-city pending-candidate tally for the dashboard badge.
type CandidateCountRow struct {
	JurisdictionID   int64
	JurisdictionName string
	JurisdictionSlug string
	PendingCount     int
}

// AuthorPlaybookRow is a summary row shown in the authoring dashboard.
type AuthorPlaybookRow struct {
	ID               int64
	Title            string
	JurisdictionName string
	JurisdictionSlug string
	TopicSlug        string
	Language         string
	Status           string
	PageKind         string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	PublishedAt      *time.Time
	StatementCount   int
	SourceCount      int
	// RevisesPublished marks a draft whose slot already holds a live page.
	// Publishing it replaces that page and retires the old version, which is a
	// different act from publishing a new one.
	RevisesPublished bool
}
