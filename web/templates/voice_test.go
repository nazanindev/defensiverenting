package templates_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/voice"
)

// Every word a renter reads follows the editorial voice rules, whether an agent
// wrote it or a person did.
//
// internal/voice has always been enforced on drafted content, at save time in
// internal/drafting. It was never pointed at these templates, and they drifted:
// 47 em dashes, spelled-out numbers, sentences past 25 words, and a homepage
// line that described a browser preference as "set your location once and it
// follows you". Every one of those was hand-written by someone who knew the
// rules. Remembering is not a mechanism.
//
// Only renter-facing templates are covered. cmd/authoring is an internal tool
// read by the two of us, and holding it to a reading level written for someone
// under stress on a phone would be theatre.

var (
	tmplAction = regexp.MustCompile(`\{\{[^}]*\}\}`)
	htmlTag    = regexp.MustCompile(`<[^>]+>`)
	metaDesc   = regexp.MustCompile(`(?i)<meta[^>]+(?:name="description"|property="og:description"|name="twitter:description")[^>]+content="([^"]*)"`)

	// Tags whose text a renter actually reads. Attribute values are excluded
	// deliberately: alt and aria-label are read aloud but are labels rather
	// than prose, and linting them produces noise about fragments.
	proseTags = []string{"p", "li", "h1", "h2", "h3", "title", "summary", "figcaption"}

	entities = strings.NewReplacer(
		"&mdash;", "—", "&ndash;", "–", "&amp;", "&", "&nbsp;", " ",
		"&rsquo;", "'", "&lsquo;", "'", "&hellip;", "...", "&rarr;", "", "&larr;", "",
	)
)

// visibleText strips markup and template actions, leaving what a reader sees.
// Actions become a placeholder city name so a sentence built around one still
// reads as a sentence and its length is counted honestly.
func visibleText(s string) string {
	s = tmplAction.ReplaceAllString(s, "Boston")
	s = htmlTag.ReplaceAllString(s, " ")
	s = entities.Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

func TestPublicTemplatesFollowTheVoiceRules(t *testing.T) {
	files, err := filepath.Glob("*.html")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no templates found — this test would pass vacuously")
	}

	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			s := string(raw)

			blocks := map[string]string{}
			for _, tag := range proseTags {
				// `\s` before the attributes, so <li> does not also match <link>.
				re := regexp.MustCompile(`(?is)<` + tag + `(?:\s[^>]*)?>(.*?)</` + tag + `>`)
				for i, m := range re.FindAllStringSubmatch(s, -1) {
					if text := visibleText(m[1]); text != "" {
						blocks[label(tag, i, text)] = text
					}
				}
			}
			// Meta descriptions are prose too: they are what someone reads in a
			// search result, before they ever reach the page.
			for i, m := range metaDesc.FindAllStringSubmatch(s, -1) {
				if text := visibleText(m[1]); text != "" {
					blocks[label("meta", i, text)] = text
				}
			}

			for _, v := range voice.LintAll("en", blocks) {
				t.Errorf("%s", v)
			}
		})
	}
}

// label identifies a block in the failure message, so a violation points at the
// text rather than at a file.
func label(tag string, i int, text string) string {
	if r := []rune(text); len(r) > 60 {
		text = string(r[:60]) + "…"
	}
	return fmt.Sprintf("%s#%02d %q", tag, i, text)
}
