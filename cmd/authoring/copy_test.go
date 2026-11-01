package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/nazanindev/defensiverenting/internal/sitecopy"
)

// The copy screens render with and without a saved row, and the refusal
// lists its reasons on the same form with the typed text kept.
func TestCopyTemplatesRender(t *testing.T) {
	tmpl := parseTemplates(t)
	info, _ := sitecopy.Lookup("about")

	var buf bytes.Buffer
	rows := []copyRow{
		{Info: info, Title: "About RenterLaw"},
		{Info: info, Title: "Edited", UpdatedBy: "editor", UpdatedAt: time.Now(), Edited: true},
	}
	if err := tmpl.ExecuteTemplate(&buf, "copy.html", map[string]any{"Rows": rows, "SiteURL": "https://example.test"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "built-in text") || !strings.Contains(buf.String(), "editor") {
		t.Errorf("list: %s", buf.String())
	}

	buf.Reset()
	page := sitecopy.Default("about")
	page.Body = "The landlrod site."
	f := copyForm{Info: info, Page: page, Problems: sitecopy.Check(page), SiteURL: "https://example.test"}
	if err := tmpl.ExecuteTemplate(&buf, "copyedit.html", f); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Not saved") || !strings.Contains(out, "The landlrod site.") || !strings.Contains(out, "https://example.test/about") {
		t.Errorf("edit: %s", out)
	}
}
