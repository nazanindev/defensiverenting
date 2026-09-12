package templates

import (
	"bytes"
	"strings"
	"testing"
)

// The analytics beacon renders only when a token is set. Local development and
// tests must never ship a script that phones out.
func TestAnalyticsBeaconNeedsToken(t *testing.T) {
	render := func() string {
		var b bytes.Buffer
		if err := tmpl.ExecuteTemplate(&b, "site-footer", NotFoundPage{}); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	SetAnalyticsToken("")
	if strings.Contains(render(), "cloudflareinsights") {
		t.Fatal("beacon rendered with no token")
	}
	SetAnalyticsToken("abc123")
	defer SetAnalyticsToken("")
	out := render()
	if !strings.Contains(out, "beacon.min.js") || !strings.Contains(out, `"token": "abc123"`) {
		t.Fatalf("beacon missing or token not embedded:\n%s", out)
	}
}
