package drafting

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"
)

const pdftotextTimeout = 20 * time.Second

// readingOrderMark separates the layout reading of a PDF from its
// reading-order reading when both are kept (see pdfExtract).
const readingOrderMark = "=== The same document, read column by column ==="

// isPDF detects a PDF response by Content-Type or by the file's magic bytes
// (some .gov servers mislabel PDFs as text/html or octet-stream).
func isPDF(contentType string, body []byte) bool {
	if strings.Contains(strings.ToLower(contentType), "application/pdf") {
		return true
	}
	return bytes.HasPrefix(body, []byte("%PDF-"))
}

// pdfExtract returns the plain text of a PDF document. It prefers poppler's
// pdftotext when it's installed locally: government PDF generators commonly
// produce CID/Type0 font encodings and non-standard ToUnicode maps that
// pdftotext handles correctly and that pdfExtractGo, the pure-Go reader
// below, either garbles or panics on. When pdftotext isn't on PATH, or it
// produces nothing, pdfExtractGo is the fallback, so a source is still
// readable with no local dependency at all.
//
// The extractor that produced the text is returned with it: the two produce
// different text from the same file, and a quote confirmed under one cannot
// be declared missing under the other (see Comparable).
func pdfExtract(body []byte) (string, string, error) {
	if text, err := pdftotextExtract(body, true); err == nil && text != "" {
		// A two-column page in layout mode interleaves its columns line
		// by line, and a sidebar box lands inside the paragraph next to
		// it (the Georgia landlord-tenant handbook, the Tennessee renter
		// booklets). Plain mode reads each column through in order. When
		// the two readings differ, both are kept, so a quote from either
		// matches and every quote confirmed in layout text still does.
		if plain, err := pdftotextExtract(body, false); err == nil && plain != "" &&
			normalizeForMatch(plain) != normalizeForMatch(text) {
			return text + "\n\n" + readingOrderMark + "\n\n" + plain, ExtractorPDFToTextBoth, nil
		}
		return text, ExtractorPDFToText, nil
	}
	text, err := pdfExtractGo(body)
	return text, ExtractorPDFGo, err
}

// pdftotextExtract shells out to poppler's pdftotext (the poppler-utils /
// poppler package). Any failure — the binary isn't installed, or it errors
// on this particular file — is returned as an error for pdfExtract to treat
// as "fall through", not surfaced to the caller directly.
func pdftotextExtract(body []byte, layout bool) (string, error) {
	path, err := exec.LookPath("pdftotext")
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), pdftotextTimeout)
	defer cancel()
	// "-layout" keeps the page's line and column positions, which suits
	// statute PDFs with line numbers and indented subsections. Without it,
	// pdftotext reads each column through before the next.
	args := []string{"-", "-"}
	if layout {
		args = append([]string{"-layout"}, args...)
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdin = bytes.NewReader(body)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pdftotext: %w", err)
	}
	return strings.TrimSpace(out.String()), nil
}

// pdfExtractGo is the pure-Go PDF text extractor. It requires no local
// dependency, but the library can panic on malformed files, so the whole
// extraction is recovered into an error.
func pdfExtractGo(body []byte) (text string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("pdf extraction failed: %v", r)
		}
	}()
	r, err := pdf.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return "", fmt.Errorf("pdf extraction failed: %w", err)
	}
	plain, err := r.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("pdf extraction failed: %w", err)
	}
	out, err := io.ReadAll(plain)
	if err != nil {
		return "", fmt.Errorf("pdf extraction failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
