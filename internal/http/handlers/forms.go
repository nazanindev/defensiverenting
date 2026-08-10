package handlers

import (
	"net/http"
	"strings"

	tmpl "github.com/nazanindev/defensiverenting/web/templates"
)

// The report and contact forms post to a Cloudflare Worker, not here. This
// server only renders the form and the thank-you page, so there is no POST
// route, no body parsing, and no spam handling in this process. When a
// submission is rejected the Worker sends the reader back with ?error=<code>,
// and the codes below are the whole contract between the two.
//
// Anything unrecognised falls through to a generic message rather than being
// shown raw: the query string is reader-controlled, and a code we did not
// write is not text we want on the page.
var formErrors = map[string]string{
	"captcha":   "The spam check did not pass. Please try again.",
	"message":   "Please write at least a few words about the problem.",
	"email":     "That email address does not look right. Check it, or leave it blank.",
	"ratelimit": "You have sent several messages in the last hour. Please try again later.",
}

const genericFormError = "Something went wrong on our side. Your message was not sent. Please try again."

func formError(r *http.Request) string {
	code := r.URL.Query().Get("error")
	if code == "" {
		return ""
	}
	if msg, ok := formErrors[code]; ok {
		return msg
	}
	return genericFormError
}

// sitePath keeps only a path on this site. The value arrives in ?url= from a
// "Report a problem" link, so a reader can put anything there; an absolute URL
// to somewhere else would turn the form into a way to make our page vouch for
// a stranger's link.
func sitePath(raw string) string {
	if !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") {
		return ""
	}
	if len(raw) > 500 {
		return ""
	}
	return raw
}

// Report renders the "something here is wrong" form.
func Report(formsURL, turnstileSiteKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		render(w, r, http.StatusOK, tmpl.ReportPage{
			FormsURL:         formsURL,
			TurnstileSiteKey: turnstileSiteKey,
			ErrorMessage:     formError(r),
			PageURL:          sitePath(q.Get("url")),
			OrgName:          q.Get("org"),
		})
	}
}

// Contact renders the general contact form.
func Contact(formsURL, turnstileSiteKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		render(w, r, http.StatusOK, tmpl.ContactPage{
			FormsURL:         formsURL,
			TurnstileSiteKey: turnstileSiteKey,
			ErrorMessage:     formError(r),
		})
	}
}

// Thanks confirms a submission. It is a plain GET so a reload cannot resend.
func Thanks(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("k")
	if kind != "report" {
		kind = "contact"
	}
	render(w, r, http.StatusOK, tmpl.ThanksPage{Kind: kind})
}
