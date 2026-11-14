package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
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

func BenchmarkGetLanguagePreferenceAdvisory(b *testing.B) {
	for _, test := range []struct{ name, header string }{
		{"browser", "fr-CA,fr;q=0.9,en-US;q=0.8,en;q=0.7"},
		{"comma_tags_1MiB", strings.Repeat("en,", (1<<20)/3) + "en"},
		{"wildcards_1MiB", strings.Repeat("*,", (1<<20)/2) + "*"},
	} {
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				GetLanguagePreference(test.header, "en")
			}
		})
	}
}

func TestAcceptLanguageRejectsCommaInflation(t *testing.T) {
	for _, header := range []string{strings.Repeat("en,", 1000) + "en", strings.Repeat("*,", 1000) + "*"} {
		if !acceptLanguageTooComplex(header) {
			t.Fatalf("accepted %d comma-separated entries", strings.Count(header, ",")+1)
		}
	}
}

func TestAcceptLanguageComplexityBoundaries(t *testing.T) {
	for _, test := range []struct {
		name, header string
		rejected     bool
	}{
		{"byte limit", "fr" + strings.Repeat(" ", maxAcceptLanguageBytes-2), false},
		{"over byte limit", "fr" + strings.Repeat(" ", maxAcceptLanguageBytes-1), true},
		{"range limit", strings.Repeat("fr,", maxAcceptLanguageRanges-1) + "fr", false},
		{"over range limit", strings.Repeat("fr,", maxAcceptLanguageRanges) + "fr", true},
		{"wildcard range limit", strings.Repeat("*,", maxAcceptLanguageRanges-1) + "*", false},
		{"over wildcard range limit", strings.Repeat("*,", maxAcceptLanguageRanges) + "*", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := acceptLanguageTooComplex(test.header); got != test.rejected {
				t.Fatalf("rejected=%v, want %v", got, test.rejected)
			}
			if !strings.Contains(test.name, "wildcard") {
				pref := GetLanguagePreference(test.header, "en")
				if test.rejected && len(pref) != 0 {
					t.Fatalf("over-limit input did not use default: %#v", pref)
				}
				if !test.rejected && (len(pref) != 1 || pref[0] != "fr") {
					t.Fatalf("valid boundary input was lost: %#v", pref)
				}
			}
		})
	}
}

func TestGetLanguagePreferenceBoundsAdversarialAllocations(t *testing.T) {
	for _, test := range []struct{ name, header, defaultLanguage string }{
		{"comma inflation", strings.Repeat("fr,", 1000) + "fr", "en"},
		{"large comma inflation", strings.Repeat("fr,", (1<<20)/3) + "fr", "en"},
		{"large wildcards", strings.Repeat("*,", (1<<20)/2) + "*", "en"},
		{"large invalid value", strings.Repeat("x", 1<<20), "en"},
		{"invalid default", "", strings.Repeat("fr,", 1000) + "fr"},
	} {
		t.Run(test.name, func(t *testing.T) {
			allocations := testing.AllocsPerRun(5, func() {
				if pref := GetLanguagePreference(test.header, test.defaultLanguage); len(pref) != 0 {
					t.Fatalf("expected default preferences, got %#v", pref)
				}
			})
			if allocations > 128 {
				t.Fatalf("oversized input allocated %.0f times, want at most 128", allocations)
			}
		})
	}
}

func TestLanguageMiddlewareBoundsBeforeAuthentication(t *testing.T) {
	middleware := &LanguageMiddleware{defaultLanguage: "en"}
	router := gin.New()
	router.Use(middleware.LanguageMiddlewareFunc)
	var observed []string
	router.Use(func(c *gin.Context) {
		observed = c.Request.Context().Value("language_preference").([]string)
		c.AbortWithStatus(http.StatusUnauthorized)
	})
	router.GET("/protected", func(c *gin.Context) { t.Fatal("unauthenticated request reached handler") })
	for _, test := range []struct {
		name, header string
		wantFrench   bool
	}{
		{"browser", "fr-CA,fr;q=0.9,en;q=0.7", true},
		{"comma inflation", strings.Repeat("fr,", 1000) + "fr", false},
		{"large comma inflation", strings.Repeat("fr,", (1<<20)/3) + "fr", false},
		{"large wildcard inflation", strings.Repeat("*,", (1<<20)/2) + "*", false},
		{"large malformed", strings.Repeat("x", 1<<20), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/protected", nil)
			request.Header.Set("Accept-Language", test.header)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status=%d, want unchanged authentication denial", response.Code)
			}
			if test.wantFrench {
				if len(observed) == 0 || observed[0] != "fr" {
					t.Fatalf("browser preferences lost: %#v", observed)
				}
			} else if len(observed) != 0 {
				t.Fatalf("oversized header did not use default: %#v", observed)
			}
		})
	}
}

func TestLanguagePreferenceDoesNotLogUntrustedHeader(t *testing.T) {
	logger := log.StandardLogger()
	level := logger.GetLevel()
	hooks := logger.ReplaceHooks(make(log.LevelHooks))
	defer logger.ReplaceHooks(hooks)
	defer logger.SetLevel(level)
	logger.SetLevel(log.DebugLevel)
	hook := logtest.NewGlobal()
	const marker = "private-language-header-marker"
	for _, header := range []string{"en;q=" + marker, marker + strings.Repeat("x", 1<<20)} {
		GetLanguagePreference(header, "en")
	}
	for _, entry := range hook.AllEntries() {
		if strings.Contains(entry.Message, marker) {
			t.Fatal("untrusted language header was logged")
		}
		if len(entry.Message) > 256 {
			t.Fatalf("language diagnostic too long: %d bytes", len(entry.Message))
		}
	}
}
