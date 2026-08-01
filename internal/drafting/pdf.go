package drafting

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/ledongthuc/pdf"
)

// isPDF detects a PDF response by Content-Type or by the file's magic bytes
// (some .gov servers mislabel PDFs as text/html or octet-stream).
func isPDF(contentType string, body []byte) bool {
	if strings.Contains(strings.ToLower(contentType), "application/pdf") {
		return true
	}
	return bytes.HasPrefix(body, []byte("%PDF-"))
}

// pdfExtract returns the plain text of a PDF document. The pdf library can
// panic on malformed files, so the whole extraction is recovered into an error.
func pdfExtract(body []byte) (text string, err error) {
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
