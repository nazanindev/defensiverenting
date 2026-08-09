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
