package main

import (
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
)

func TestDashboardFallbackRealE2E(t *testing.T) {
	fixture := startDaptinE2E(t, transportE2EDaptinOptions{})
	get := func(path string) (int, string, string) {
		t.Helper()
		response, err := fixture.Client.Get(fixture.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, response.Header.Get("Content-Type"), string(body)
	}

	status, contentType, index := get("/")
	if status != http.StatusOK || !strings.HasPrefix(contentType, "text/html") {
		t.Fatalf("dashboard root = %d %q", status, contentType)
	}
	asset := regexp.MustCompile(`src="(/assets/[^"]+\.js)"`).FindStringSubmatch(index)
	if len(asset) != 2 {
		t.Fatalf("dashboard index has no JavaScript asset: %q", index)
	}
	status, contentType, _ = get(asset[1])
	if status != http.StatusOK || !strings.Contains(contentType, "javascript") {
		t.Fatalf("dashboard JavaScript asset = %d %q", status, contentType)
	}

	for _, path := range []string{"/api/no_such_resource", "/assets/no-such-file.js", "/images/no-such-file.svg"} {
		t.Run(path, func(t *testing.T) {
			status, contentType, body := get(path)
			if status != http.StatusNotFound || strings.Contains(contentType, "text/html") || body == index {
				t.Fatalf("missing path %s = %d %q %q, want 404 without dashboard HTML", path, status, contentType, body)
			}
			if path == "/api/no_such_resource" && !strings.Contains(contentType, "application/json") {
				t.Fatalf("missing API path content type = %q, want JSON", contentType)
			}
		})
	}
	status, contentType, body := get("/files/sample")
	if status != http.StatusOK || !strings.HasPrefix(contentType, "text/html") || body != index {
		t.Fatalf("SPA deep link = %d %q %q, want dashboard index", status, contentType, body)
	}
}
