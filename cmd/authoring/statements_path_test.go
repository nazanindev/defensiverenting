package main

import (
	"net/url"
	"testing"
)

// A form posted from a card returns to that card; a message wins the top.
func TestFilterPath_returnsToCard(t *testing.T) {
	f := readFilter(url.Values{"page": {"3"}, "at": {"p3-7"}})
	if got, want := f.path(""), "/statements?page=3#p3-7"; got != want {
		t.Errorf("path() = %q, want %q", got, want)
	}
	if got, want := f.path("Not done."), "/statements?msg=Not+done.&page=3"; got != want {
		t.Errorf("path(msg) = %q, want %q", got, want)
	}
	f = readFilter(url.Values{"at": {"javascript:alert(1)"}})
	if got, want := f.path(""), "/statements"; got != want {
		t.Errorf("bad at: path() = %q, want %q", got, want)
	}
}
