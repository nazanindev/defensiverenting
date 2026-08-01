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
	db          store.Store
	cache       *fetchCache
	fetch       func(url string) (fetched, error) // overridable in tests
	extract     textExtractor
	archiveBase string // Internet Archive fallback prefix, overridable in tests
}

// fetched is the result of reading a source: its extracted text, and how it
// was obtained when a fallback was needed.
type fetched struct {
	Text string
	Via  string // empty for a direct fetch; e.g. "web.archive.org snapshot"
}

// New builds a Toolbelt backed by a store, an HTTP fetcher, and the default
// HTML→text extractor.
func New(db store.Store) *Toolbelt {
	tb := &Toolbelt{db: db, cache: newFetchCache(), extract: htmlStripper{}, archiveBase: defaultArchiveBase}
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

	// defaultArchiveBase is the Internet Archive's newest-snapshot endpoint;
	// the id_ flag returns the original page without archive chrome.
	defaultArchiveBase = "https://web.archive.org/web/2id_/"

	// minUsableChars is the least extracted text a page can have and still be
	// quotable. Below it, the fetch is treated as blocked (a JS-only shell or
	// a bot-block page) and the archive fallback kicks in. JS shells carry up
	// to ~1.5k chars of nav text, so this sits above that; a legitimately
	// short page just falls back to its direct text when the archive is no
	// richer.
	minUsableChars = 2000
)

// httpFetch reads a source: direct fetch first, then the newest Internet
// Archive snapshot of the same URL when the direct fetch fails or returns a
// page too thin to quote from (JS-only shells, Cloudflare blocks). The
// citation still points at the original URL; Via records the fallback so the
// agent and the human reviewer can see how the text was obtained.
func (tb *Toolbelt) httpFetch(url string) (fetched, error) {
	text, err := tb.fetchDirect(url)
	if err == nil && !tooThin(text) {
		return fetched{Text: text}, nil
	}
	atext, aerr := tb.fetchDirect(tb.archiveBase + url)
	if aerr == nil && !tooThin(atext) {
		return fetched{Text: atext, Via: "web.archive.org snapshot"}, nil
	}
	if err == nil {
		return fetched{Text: text}, nil // direct was thin, but the archive was no better
	}
	return fetched{}, err
}

// fetchDirect performs one GET and extracts readable text: PDF extraction for
// PDF responses, HTML stripping otherwise. Non-2xx statuses are errors.
func (tb *Toolbelt) fetchDirect(url string) (string, error) {
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
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}
	if isPDF(resp.Header.Get("Content-Type"), body) {
		return pdfExtract(body)
	}
	return tb.extract.extract(string(body)), nil
}

// tooThin reports whether extracted text is too short to quote from — the
// signature of a JS-only shell or a block page.
func tooThin(text string) bool {
	return len(normalizeForMatch(text)) < minUsableChars
}

// FetchExtract fetches a URL and returns its extracted readable text using the
// same direct HTTP fetch + extraction pipeline as the drafting tools. It never
// falls back to the Internet Archive: the source-change checker hashes this
// text to detect upstream drift, so it must reflect the live page only.
func FetchExtract(url string) (string, error) {
	tb := &Toolbelt{extract: htmlStripper{}}
	return tb.fetchDirect(url)
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

// stripAllWS removes all whitespace. Some PDFs encode word spacing purely via
// glyph positioning, so their extracted text arrives with words fused together
// ("THELANDLORDANDTENANTACT"). Comparing with whitespace removed lets a
// normally-spaced quote match such text while still requiring every character
// in order — the verbatim invariant is unchanged.
func stripAllWS(s string) string {
	return strings.Join(strings.Fields(s), "")
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
