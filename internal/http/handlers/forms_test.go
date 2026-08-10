package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/http/handlers"
)

func getReport(t *testing.T, target, siteKey string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	handlers.Report("https://forms.example.org", siteKey)(rec, httptest.NewRequest(http.MethodGet, target, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	return rec.Body.String()
}

// The reported page arrives in a query parameter, so a reader controls it. It
// is rendered back into a link and a hidden field, which is exactly the shape
// that turns an unchecked value into our page vouching for someone else's URL.
func TestReport_keepsOnlySameSitePaths(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want bool
	}{
		{"site path", "/j/boston/repairs", true},
		{"path with query", "/t/repairs?j=boston", true},
		{"absolute elsewhere", "https://evil.example/pwn", false},
		{"protocol relative", "//evil.example/pwn", false},
		{"javascript scheme", "javascript:alert(1)", false},
		{"bare word", "repairs", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := getReport(t, "/report?url="+tc.url, "")
			got := strings.Contains(body, `name="page_url" value=`)
			if got != tc.want {
				t.Errorf("hidden page_url present = %v, want %v (url %q)", got, tc.want, tc.url)
			}
			if !tc.want && strings.Contains(body, "evil.example") {
				t.Error("rejected URL still reached the page")
			}
		})
	}
}

// A rejected submission comes back as a redirect carrying a code. The reader
// never sees the code, and a code we did not write never becomes page text.
func TestReport_errorCodesBecomeReaderFacingText(t *testing.T) {
	cases := []struct {
		code string
		want string
	}{
		{"captcha", "The spam check did not pass"},
		{"ratelimit", "several messages in the last hour"},
		{"server", "Something went wrong on our side"},
		{"<script>alert(1)</script>", "Something went wrong on our side"},
	}

	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			body := getReport(t, "/report?error="+tc.code, "")
			if !strings.Contains(body, tc.want) {
				t.Errorf("page missing %q for error code %q", tc.want, tc.code)
			}
			if strings.Contains(body, "<script>alert") {
				t.Error("error code was rendered as markup")
			}
		})
	}
}

func TestReport_noErrorBannerOnFirstVisit(t *testing.T) {
	if body := getReport(t, "/report", ""); strings.Contains(body, `class="form-error"`) {
		t.Error("a first visit should not show an error")
	}
}

// The widget is the only thing standing between the form and a spam flood, so
// its absence should be a deliberate local-development choice, not a typo that
// ships. Assert both directions.
func TestReport_turnstileFollowsTheSiteKey(t *testing.T) {
	withKey := getReport(t, "/report", "0x4AAA")
	if !strings.Contains(withKey, `data-sitekey="0x4AAA"`) {
		t.Error("configured site key should render the widget")
	}
	if !strings.Contains(withKey, "challenges.cloudflare.com/turnstile") {
		t.Error("widget rendered without its script")
	}

	if noKey := getReport(t, "/report", ""); strings.Contains(noKey, "challenges.cloudflare.com") {
		t.Error("empty site key should render no widget")
	}
}

func TestReport_postsToTheWorker(t *testing.T) {
	body := getReport(t, "/report", "")
	if !strings.Contains(body, `action="https://forms.example.org/report"`) {
		t.Error("form should post to the configured forms origin")
	}
}

func TestContact_postsToTheWorker(t *testing.T) {
	rec := httptest.NewRecorder()
	handlers.Contact("https://forms.example.org", "")(rec, httptest.NewRequest(http.MethodGet, "/contact", nil))

	if !strings.Contains(rec.Body.String(), `action="https://forms.example.org/contact"`) {
		t.Error("form should post to the configured forms origin")
	}
}

// ?k= comes back from the Worker, so anything other than the one value that
// changes the wording falls through to the general message.
func TestThanks_wording(t *testing.T) {
	cases := []struct {
		target string
		want   string
	}{
		{"/thanks?k=report", "Thank you for the report"},
		{"/thanks?k=contact", "We got your message"},
		{"/thanks", "We got your message"},
		{"/thanks?k=nonsense", "We got your message"},
	}

	for _, tc := range cases {
		t.Run(tc.target, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handlers.Thanks(rec, httptest.NewRequest(http.MethodGet, tc.target, nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), tc.want) {
				t.Errorf("page missing %q", tc.want)
			}
		})
	}
}
