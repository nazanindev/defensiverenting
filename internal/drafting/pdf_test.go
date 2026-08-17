package drafting

import "testing"

// pdftotextExtract must return an error on non-PDF bytes whether or not
// poppler is installed locally: missing binary -> LookPath error; installed
// binary -> pdftotext itself rejects the invalid PDF. Either way it must
// never panic, since pdfExtract relies on that error to fall through.
func TestPdftotextExtract_InvalidBytesErrors(t *testing.T) {
	if _, err := pdftotextExtract([]byte("not a pdf")); err == nil {
		t.Error("pdftotextExtract(garbage) = nil error, want an error")
	}
}

// pdfExtract must fall through pdftotext (unavailable or rejecting) to the
// pure-Go extractor and return a clean error, never panic, on bytes that
// aren't a real PDF.
func TestPdfExtract_InvalidBytesReturnsErrorNoPanic(t *testing.T) {
	if _, err := pdfExtract([]byte("not a pdf")); err == nil {
		t.Error("pdfExtract(garbage) = nil error, want an error")
	}
}

func TestPdfExtractGo_InvalidBytesReturnsErrorNoPanic(t *testing.T) {
	if _, err := pdfExtractGo([]byte("not a pdf")); err == nil {
		t.Error("pdfExtractGo(garbage) = nil error, want an error")
	}
}
