package server

import (
	stdjson "encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/golang-lru"
)

func TestDashboardNoRouteKeepsSPARoutesAndRejectsMissingAPIsAndAssets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	files := map[string]string{
		"index.html":      "<html>dashboard shell</html>",
		"assets/app.js":   "console.log('dashboard')",
		"images/logo.svg": "<svg></svg>",
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	oldCache := diskFileCache
	cache, err := lru.New(16)
	if err != nil {
		t.Fatal(err)
	}
	diskFileCache = cache
	t.Cleanup(func() { diskFileCache = oldCache })

	router := gin.New()
	SetupNoRouteRouter(http.Dir(root), router)
	router.GET("/live", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	cases := []struct {
		path         string
		status       int
		body         string
		wantAPIError bool
	}{
		{"/api/no_such_resource", http.StatusNotFound, "", true},
		{"/action/world/no_such_action", http.StatusNotFound, "", true},
		{"/assets/no-such-file.js", http.StatusNotFound, "", false},
		{"/images/no-such-file.svg", http.StatusNotFound, "", false},
		{"/assets/app.js", http.StatusOK, files["assets/app.js"], false},
		{"/images/logo.svg", http.StatusOK, files["images/logo.svg"], false},
		{"/files/sample", http.StatusOK, files["index.html"], false},
		{"/users/jane.doe", http.StatusOK, files["index.html"], false},
		{"/apiary/sample", http.StatusOK, files["index.html"], false},
		{"/live", http.StatusNoContent, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if recorder.Code != tc.status {
				t.Fatalf("GET %s status = %d, want %d", tc.path, recorder.Code, tc.status)
			}
			if tc.status == http.StatusNotFound {
				if strings.Contains(recorder.Body.String(), files["index.html"]) || strings.Contains(recorder.Header().Get("Content-Type"), "text/html") {
					t.Fatalf("GET %s returned dashboard HTML with 404", tc.path)
				}
				if tc.wantAPIError {
					var response struct {
						Errors []struct{ Status string } `json:"errors"`
					}
					if !strings.Contains(recorder.Header().Get("Content-Type"), "application/json") || stdjson.Unmarshal(recorder.Body.Bytes(), &response) != nil || len(response.Errors) != 1 || response.Errors[0].Status != "404" {
						t.Fatalf("GET %s returned invalid API error: %q", tc.path, recorder.Body.String())
					}
				}
			} else if recorder.Body.String() != tc.body {
				t.Fatalf("GET %s body = %q, want %q", tc.path, recorder.Body.String(), tc.body)
			}
		})
	}
}
