package main

import (
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

// A save explains the official terms in the statements it changed and leaves
// stored text alone, so untouched statements keep their words and stamps.
func TestExplainStatementsOnlyChanged(t *testing.T) {
	old := "Your landlord must keep the utilities on."
	var stored store.CitedStatement
	stored.BodyMD = old
	sts := []store.IngestStatementParams{
		{BodyMD: old},
		{BodyMD: "A utility shutoff by your landlord is against the law."},
	}
	notes := explainStatements("en", sts, []store.CitedStatement{stored})
	if sts[0].BodyMD != old {
		t.Errorf("untouched statement was rewritten: %q", sts[0].BodyMD)
	}
	if want := "A utility shutoff (water, electric, gas) by your landlord is against the law."; sts[1].BodyMD != want {
		t.Errorf("got %q, want %q", sts[1].BodyMD, want)
	}
	if len(notes) != 1 {
		t.Errorf("notes = %v, want one", notes)
	}
}
