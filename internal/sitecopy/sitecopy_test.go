package sitecopy

import (
	"strings"
	"testing"
)

// The built-in text is held to the same rules as an edit. This is also what
// keeps the defaults honest after a hand conversion from the old templates.
func TestDefaultsPassCheck(t *testing.T) {
	for _, info := range Pages() {
		p := Default(info.Slug)
		if p.Title == "" || p.Description == "" || p.Body == "" {
			t.Errorf("%s: empty field in %+v", info.Slug, p)
		}
		for _, v := range Check(p) {
			t.Errorf("%s: %s", info.Slug, v)
		}
	}
}

func TestCheckRefusesBadCopy(t *testing.T) {
	base := Default("support")
	cases := map[string]Page{
		"typo":       {Slug: "support", Title: base.Title, Description: base.Description, Body: "The landlrod is free."},
		"em dash":    {Slug: "support", Title: base.Title, Description: base.Description, Body: "The site is free — no ads."},
		"empty body": {Slug: "support", Title: base.Title, Description: base.Description},
		"h1 in body": {Slug: "support", Title: base.Title, Description: base.Description, Body: "# Support\n\nThe site is free."},
	}
	for name, p := range cases {
		if got := Check(p); len(got) == 0 {
			t.Errorf("%s: expected a refusal", name)
		}
	}
	if got := Check(cases["typo"]); !strings.Contains(strings.Join(got, "\n"), `"landlrod"`) {
		t.Errorf("typo not named: %v", got)
	}
}

func TestMisspelled(t *testing.T) {
	ok := "We use Claude, made by Anthropic. PDFs and the landlord's lease. Write to contact@renterlaw.org or see https://example.com/x. Renters' rights. It doesn’t follow you."
	if bad := Misspelled(ok); len(bad) != 0 {
		t.Errorf("false positives: %v", bad)
	}
	if bad := Misspelled("The landlrod must retrun the deposit."); len(bad) != 2 {
		t.Errorf("want 2 misspellings, got %v", bad)
	}
}

func TestRenderEscapesRawHTML(t *testing.T) {
	h, err := Render("Hello <script>alert(1)</script> [link](/about)")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(h), "<script>") || !strings.Contains(string(h), `href="/about"`) {
		t.Errorf("render: %s", h)
	}
}
