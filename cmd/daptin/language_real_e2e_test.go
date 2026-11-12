package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAcceptLanguageBoundsRealE2E(t *testing.T) {
	fixture := startDaptinE2E(t, transportE2EDaptinOptions{})
	const marker = "untrusted-language-advisory-marker"
	for _, test := range []struct{ name, header string }{
		{"browser", "fr-CA,fr;q=0.9,en-US;q=0.8,en;q=0.7"},
		{"comma inflation", strings.Repeat("en,", 1000) + "en"},
		{"large comma inflation", strings.Repeat("en,", (1<<19)/3) + "en"},
		{"large wildcard inflation", strings.Repeat("*,", (1<<19)/2) + "*"},
		{"large malformed", marker + strings.Repeat("x", 1<<19)},
		{"small malformed", "en;q=" + marker},
	} {
		t.Run(test.name, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodGet, fixture.URL+"/v1/models", nil)
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Accept-Language", test.header)
			// No account or bearer token: this exercises the pre-authentication path.
			response, err := fixture.Client.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			_, readErr := io.Copy(io.Discard, response.Body)
			response.Body.Close()
			if readErr != nil {
				t.Fatal(readErr)
			}
			if response.StatusCode != http.StatusOK {
				t.Fatalf("status=%d, want normal guest model discovery", response.StatusCode)
			}
		})
	}
	fixture.Process.stopProcess()
	if strings.Contains(fixture.Process.logs.String(), marker) {
		t.Fatal("raw Accept-Language header leaked into server logs")
	}
}
