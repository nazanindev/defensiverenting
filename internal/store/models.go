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

// LangPrefix returns the URL path prefix for lang: "" for English (the
// unprefixed default, matching drafting.ResolveLanguage and voice.Supported's
// treatment of "en" as the base case everywhere else), "/es" for Spanish.
// See docs/ADRs/ADR-007. Every URL for language-scoped content is built by
// prepending this to Path()/TopicPath() — via PathIn/TopicPathIn below —
// rather than each call site concatenating it by hand.
func LangPrefix(lang string) string {
	if lang == "" || lang == "en" {
		return ""
	}
	return "/" + lang
}

// PathIn is Path() prefixed for lang: the canonical URL for this
// jurisdiction's hub page in a specific language.
func (j Jurisdiction) PathIn(lang string) string { return LangPrefix(lang) + j.Path() }

// TopicPathIn is TopicPath() prefixed for lang: the canonical URL for one
// topic within this jurisdiction, in a specific language.
func (j Jurisdiction) TopicPathIn(lang, topicSlug string) string {
	return LangPrefix(lang) + j.TopicPath(topicSlug)
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
	// LastCheckedAt is when the checker last fetched this source and examined
	// its cited quotes. RetrievedAt cannot carry that meaning: UpsertSource
	// bumps it on every save without fetching anything.
	LastCheckedAt *time.Time
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
	// ManuallyVerified marks a citation the automated fetch could not check
	// (the source blocked it outright) that a human reviewer attested to
	// instead. Distinguishes an eyeballed citation from a machine-matched one.
	ManuallyVerified bool
	// CheckedAt is when the quote was last confirmed to appear at the live
	// source, by whichever path looked (save-time verification, a check-sources
	// run, or manual attestation). Nil means never confirmed.
	CheckedAt *time.Time
}

type CitationWithSource struct {
	SourceID         int64
	Locator          string
	Quote            string // verbatim line from the source backing this citation
	ManuallyVerified bool
	CheckedAt        *time.Time // see Citation.CheckedAt
	// CheckedBy names who confirmed the quote at CheckedAt — a person's first
	// name, or an Actor* constant. Shown in the authoring portal only; no
	// public template prints it. Empty when CheckedAt is nil.
	CheckedBy  string
	SourceURL  string
	Publisher  string
	SourceKind string // statute|regulation|gov_guidance|nonprofit|editorial
}

// Concept names a claim that recurs across jurisdictions, from the closed
// registry migration 000019 seeded (ADR-011). TopicSlug is the owning topic;
// concepts owned by renting-fundamentals are cross-cutting and may be tagged
// on any page.
type Concept struct {
	ID        int64
	Slug      string
	Name      string
	TopicID   int64
	TopicSlug string
	// Definition is the registry's plain-language gloss of the term
	// (migration 000024): what the word means, free of jurisdictional claims
	// — the law stays in statements. Populated by the reference-layer
	// queries; may be empty elsewhere.
	Definition string
}

// ConceptCoverageRow is one concept's standing per place (ADR-011 D3),
// shaped like CoverageRow so the dashboard renders both matrices the same
// way: a dot per cell, not a wall of place names.
type ConceptCoverageRow struct {
	Concept  Concept
	National bool
	// Status maps a place's display name to "localized" (its own tagged,
	// cited statement), "generic" (covered only by the national statement —
	// the research queue), or "missing" (no statement anywhere up the
	// chain). A place absent from the map is out of scope: a topic-owned
	// concept where the place has no page of that topic is not a gap.
	Status map[string]string
}

// Term is one row of the reference layer (ADR-012): a concept that has at
// least one published tagged statement, with a blurb derived from its
// national definition when one exists.
type Term struct {
	Slug      string
	Name      string
	TopicSlug string
	// Blurb is the registry definition (migration 000024), falling back to
	// the first sentence of the national tagged statement for any concept the
	// registry has not glossed yet.
	Blurb string
	// HasNational reports whether the national page states the claim — the
	// condition for appearing on the homepage's reference list (ADR-012 D3).
	HasNational bool
	// Localized counts the non-national places whose published page carries
	// the concept.
	Localized int
}

// ConceptInstance is one place's published statement for a concept, with
// enough context to link back to the statement's own page at its anchor.
type ConceptInstance struct {
	Jurisdiction Jurisdiction
	TopicSlug    string
	Statement    CitedStatement
}

// ConceptPageData is everything /c/{slug} renders (ADR-012 D1): every
// published national statement carrying the concept (statements may share a
// tag, so this is a slice — a second national statement is more of the
// general rule, never a "where you live" entry), and every published
// localized instance.
type ConceptPageData struct {
	Concept  Concept
	National []ConceptInstance
	Local    []ConceptInstance
}

// SourceUsage is one source's citation structure across the whole site
// (ADR-011 D6): the same page cited under different statutes is one source
// with several locators, and this row is what makes that visible.
type SourceUsage struct {
	SourceID   int64
	Statements int
	Pages      int
	Locators   []string
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
	// AuthorNotes is authoring-portal working text. Only the authoring queries
	// populate it; the public render path never reads it.
	AuthorNotes    string
	LastReviewedAt *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	PublishedAt    *time.Time
	// UpdatedBy names who last touched this row — saved, published or took it
	// down. On a published page this is also what the public "Last reviewed
	// by" byline reads: a later edit moves the credit to whoever made it.
	// Empty on rows from before the portal had per-person logins; the byline
	// falls back to the historical reviewer claim.
	UpdatedBy string
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
	ID          int64
	BodyMD      string
	ConceptSlug string // "" when untagged; doubles as the public anchor (ADR-011 D4)
	// TopicRefSlug/Name mark a statement that summarizes a whole topic rather
	// than making one claim (ADR-011 D7). Mutually exclusive with ConceptSlug,
	// enforced by the statements_one_tag_check constraint.
	TopicRefSlug string
	TopicRefName string
	Citations    []CitationWithSource
}

type SearchResult struct {
	Type string // "statement"|"playbook"|"term"
	// TermSlug/TermName carry a "term" result: a registry concept whose
	// reference page answers the query (ADR-012 D4).
	TermSlug         string
	TermName         string
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
	Language               string
	LastMod                *time.Time
}

// Path is the URL to emit for this entry. The sitemap must agree with the
// canonical tag on the page, so both are built from Jurisdiction.TopicPathIn.
func (e SitemapEntry) Path() string {
	j := Jurisdiction{Kind: e.JurisdictionKind, Slug: e.JurisdictionSlug, ParentSlug: e.JurisdictionParentSlug}
	return j.TopicPathIn(e.Language, e.TopicSlug)
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
	// UpdatedBy names who last touched the page; see Playbook.UpdatedBy.
	UpdatedBy      string
	StatementCount int
	SourceCount    int
	// RevisesPublished marks a draft whose slot already holds a live page.
	// Publishing it replaces that page and retires the old version, which is a
	// different act from publishing a new one.
	RevisesPublished bool
}
