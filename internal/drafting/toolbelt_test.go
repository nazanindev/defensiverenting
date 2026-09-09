package drafting

import (
	"strings"
	"testing"

	"github.com/nazanindev/defensiverenting/internal/store"
)

func TestResolveLanguage_refusesDeferredLanguage(t *testing.T) {
	prev := store.ContentLanguages
	store.ContentLanguages = []string{"en"}
	t.Cleanup(func() { store.ContentLanguages = prev })
	if _, err := ResolveLanguage("es"); err == nil || !strings.Contains(err.Error(), "ADR-015") {
		t.Errorf("ResolveLanguage(es) err = %v, want a deferral naming ADR-015", err)
	}
	if got, err := ResolveLanguage(""); err != nil || got != "en" {
		t.Errorf("ResolveLanguage(\"\") = %q, %v", got, err)
	}
	if _, err := ResolveLanguage("fr"); err == nil || strings.Contains(err.Error(), "ADR-015") {
		t.Errorf("an unsupported language must still be reported as unsupported, got %v", err)
	}
}
