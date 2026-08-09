package main

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// resolveFromForm drives resolveTopic through a real form POST, the way both
// authoring form handlers reach it.
func resolveFromForm(t *testing.T, topicKey, customName, citySlug string) (slug, name, msg string) {
	t.Helper()
	form := url.Values{}
	form.Set("topic_key", topicKey)
	form.Set("custom_topic_name", customName)
	r := httptest.NewRequest("POST", "/new", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return resolveTopic(r, citySlug)
}

func TestResolveTopic_sharedSlugIsNotCityPrefixed(t *testing.T) {
	// The 2026-08-01 incident: both form paths used to build the slug as
	// citySlug + "-" + topicKey, fragmenting topic hubs across cities.
	slug, name, msg := resolveFromForm(t, "cant-pay-rent", "", "boston")
	if msg != "" {
		t.Fatalf("unexpected rejection: %s", msg)
	}
	if slug != "cant-pay-rent" {
		t.Errorf("slug = %q, want cant-pay-rent (no city prefix)", slug)
	}
	if name != "Can't Pay Rent" {
		t.Errorf("name = %q, want the dropdown label", name)
	}
}

func TestResolveTopic_rejectsCityPrefixedCustomTopic(t *testing.T) {
	slug, _, msg := resolveFromForm(t, "custom", "Boston Security Deposits", "boston")
	if msg == "" {
		t.Fatalf("expected rejection, got slug %q", slug)
	}
	if !strings.Contains(msg, "security-deposits") {
		t.Errorf("message should suggest the unprefixed slug, got: %s", msg)
	}
}

func TestResolveTopic_customTopicKeepsTypedName(t *testing.T) {
	slug, name, msg := resolveFromForm(t, "custom", "Bed Bugs", "boston")
	if msg != "" {
		t.Fatalf("unexpected rejection: %s", msg)
	}
	if slug != "bed-bugs" {
		t.Errorf("slug = %q, want bed-bugs", slug)
	}
	if name != "Bed Bugs" {
		t.Errorf("name = %q, want the name the author typed", name)
	}
}

func TestResolveTopic_requiresATopic(t *testing.T) {
	if _, _, msg := resolveFromForm(t, "", "", "boston"); msg == "" {
		t.Error("expected a message when no topic is selected")
	}
}

// A city named so that a legitimate topic slug shares its prefix must still be
// allowed: only an exact "{city}-" prefix is a fragmenting slug.
func TestResolveTopic_allowsUnrelatedSlugSharingCityPrefix(t *testing.T) {
	if _, _, msg := resolveFromForm(t, "renting-fundamentals", "", "rent"); msg != "" {
		t.Errorf("unexpected rejection: %s", msg)
	}
}

func TestKnownTopicsCoverDropdown(t *testing.T) {
	// Every key offered in form.html must resolve to a display name, or saving
	// that option would fall back to slugToTitle and invent a name.
	for _, key := range []string{
		"heat-not-working", "cant-pay-rent", "notice-to-quit",
		"security-deposit-not-returned", "landlord-entry-without-notice",
		"uninhabitable-conditions", "rent-increase", "discrimination",
		"lease-renewal", "noise-complaints", "move-in-checklist",
		"move-out-checklist", "resource-directory",
	} {
		if knownTopics[key] == "" {
			t.Errorf("dropdown key %q has no display name in knownTopics", key)
		}
	}
}
