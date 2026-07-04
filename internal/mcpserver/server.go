// Package mcpserver exposes the platform's research + drafting capabilities as
// MCP tools, so a headless Claude agent can find authoritative primary sources,
// read them, and write a *draft* playbook for the author to verify and publish.
//
// The trust anchor is the verbatim-substring guardrail in save_draft_playbook:
// a citation whose quote is not literally present in the text fetch_source
// actually fetched is rejected, so a fabricated citation cannot be saved. The
// agent has no publish tool — every write lands as status="draft".
package mcpserver

import (
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nazanindev/defensiverenting/internal/store"
)

// Server holds the dependencies shared by the tool handlers: the data store and
// the in-process cache of fetched source text that the verbatim check reads from.
type Server struct {
	db      store.Store
	cache   *fetchCache
	fetch   func(url string) (string, error) // overridable in tests
	extract textExtractor
}

// New builds an MCP server with the drafting toolbelt registered.
func New(db store.Store) *mcp.Server {
	s := &Server{
		db:      db,
		cache:   newFetchCache(),
		extract: htmlStripper{},
	}
	s.fetch = s.httpFetch

	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "defensiverenting",
		Version: "v0.1.0",
	}, nil)

	mcp.AddTool(srv, &mcp.Tool{
		Name: "find_sources",
		Description: "List ranked, vetted authoritative primary sources (statutes, ordinances, " +
			"government guidance, legal-aid orgs) for a city, to seed research. Start here.",
	}, s.findSources)

	mcp.AddTool(srv, &mcp.Tool{
		Name: "fetch_source",
		Description: "Fetch a source URL and return its readable text. You MUST fetch a source " +
			"through this tool before citing it: save_draft_playbook only accepts quotes that " +
			"appear verbatim in text returned here.",
	}, s.fetchSource)

	mcp.AddTool(srv, &mcp.Tool{
		Name: "save_draft_playbook",
		Description: "Save a DRAFT tenant-rights playbook (statements + citations) for the author " +
			"to verify and publish. Every citation's quote must be a verbatim line from a source " +
			"you fetched via fetch_source, or the save is rejected. This never publishes anything.",
	}, s.saveDraftPlaybook)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_jurisdictions",
		Description: "List the cities known to the platform (slug + name), for resolving a city.",
	}, s.listJurisdictions)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_topics",
		Description: "List topics that already have a published playbook for a city, so you don't duplicate existing coverage.",
	}, s.listTopics)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_playbook",
		Description: "Fetch an existing published playbook for a city+topic (to review current coverage). Returns not-found if none exists.",
	}, s.getPlaybook)

	return srv
}

// ---- fetch cache -----------------------------------------------------------

// fetchCache stores the extracted text of every URL fetched this session, keyed
// by URL. save_draft_playbook checks citation quotes against these entries.
type fetchCache struct {
	mu sync.Mutex
	m  map[string]string
}

func newFetchCache() *fetchCache { return &fetchCache{m: map[string]string{}} }

func (c *fetchCache) put(url, text string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[url] = text
}

func (c *fetchCache) get(url string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	t, ok := c.m[url]
	return t, ok
}

// ---- HTTP fetch + text extraction ------------------------------------------

const (
	fetchTimeout  = 20 * time.Second
	maxBodyBytes  = 8 << 20 // 8 MiB read cap
	maxReturnRune = 60_000  // truncate the text returned to the agent (full text is still cached)
	userAgent     = "defensiverenting-mcp/0.1 (+https://defensiverenting.com)"
)

func (s *Server) httpFetch(url string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	client := &http.Client{Timeout: fetchTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return "", err
	}
	return s.extract.extract(string(body)), nil
}

// textExtractor turns a fetched document body into readable text. Kept behind an
// interface so a sturdier extractor (readability, PDF) can swap in later.
type textExtractor interface {
	extract(body string) string
}

type htmlStripper struct{}

var (
	reScriptStyle = regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</(script|style)>`)
	reBlockTag    = regexp.MustCompile(`(?i)</?(p|div|br|li|tr|h[1-6]|section|article|ul|ol|table|thead|tbody|blockquote|header|footer)[^>]*>`)
	reAnyTag      = regexp.MustCompile(`<[^>]+>`)
	reInlineWS    = regexp.MustCompile(`[ \t\f\v]+`)
	reBlankLines  = regexp.MustCompile(`\n{3,}`)
)

func (htmlStripper) extract(body string) string {
	s := reScriptStyle.ReplaceAllString(body, " ")
	s = reBlockTag.ReplaceAllString(s, "\n")
	s = reAnyTag.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = strings.TrimSpace(reInlineWS.ReplaceAllString(ln, " "))
	}
	s = strings.Join(lines, "\n")
	s = reBlankLines.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// normalizeForMatch collapses all runs of whitespace (including newlines) to a
// single space, so the verbatim check is robust to layout differences between
// the fetched text and the agent's quote while still requiring the exact words.
func normalizeForMatch(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
