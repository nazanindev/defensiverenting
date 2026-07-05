// Package drafting is the shared tool layer for AI-assisted playbook drafting.
// It holds the research + draft operations (find sources, fetch source text,
// save a draft playbook) as plain-Go methods with no dependency on any specific
// front-end. Two adapters wrap it: internal/mcpserver exposes it as MCP tools
// (for Claude Code), and cmd/draft drives it from an Anthropic API tool-use loop
// (the production worker).
//
// The trust anchor lives here: SaveDraft rejects any citation whose quote is not
// a verbatim (whitespace-normalized) substring of the text FetchSource fetched,
// so a fabricated citation cannot be saved. There is no publish operation — every
// write is status="draft".
package drafting

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/nazanindev/defensiverenting/internal/store"
)

const draftLanguage = "en"

// Toolbelt bundles the dependencies the drafting operations share.
type Toolbelt struct {
	db      store.Store
	cache   *fetchCache
	fetch   func(url string) (string, error) // overridable in tests
	extract textExtractor
}

// New builds a Toolbelt backed by a store, an HTTP fetcher, and the default
// HTML→text extractor.
func New(db store.Store) *Toolbelt {
	tb := &Toolbelt{db: db, cache: newFetchCache(), extract: htmlStripper{}}
	tb.fetch = tb.httpFetch
	return tb
}

// RejectionError is an agent-visible, fixable failure (bad input, failed
// guardrail). Adapters surface its message to the model rather than treating it
// as a hard error, so the agent can correct and retry.
type RejectionError struct{ Msg string }

func (e *RejectionError) Error() string { return e.Msg }

func reject(format string, args ...any) error {
	return &RejectionError{Msg: fmt.Sprintf(format, args...)}
}

// ---- fetch cache -----------------------------------------------------------

// fetchCache stores the extracted text of every URL fetched this session, keyed
// by URL. SaveDraft checks citation quotes against these entries.
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
	userAgent     = "defensiverenting-drafting/0.1 (+https://defensiverenting.com)"
)

func (tb *Toolbelt) httpFetch(url string) (string, error) {
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
	return tb.extract.extract(string(body)), nil
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

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

var validKinds = map[string]bool{
	"statute": true, "regulation": true, "gov_guidance": true,
	"nonprofit": true, "editorial": true, "court_ruling": true,
}

func defaultKind(k string) string {
	if validKinds[k] {
		return k
	}
	return "gov_guidance"
}
