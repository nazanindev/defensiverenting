package main

import "testing"

func TestWordDiff(t *testing.T) {
	got := string(wordDiff("Every county court has one.", "Most county courts have one."))
	want := `<del>Every</del> <ins>Most</ins> county <del>court has</del> <ins>courts have</ins> one.`
	if got != want {
		t.Errorf("wordDiff =\n  %s\nwant\n  %s", got, want)
	}
	if got := string(wordDiff("same words", "same words")); got != "same words" {
		t.Errorf("unchanged text = %q", got)
	}
	if got := string(wordDiff("", "<b>")); got != "<ins>&lt;b&gt;</ins>" {
		t.Errorf("all new, escaped = %q", got)
	}
}
