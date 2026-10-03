package store

import "testing"

// Stamping one of two unreviewed statements on a live page is fewer of the
// same problem, not a new one. The old wording put the positions in brackets
// after a count, so the comparison saw new text and refused every edit that
// stamped one statement while another stayed unread.
func TestAddedIssues_unreviewedShrinkingIsNotNew(t *testing.T) {
	is := func(positions string) PageIssue {
		return PageIssue{Code: "unreviewed-statement",
			Detail: "statement(s) " + positions + " have not been reviewed — read each with its citations and mark it reviewed"}
	}
	if added := addedIssues([]PageIssue{is("3, 6")}, []PageIssue{is("6")}); len(added) != 0 {
		t.Fatalf("one fewer unreviewed statement read as a new problem: %v", added)
	}
	if added := addedIssues([]PageIssue{is("6")}, []PageIssue{is("3, 6")}); len(added) != 1 {
		t.Fatalf("one more unreviewed statement was not caught: %v", added)
	}
}
