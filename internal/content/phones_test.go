package content

import (
	"html/template"
	"testing"
)

func TestLinkPhones(t *testing.T) {
	cases := []struct {
		name, in, want string
		src            int64
	}{
		{"dashes", "<p>Call 412-555-0134 today.</p>",
			`<p>Call <a href="tel:+14125550134" data-out="7">412-555-0134</a> today.</p>`, 7},
		{"parens", "<p>(215) 555-0199</p>",
			`<p><a href="tel:+12155550199" data-out="7">(215) 555-0199</a></p>`, 7},
		{"toll free with 1", "<p>Call 1-800-555-0100.</p>",
			`<p>Call <a href="tel:+18005550100" data-out="7">1-800-555-0100</a>.</p>`, 7},
		{"call 211", "<p>Call 211 for rent help.</p>",
			`<p>Call <a href="tel:211" data-out="7">211</a> for rent help.</p>`, 7},
		{"section 211 is not a phone", "<p>Under section 211 of the code.</p>",
			"<p>Under section 211 of the code.</p>", 7},
		{"inside a link stays", `<p><a href="https://x.org">412-555-0134</a></p>`,
			`<p><a href="https://x.org">412-555-0134</a></p>`, 7},
		{"inside an attribute stays", `<p><a href="https://x.org/412-555-0134">site</a></p>`,
			`<p><a href="https://x.org/412-555-0134">site</a></p>`, 7},
		{"no source, no data-out", "<p>412-555-0134</p>",
			`<p><a href="tel:+14125550134">412-555-0134</a></p>`, 0},
		{"a statute cite is not a phone", "<p>See 68 P.S. § 250.512.</p>",
			"<p>See 68 P.S. § 250.512.</p>", 7},
	}
	for _, c := range cases {
		if got := string(LinkPhones(template.HTML(c.in), c.src)); got != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.name, got, c.want)
		}
	}
}
