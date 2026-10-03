package drafting

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// twoColumnPDF builds a one-page PDF with a left and a right column of
// lines, the shape of the Georgia handbook and the Tennessee booklets.
func twoColumnPDF(left, right []string) []byte {
	var content strings.Builder
	for i, ln := range left {
		fmt.Fprintf(&content, "BT /F1 11 Tf 50 %d Td (%s) Tj ET\n", 740-i*14, ln)
	}
	for i, ln := range right {
		fmt.Fprintf(&content, "BT /F1 11 Tf 320 %d Td (%s) Tj ET\n", 740-i*14, ln)
	}
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", content.Len(), content.String()),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}
	var b strings.Builder
	b.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objs))
	for i, o := range objs {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, off := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return []byte(b.String())
}

// A paragraph in one column must be quotable across its line breaks even
// when layout mode interleaves the other column into it, and a quote that
// matched the layout reading must still match.
func TestPDFExtract_twoColumnsKeepBothReadings(t *testing.T) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("pdftotext not installed")
	}
	left := []string{"In Georgia, if the tenant leaves", "early without permission, the", "landlord can hold the tenant", "responsible for rent."}
	right := []string{"Sidebar: keep copies", "of every notice."}
	text, extractor, err := pdfExtract(twoColumnPDF(left, right))
	if err != nil {
		t.Fatal(err)
	}
	if extractor != ExtractorPDFToTextBoth {
		t.Fatalf("extractor = %q, want %q (the readings differ)", extractor, ExtractorPDFToTextBoth)
	}
	if !QuoteAppearsIn(text, "In Georgia, if the tenant leaves early without permission, the landlord can hold the tenant responsible for rent.") {
		t.Errorf("column paragraph not quotable:\n%s", text)
	}
	if !QuoteAppearsIn(text, "In Georgia, if the tenant leaves") {
		t.Error("a one-line quote from the layout reading must still match")
	}
}

// A single-column PDF reads the same both ways: its text and extractor stay
// exactly as before, so its stored fingerprint does not change.
func TestPDFExtract_oneColumnUnchanged(t *testing.T) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("pdftotext not installed")
	}
	text, extractor, err := pdfExtract(twoColumnPDF([]string{"The landlord shall return", "the deposit within 30 days."}, nil))
	if err != nil {
		t.Fatal(err)
	}
	if extractor != ExtractorPDFToText || strings.Contains(text, readingOrderMark) {
		t.Fatalf("one-column PDF changed: extractor %q, text %q", extractor, text)
	}
}
