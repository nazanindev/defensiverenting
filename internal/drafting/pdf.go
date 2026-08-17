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
func pdfExtract(body []byte) (string, error) {
	if text, err := pdftotextExtract(body); err == nil && text != "" {
		return text, nil
	}
	return pdfExtractGo(body)
}

// pdftotextExtract shells out to poppler's pdftotext (the poppler-utils /
// poppler package). Any failure — the binary isn't installed, or it errors
// on this particular file — is returned as an error for pdfExtract to treat
// as "fall through", not surfaced to the caller directly.
func pdftotextExtract(body []byte) (string, error) {
	path, err := exec.LookPath("pdftotext")
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), pdftotextTimeout)
	defer cancel()
	// "-layout" preserves the source's line/column layout rather than
	// reflowing it; QuoteAppearsIn already normalizes whitespace, so this
	// only helps multi-column statute text extract in reading order.
	cmd := exec.CommandContext(ctx, path, "-layout", "-", "-")
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
