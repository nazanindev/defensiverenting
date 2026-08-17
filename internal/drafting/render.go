package drafting

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// renderTimeout bounds a headless-render attempt, including browser launch.
const renderTimeout = 25 * time.Second

// chromeRender loads url in headless Chrome and returns the rendered page's
// visible body text. This is the fallback for sources that a static HTTP
// fetch cannot read: JS-only shells (a codelibrary.amlegal.com or
// statutes.capitol.texas.gov style client-rendered app) and bot-block
// interstitials (Cloudflare) that a real browser clears but net/http cannot.
// It runs the live URL, not a snapshot, so the text still reflects current
// law.
//
// Requires a local Chrome or Chromium; chromedp's default allocator searches
// the usual install locations (including the macOS .app bundle path) and
// falls back to PATH. If none is found, or the page errors out, this returns
// an error and the caller falls through to the next tier.
func (tb *Toolbelt) chromeRender(url string) (string, error) {
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), chromedp.DefaultExecAllocatorOptions[:]...)
	defer cancelAlloc()
	ctx, cancelCtx := chromedp.NewContext(allocCtx)
	defer cancelCtx()
	ctx, cancelTimeout := context.WithTimeout(ctx, renderTimeout)
	defer cancelTimeout()

	var text string
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		// A fixed settle window rather than a specific wait condition: sources
		// vary too widely (React app, Cloudflare JS challenge, slow gov CDN) to
		// name one DOM event that reliably means "content is in". This tool is
		// already the last-resort fallback tier, so trading a few seconds of
		// latency for working on more sites is the right side of that call.
		chromedp.Sleep(3*time.Second),
		chromedp.Text("body", &text, chromedp.NodeVisible, chromedp.ByQuery),
	)
	if err != nil {
		return "", fmt.Errorf("headless render failed: %w", err)
	}
	return strings.TrimSpace(text), nil
}
