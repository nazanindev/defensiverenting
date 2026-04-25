// Package content parses markdown files with YAML frontmatter into ingest-ready structs.
// The citation guarantee is enforced here: every statement block must reference ≥1 source.
package content

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"gopkg.in/yaml.v3"
)

// Source is a citable source declared in frontmatter.
type Source struct {
	ID           string `yaml:"id"`
	URL          string `yaml:"url"`
	Publisher    string `yaml:"publisher"`
	Jurisdiction string `yaml:"jurisdiction"`
	Kind         string `yaml:"kind"`    // statute|regulation|gov_guidance|nonprofit|editorial
	Locator      string `yaml:"locator"` // default locator for this source
}

// Frontmatter is the parsed YAML header of a content file.
type Frontmatter struct {
	Jurisdiction string   `yaml:"jurisdiction"`
	Topic        string   `yaml:"topic"`
	Language     string   `yaml:"language"`
	Title        string   `yaml:"title"`
	Intro        string   `yaml:"intro"`
	Sources      []Source `yaml:"sources"`
}

// StatementBlock is an atomic parsed claim ready for ingest.
type StatementBlock struct {
	Body     string
	Citations []ParsedCitation
}

// ParsedCitation references a source by ID with an optional locator override.
type ParsedCitation struct {
	SourceID string
	Locator  string // overrides the source-level locator if non-empty
}

// Playbook is the fully-parsed representation of a content file.
type Playbook struct {
	Frontmatter
	Statements []StatementBlock
}

var (
	// citeRef matches [source-id] or [source-id:locator] at the end of a paragraph line.
	citeRef = regexp.MustCompile(`\[([^\]/:]+)(?::([^\]]*))?\]`)
)

// Parse parses a markdown file with YAML frontmatter.
// Returns an error if any statement block has no citation reference.
func Parse(data []byte) (Playbook, error) {
	front, body, err := splitFrontmatter(data)
	if err != nil {
		return Playbook{}, fmt.Errorf("split frontmatter: %w", err)
	}

	var fm Frontmatter
	if err := yaml.Unmarshal(front, &fm); err != nil {
		return Playbook{}, fmt.Errorf("parse frontmatter yaml: %w", err)
	}
	if err := validateFrontmatter(fm); err != nil {
		return Playbook{}, err
	}

	// Build a source-id lookup
	sourceIDs := make(map[string]struct{}, len(fm.Sources))
	for _, s := range fm.Sources {
		sourceIDs[s.ID] = struct{}{}
	}
	// editorial is always available implicitly
	sourceIDs["editorial"] = struct{}{}

	statements, err := parseStatements(body, sourceIDs)
	if err != nil {
		return Playbook{}, err
	}

	return Playbook{
		Frontmatter: fm,
		Statements:  statements,
	}, nil
}

// RenderMarkdown converts a markdown string to safe HTML.
func RenderMarkdown(md string) template.HTML {
	gm := goldmark.New(
		goldmark.WithExtensions(extension.Strikethrough),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)
	var buf bytes.Buffer
	if err := gm.Convert([]byte(md), &buf); err != nil {
		escaped := template.HTMLEscapeString(md)
		//nolint:gosec // Sanitized/escaped string is intentionally marked safe for template rendering.
		return template.HTML(escaped)
	}
	//nolint:gosec // Markdown content is curated in-repo content, not user input.
	return template.HTML(buf.String())
}

// ---- internal helpers -------------------------------------------------------

func splitFrontmatter(data []byte) (front, body []byte, err error) {
	delim := []byte("---")
	if !bytes.HasPrefix(bytes.TrimSpace(data), delim) {
		return nil, nil, errors.New("missing YAML frontmatter (file must start with ---)")
	}
	// Strip leading ---
	rest := bytes.TrimPrefix(bytes.TrimSpace(data), delim)
	// Find closing ---
	end := bytes.Index(rest, append([]byte("\n"), delim...))
	if end == -1 {
		return nil, nil, errors.New("unclosed YAML frontmatter (missing closing ---)")
	}
	front = bytes.TrimSpace(rest[:end])
	body = bytes.TrimSpace(rest[end+1+len(delim):])
	return front, body, nil
}

func validateFrontmatter(fm Frontmatter) error {
	var missing []string
	if fm.Jurisdiction == "" {
		missing = append(missing, "jurisdiction")
	}
	if fm.Topic == "" {
		missing = append(missing, "topic")
	}
	if fm.Title == "" {
		missing = append(missing, "title")
	}
	if len(fm.Sources) == 0 {
		missing = append(missing, "sources (must have at least one)")
	}
	if len(missing) > 0 {
		return fmt.Errorf("frontmatter missing required fields: %s", strings.Join(missing, ", "))
	}
	return nil
}

// parseStatements splits the markdown body into paragraph blocks and extracts
// citation references. Blocks without citations are allowed only if they contain
// no prose (e.g. heading lines); prose blocks without citations are an error.
func parseStatements(body []byte, sourceIDs map[string]struct{}) ([]StatementBlock, error) {
	paragraphs := splitParagraphs(string(body))
	var statements []StatementBlock

	for i, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		// Skip pure headings / horizontal rules / HTML comments
		if isNonStatement(para) {
			continue
		}

		citations, cleanBody := extractCitations(para, sourceIDs)
		if len(citations) == 0 {
			return nil, fmt.Errorf("paragraph %d has no citation reference — add [source-id] at the end:\n%q", i+1, shortSnippet(para))
		}
		statements = append(statements, StatementBlock{
			Body:      strings.TrimSpace(cleanBody),
			Citations: citations,
		})
	}
	return statements, nil
}

func splitParagraphs(s string) []string {
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n\n")
}

func isNonStatement(s string) bool {
	return strings.HasPrefix(s, "#") ||
		strings.HasPrefix(s, "---") ||
		strings.HasPrefix(s, "<!--")
}

// extractCitations finds all [ref] / [ref:locator] tokens, validates them against
// known source IDs, and returns citations plus the body with tokens stripped.
func extractCitations(para string, sourceIDs map[string]struct{}) ([]ParsedCitation, string) {
	var citations []ParsedCitation
	clean := citeRef.ReplaceAllStringFunc(para, func(match string) string {
		sub := citeRef.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		id := sub[1]
		locator := ""
		if len(sub) == 3 {
			locator = sub[2]
		}
		if _, ok := sourceIDs[id]; !ok {
			return match // unknown id — leave in place (will surface as prose without citation)
		}
		citations = append(citations, ParsedCitation{SourceID: id, Locator: locator})
		return "" // strip from body text
	})
	return citations, strings.TrimSpace(clean)
}

func shortSnippet(s string) string {
	if len(s) > 100 {
		return s[:100] + "…"
	}
	return s
}
