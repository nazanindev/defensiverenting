package drafting

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

// Receipt is what a fetch actually produced, carried beside the text so that
// every consumer can say how the text was obtained instead of trusting a bare
// string. The checker, the authoring form's quote verifier, the drafting
// guardrail and the review queue all read the same fields, which is what
// gives "checked" one meaning across them: a quote found verbatim in text a
// live tier produced, recorded with the tier and extractor that produced it.
type Receipt struct {
	URL  string
	Text string
	// Tier is where the text came from: the live page over plain HTTP, the
	// live page through headless Chrome, or an Internet Archive snapshot.
	Tier string
	// Extractor is how the bytes became text. Different extractors produce
	// different text from the same document (pdftotext versus the pure-Go
	// reader most of all), so a quote confirmed under one cannot be declared
	// missing under another.
	Extractor string
	// Chars is the length of the whitespace-normalized text.
	Chars int
	// Hash identifies the normalized text. Two fetches with the same hash
	// are the same text, which lets a re-check say "unchanged" without
	// matching a single quote.
	Hash string
	// Thin says the text is shorter than a quotable page: the signature of a
	// script-only shell or a bot-check interstitial that answered 200. A
	// quote found in thin text is still found; a quote missing from it is
	// not evidence of anything.
	Thin bool
}

// Fetch tiers.
const (
	TierDirect  = "direct"
	TierRender  = "render"
	TierArchive = "archive"
)

// Text extractors.
const (
	ExtractorHTML      = "html"
	ExtractorRender    = "render"
	ExtractorPDFToText = "pdftotext"
	ExtractorPDFGo     = "pdfgo"
)

// newReceipt fills in the derived fields for text obtained a given way.
func newReceipt(url, text, tier, extractor string) Receipt {
	// A page served as Latin-1 or with a stray byte yields text Postgres
	// refuses ("invalid byte sequence for encoding UTF8"), and one such row
	// used to end a whole check run. Invalid bytes become U+FFFD; quotes are
	// valid UTF-8 already, so matching is unaffected.
	text = strings.ToValidUTF8(text, "\uFFFD")
	norm := normalizeForMatch(text)
	sum := sha256.Sum256([]byte(norm))
	return Receipt{
		URL: url, Text: text, Tier: tier, Extractor: extractor,
		Chars: len(norm), Hash: hex.EncodeToString(sum[:]), Thin: len(norm) < minUsableChars,
	}
}

// Live reports whether the text is the current page rather than a snapshot.
// Only a quote found in live text may be recorded as confirmed at the source.
func (r Receipt) Live() bool { return r.Tier == TierDirect || r.Tier == TierRender }

// Readable reports whether a quote missing from the text means anything: the
// text is live and not thin. Archive text is stale and thin text is a shell,
// so a quote absent from either is "could not check", never "changed".
func (r Receipt) Readable() bool { return r.Live() && !r.Thin }

// Via is the fallback label shown to the drafting agent and reviewer: empty
// for a plain direct fetch, otherwise which fallback supplied the text.
func (r Receipt) Via() string {
	switch r.Tier {
	case TierRender:
		return "headless render"
	case TierArchive:
		return "web.archive.org snapshot"
	}
	return ""
}

// Describe is the one-line human account of how the text was obtained, for
// evidence records and stamps: "direct fetch, html" or "headless render" or
// "direct fetch, pdftotext (thin: 812 chars)".
func (r Receipt) Describe() string {
	var b strings.Builder
	switch r.Tier {
	case TierDirect:
		b.WriteString("direct fetch")
	case TierRender:
		b.WriteString("headless render")
	case TierArchive:
		b.WriteString("web.archive.org snapshot")
	default:
		b.WriteString("unknown tier")
	}
	if r.Extractor != "" && r.Extractor != ExtractorRender {
		b.WriteString(", " + r.Extractor)
	}
	if r.Thin {
		b.WriteString(" (thin: " + strconv.Itoa(r.Chars) + " chars)")
	}
	return b.String()
}

// Comparable reports whether a quote confirmed under the baseline extractor
// can be declared missing from text the current extractor produced. Text from
// the pure-Go PDF reader is a degraded rendering of what pdftotext gives, so a
// quote that pdftotext confirmed and pdfgo cannot find is "not comparable
// here", not drift. An empty baseline (rows confirmed before extractors were
// recorded, or attested by hand) is comparable with anything: there is no
// better evidence to defer to.
func Comparable(baselineExtractor, currentExtractor string) bool {
	return baselineExtractor != ExtractorPDFToText || currentExtractor != ExtractorPDFGo
}

// contextRadius is how much text Context keeps on each side of a quote.
const contextRadius = 240

// Context returns the passage around quote in text, whitespace-normalized,
// with contextRadius characters on either side. It is the baseline a later
// drift proposal shows beside the passage that replaced it, so a reviewer
// sees what moved rather than only that something did. Empty when the quote
// is not in the normalized text (a fused-PDF match has no usable context).
func Context(text, quote string) string {
	hay, needle := normalizeForMatch(text), normalizeForMatch(quote)
	if needle == "" {
		return ""
	}
	at := strings.Index(hay, needle)
	if at < 0 {
		return ""
	}
	start, end := at-contextRadius, at+len(needle)+contextRadius
	if start < 0 {
		start = 0
	}
	if end > len(hay) {
		end = len(hay)
	}
	// Do not cut a UTF-8 sequence in half.
	for start > 0 && !isRuneStart(hay[start]) {
		start--
	}
	for end < len(hay) && !isRuneStart(hay[end]) {
		end++
	}
	out := hay[start:end]
	if start > 0 {
		out = "…" + out
	}
	if end < len(hay) {
		out += "…"
	}
	return out
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }
