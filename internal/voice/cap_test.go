package voice

import (
	"strings"
	"testing"
)

func TestLintAll_statementWordCap(t *testing.T) {
	long := strings.Repeat("The landlord must act. ", 25) // 100 words, short sentences
	v := LintAll("en", map[string]string{"body_md": long})
	found := false
	for _, x := range v {
		if strings.Contains(x, "split it into separate statements") {
			found = true
		}
	}
	if !found {
		t.Errorf("a 100-word statement passed the cap: %v", v)
	}
	if v := LintAll("en", map[string]string{"intro_md": long}); len(v) != 0 {
		t.Errorf("the cap applied to a non-statement text: %v", v)
	}
	if v := LintAll("en", map[string]string{"body_md": strings.Repeat("The landlord must act. ", 20)}); len(v) != 0 {
		t.Errorf("an 80-word statement failed: %v", v)
	}
}
