package sitecopy

import (
	"bufio"
	"bytes"
	_ "embed"
	"regexp"
	"strings"
	"sync"
	"unicode"
)

// The spelling pass is deliberately simple: a word is fine if the American
// English list (dict/american-english, SCOWL size 50) or the site's own word
// list (words.txt) knows it. Names, brands and acronyms the site uses go in
// words.txt. Emails and links are skipped, and so are all-caps acronyms,
// which the voice rules handle separately.

//go:embed dict/american-english
var dictRaw []byte

//go:embed words.txt
var siteWordsRaw []byte

var (
	dictOnce sync.Once
	dict     map[string]bool
)

func loadDict() {
	dict = map[string]bool{}
	for _, raw := range [][]byte{dictRaw, siteWordsRaw} {
		sc := bufio.NewScanner(bytes.NewReader(raw))
		for sc.Scan() {
			w := strings.TrimSpace(sc.Text())
			if w == "" || strings.HasPrefix(w, "#") {
				continue
			}
			dict[w] = true
			dict[strings.ToLower(w)] = true
		}
	}
}

var (
	skipRun = regexp.MustCompile(`\S+@\S+|https?://\S+|www\.\S+`)
	wordRun = regexp.MustCompile(`[A-Za-z]+(?:'[A-Za-z]+)*`)
)

// Misspelled returns the words in text the dictionary does not know, each
// once, in order of first appearance.
func Misspelled(text string) []string {
	dictOnce.Do(loadDict)
	text = skipRun.ReplaceAllString(text, " ")
	// Typographic apostrophes read as the plain one the list uses.
	text = strings.NewReplacer("’", "'", "‘", "'").Replace(text)
	var out []string
	seen := map[string]bool{}
	for _, w := range wordRun.FindAllString(text, -1) {
		if known(w) || seen[w] {
			continue
		}
		seen[w] = true
		out = append(out, w)
	}
	return out
}

func known(w string) bool {
	if len(w) == 1 {
		return true
	}
	if dict[w] || dict[strings.ToLower(w)] {
		return true
	}
	// Acronyms (AI, PDF, IP) and their plurals (PDFs).
	if isUpper(strings.TrimSuffix(w, "s")) {
		return true
	}
	// Possessives of known words: the landlord's, Anthropic's.
	if base, ok := strings.CutSuffix(w, "'s"); ok && known(base) {
		return true
	}
	// Plurals of site words that the list carries only in the singular.
	if base, ok := strings.CutSuffix(w, "s"); ok && len(base) > 2 && (dict[base] || dict[strings.ToLower(base)]) {
		return true
	}
	return false
}

func isUpper(s string) bool {
	if len(s) < 2 {
		return false
	}
	for _, r := range s {
		if !unicode.IsUpper(r) {
			return false
		}
	}
	return true
}
