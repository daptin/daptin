package server

import (
	"strings"
	"testing"
	"time"
)

func TestGetLanguagePreferenceAcceptsBrowserHeader(t *testing.T) {
	pref := GetLanguagePreference("fr-CA,fr;q=0.9,en-US;q=0.8,en;q=0.7", "en")
	if len(pref) < 2 {
		t.Fatalf("expected language preferences, got %#v", pref)
	}
	if pref[0] != "fr" {
		t.Fatalf("expected fr preference first, got %#v", pref)
	}
}

func TestGetLanguagePreferenceRejectsLargeUnderscoreHeaderQuickly(t *testing.T) {
	header := "en" + strings.Repeat("_abcdefghi", 25600)
	done := make(chan []string, 1)

	go func() {
		done <- GetLanguagePreference(header, "en")
	}()

	select {
	case pref := <-done:
		if len(pref) != 0 {
			t.Fatalf("expected fallback to default language preferences, got %#v", pref)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("GetLanguagePreference took too long for malformed underscore-separated Accept-Language header")
	}
}

func TestAcceptLanguageTooComplexCountsHyphenAndUnderscore(t *testing.T) {
	if acceptLanguageTooComplex(strings.Repeat("a_", maxAcceptLanguageSeparators)) {
		t.Fatal("expected separator count at limit to be accepted")
	}
	if !acceptLanguageTooComplex(strings.Repeat("a_", maxAcceptLanguageSeparators+1)) {
		t.Fatal("expected separator count above limit to be rejected")
	}
	if !acceptLanguageTooComplex(strings.Repeat("a-", maxAcceptLanguageSeparators/2+1) + strings.Repeat("a_", maxAcceptLanguageSeparators/2+1)) {
		t.Fatal("expected combined hyphen and underscore separators to be rejected")
	}
}
