package store

import "time"

type Jurisdiction struct {
	ID       int64
	ParentID *int64
	Kind     string // country|state|city
	Name     string
	Slug     string
}

type Source struct {
	ID             int64
	URL            string
	Publisher      string
	JurisdictionID *int64
	Kind           string // statute|regulation|gov_guidance|nonprofit|editorial
	RetrievedAt    *time.Time
	ContentHash    *string
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
}

type CitationWithSource struct {
	SourceID    int64
	Locator     string
	SourceURL   string
	Publisher   string
	SourceKind  string // statute|regulation|gov_guidance|nonprofit|editorial
}

type Topic struct {
	ID   int64
	Slug string
	Name string
}

type Playbook struct {
	ID             int64
	JurisdictionID int64
	TopicID        int64
	Language       string
	Slug           string
	Title          string
	IntroMD        string
	LastReviewedAt *time.Time
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
	TopicSlug        string
}

// SitemapEntry is a single playbook URL row used to build sitemap.xml.
type SitemapEntry struct {
	JurisdictionSlug string
	TopicSlug        string
	LastMod          *time.Time
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
}
