package server

import "testing"

func TestConfiguredIMAPHostnameUsesExplicitIMAPHostname(t *testing.T) {
	got := configuredIMAPHostname(" imap.canaster.in ", "api.canaster.in")
	if got != "imap.canaster.in" {
		t.Fatalf("expected explicit imap hostname, got %q", got)
	}
}

func TestConfiguredIMAPHostnameFallsBackToDerivedBackendHostname(t *testing.T) {
	got := configuredIMAPHostname("", "api.canaster.in")
	if got != "imap.api.canaster.in" {
		t.Fatalf("expected derived hostname, got %q", got)
	}
}

func TestConfiguredIMAPHostnameFallsBackToLocalhostWhenBackendHostnameIsEmpty(t *testing.T) {
	got := configuredIMAPHostname("", " ")
	if got != "imap.localhost" {
		t.Fatalf("expected localhost fallback hostname, got %q", got)
	}
}
