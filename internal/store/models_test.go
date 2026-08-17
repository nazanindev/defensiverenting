package store_test

import (
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

func TestJurisdictionPath(t *testing.T) {
	tests := []struct {
		name string
		j    store.Jurisdiction
		want string
	}{
		{
			name: "city sits under its state",
			j:    store.Jurisdiction{Kind: "city", Slug: "chicago", ParentSlug: "illinois"},
			want: "/j/illinois/chicago",
		},
		{
			name: "state is one segment",
			j:    store.Jurisdiction{Kind: "state", Slug: "illinois", ParentSlug: "united-states"},
			want: "/j/illinois",
		},
		{
			name: "country is one segment",
			j:    store.Jurisdiction{Kind: "country", Slug: "united-states"},
			want: "/j/united-states",
		},
		{
			// The hierarchy repair should prevent this, but a parentless city
			// must not produce "/j//boston".
			name: "city with no parent falls back to a flat path",
			j:    store.Jurisdiction{Kind: "city", Slug: "boston"},
			want: "/j/boston",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.j.Path(); got != tc.want {
				t.Errorf("Path() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestJurisdictionTopicPath(t *testing.T) {
	j := store.Jurisdiction{Kind: "city", Slug: "chicago", ParentSlug: "illinois"}
	if got, want := j.TopicPath("security-deposits"), "/j/illinois/chicago/security-deposits"; got != want {
		t.Errorf("TopicPath() = %q, want %q", got, want)
	}
	s := store.Jurisdiction{Kind: "state", Slug: "texas", ParentSlug: "united-states"}
	if got, want := s.TopicPath("security-deposits"), "/j/texas/security-deposits"; got != want {
		t.Errorf("state TopicPath() = %q, want %q", got, want)
	}
}

// ADR-007 D1: English stays unprefixed (matching ResolveLanguage/voice.Supported
// treating "en" as the base case everywhere else); every other language gets a
// leading "/{lang}".
func TestLangPrefix(t *testing.T) {
	for _, tc := range []struct{ lang, want string }{
		{"", ""}, {"en", ""}, {"es", "/es"}, {"fr", "/fr"},
	} {
		if got := store.LangPrefix(tc.lang); got != tc.want {
			t.Errorf("LangPrefix(%q) = %q, want %q", tc.lang, got, tc.want)
		}
	}
}

func TestJurisdictionPathIn(t *testing.T) {
	j := store.Jurisdiction{Kind: "city", Slug: "chicago", ParentSlug: "illinois"}
	if got, want := j.PathIn("en"), "/j/illinois/chicago"; got != want {
		t.Errorf("PathIn(en) = %q, want %q", got, want)
	}
	if got, want := j.PathIn("es"), "/es/j/illinois/chicago"; got != want {
		t.Errorf("PathIn(es) = %q, want %q", got, want)
	}
}

func TestJurisdictionTopicPathIn(t *testing.T) {
	j := store.Jurisdiction{Kind: "city", Slug: "chicago", ParentSlug: "illinois"}
	if got, want := j.TopicPathIn("es", "security-deposits"), "/es/j/illinois/chicago/security-deposits"; got != want {
		t.Errorf("TopicPathIn(es, ...) = %q, want %q", got, want)
	}
}

// The sitemap's URL must agree with the canonical tag the page itself emits,
// so both go through TopicPathIn — this pins the sitemap side of that.
func TestSitemapEntryPath(t *testing.T) {
	e := store.SitemapEntry{JurisdictionSlug: "chicago", JurisdictionParentSlug: "illinois", JurisdictionKind: "city", TopicSlug: "security-deposits", Language: "es"}
	if got, want := e.Path(), "/es/j/illinois/chicago/security-deposits"; got != want {
		t.Errorf("SitemapEntry.Path() = %q, want %q", got, want)
	}
}
