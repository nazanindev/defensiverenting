package drafting

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func longPage(marker string) string {
	return "<html><body><p>" + marker + " " + strings.Repeat("statute text ", 300) + "</p></body></html>"
}

func TestHTTPFetch_directSuccess(t *testing.T) {
	direct := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(longPage("DIRECT")))
	}))
	defer direct.Close()

	tb := &Toolbelt{extract: htmlStripper{}, archiveBase: "http://127.0.0.1:1/"} // archive unreachable on purpose
	got, err := tb.httpFetch(direct.URL)
	if err != nil {
		t.Fatalf("httpFetch: %v", err)
	}
	if !strings.Contains(got.Text, "DIRECT") || got.Via != "" {
		t.Errorf("got Via=%q text=%.40q, want direct text and empty Via", got.Via, got.Text)
	}
}

func TestHTTPFetch_archiveFallbackOnBlock(t *testing.T) {
	for name, handler := range map[string]http.HandlerFunc{
		"403 block": func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "Access Denied", http.StatusForbidden)
		},
		"js shell": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`<html><body><div id="app"></div><script>boot()</script></body></html>`))
		},
	} {
		t.Run(name, func(t *testing.T) {
			direct := httptest.NewServer(handler)
			defer direct.Close()
			archive := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !strings.Contains(r.URL.String(), "/snap/") {
					t.Errorf("archive fallback did not use archiveBase prefix: %s", r.URL)
				}
				_, _ = w.Write([]byte(longPage("ARCHIVED")))
			}))
			defer archive.Close()

			tb := &Toolbelt{extract: htmlStripper{}, archiveBase: archive.URL + "/snap/"}
			got, err := tb.httpFetch(direct.URL)
			if err != nil {
				t.Fatalf("httpFetch: %v", err)
			}
			if !strings.Contains(got.Text, "ARCHIVED") {
				t.Errorf("text = %.60q, want archived content", got.Text)
			}
			if got.Via != "web.archive.org snapshot" {
				t.Errorf("Via = %q, want snapshot marker", got.Via)
			}
		})
	}
}

func TestHTTPFetch_renderFallbackOnThinPage(t *testing.T) {
	direct := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body><div id="app"></div><script>boot()</script></body></html>`))
	}))
	defer direct.Close()

	tb := &Toolbelt{
		extract:     htmlStripper{},
		archiveBase: "http://127.0.0.1:1/", // unreachable: the render tier should satisfy this before archive is tried
		render: func(url string) (string, error) {
			return "RENDERED " + strings.Repeat("statute text ", 300), nil
		},
	}
	got, err := tb.httpFetch(direct.URL)
	if err != nil {
		t.Fatalf("httpFetch: %v", err)
	}
	if !strings.Contains(got.Text, "RENDERED") || got.Via != "headless render" {
		t.Errorf("got Via=%q text=%.40q, want headless-render text and Via=%q", got.Via, got.Text, "headless render")
	}
}

func TestHTTPFetch_renderFailureFallsThroughToArchive(t *testing.T) {
	direct := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body><div id="app"></div><script>boot()</script></body></html>`))
	}))
	defer direct.Close()
	archive := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(longPage("ARCHIVED")))
	}))
	defer archive.Close()

	tb := &Toolbelt{
		extract:     htmlStripper{},
		archiveBase: archive.URL + "/snap/",
		render: func(url string) (string, error) {
			return "", errors.New("no local chrome found")
		},
	}
	got, err := tb.httpFetch(direct.URL)
	if err != nil {
		t.Fatalf("httpFetch: %v", err)
	}
	if !strings.Contains(got.Text, "ARCHIVED") || got.Via != "web.archive.org snapshot" {
		t.Errorf("got Via=%q text=%.40q, want archive text when render fails", got.Via, got.Text)
	}
}

func TestHTTPFetch_renderNotConsultedWhenDirectSucceeds(t *testing.T) {
	direct := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(longPage("DIRECT")))
	}))
	defer direct.Close()

	tb := &Toolbelt{
		extract:     htmlStripper{},
		archiveBase: "http://127.0.0.1:1/",
		render: func(url string) (string, error) {
			t.Fatal("render should not be called when the direct fetch already succeeded")
			return "", nil
		},
	}
	if _, err := tb.httpFetch(direct.URL); err != nil {
		t.Fatalf("httpFetch: %v", err)
	}
}

func TestHTTPFetch_errorWhenBothFail(t *testing.T) {
	direct := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusForbidden)
	}))
	defer direct.Close()

	tb := &Toolbelt{extract: htmlStripper{}, archiveBase: "http://127.0.0.1:1/"}
	if _, err := tb.httpFetch(direct.URL); err == nil {
		t.Fatal("httpFetch = nil error, want failure when direct is blocked and archive is down")
	}
}

func TestFetchNoArchive_renderFallbackOn403(t *testing.T) {
	// The class of bug this guards against: a reviewer pastes a quote from a
	// source behind Cloudflare-style bot detection. The direct fetch that backs
	// the quote verifier and checkSources must not give up at the 403 when a
	// render would clear it, the way httpFetch already does for fetch_source.
	direct := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "Access Denied", http.StatusForbidden)
	}))
	defer direct.Close()

	tb := &Toolbelt{
		extract: htmlStripper{},
		render: func(url string) (string, error) {
			return "RENDERED " + strings.Repeat("statute text ", 300), nil
		},
	}
	got, err := tb.fetchNoArchive(direct.URL)
	if err != nil {
		t.Fatalf("fetchNoArchive: %v", err)
	}
	if !strings.Contains(got, "RENDERED") {
		t.Errorf("text = %.40q, want rendered content", got)
	}
}

func TestFetchNoArchive_neverFallsBackToArchive(t *testing.T) {
	// FetchExtract backs drift detection and quote verification, both of which
	// need the live page; a Toolbelt with no archiveBase set proves this path
	// never reaches for one even when direct and render both fail.
	direct := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "Access Denied", http.StatusForbidden)
	}))
	defer direct.Close()

	tb := &Toolbelt{
		extract: htmlStripper{},
		render: func(url string) (string, error) {
			return "", errors.New("no local chrome found")
		},
	}
	if _, err := tb.fetchNoArchive(direct.URL); err == nil {
		t.Fatal("fetchNoArchive = nil error, want the original 403 when render also fails")
	}
}

func TestIsPDF(t *testing.T) {
	if !isPDF("application/pdf", nil) || !isPDF("", []byte("%PDF-1.7 rest")) {
		t.Error("isPDF should detect content-type and magic bytes")
	}
	if isPDF("text/html", []byte("<html>")) {
		t.Error("isPDF false positive on html")
	}
}

func TestStripAllWS_fusedPDFQuoteMatch(t *testing.T) {
	cached := "THELANDLORDANDTENANTACTOF1951 Relatingtotherights,obligationsandliabilities"
	quote := "THE LANDLORD AND TENANT ACT OF 1951"
	if !strings.Contains(stripAllWS(cached), stripAllWS(quote)) {
		t.Error("spaced quote should match fused PDF text with whitespace stripped")
	}
	if strings.Contains(stripAllWS(cached), stripAllWS("THE TENANT LANDLORD ACT")) {
		t.Error("reordered words must not match")
	}
}
