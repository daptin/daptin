package server

import (
	stdjson "encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	rice "github.com/GeertJohan/go.rice"
	"github.com/GeertJohan/go.rice/embedded"
	"github.com/gin-gonic/gin"
	"github.com/hashicorp/golang-lru"
)

type dashboardRecordingFS struct {
	http.FileSystem
	opens []string
}

func (fs *dashboardRecordingFS) Open(name string) (http.File, error) {
	fs.opens = append(fs.opens, name)
	return fs.FileSystem.Open(name)
}

func TestDashboardNoRouteRejectsEscapingPathsBeforeCacheAndOpen(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	dashboard := filepath.Join(root, "dashboard")
	for name, content := range map[string]string{
		"dashboard/index.html":    "dashboard shell",
		"dashboard/assets/app.js": "dashboard script",
		"private.txt":             "outside dashboard",
	} {
		file := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(root)
	config := rice.Config{LocateOrder: []rice.LocateMethod{rice.LocateWorkingDirectory}}
	diskBox, err := config.FindBox("dashboard")
	if err != nil {
		t.Fatal(err)
	}
	embeddedName := t.Name()
	embedded.RegisterEmbeddedBox(embeddedName, &embedded.EmbeddedBox{
		Name: embeddedName,
		Files: map[string]*embedded.EmbeddedFile{
			"index.html":    {Filename: "index.html", Content: "dashboard shell", FileModTime: time.Now()},
			"assets/app.js": {Filename: "assets/app.js", Content: "dashboard script", FileModTime: time.Now()},
		},
	})
	t.Cleanup(func() { delete(embedded.EmbeddedBoxes, embeddedName) })
	config = rice.Config{LocateOrder: []rice.LocateMethod{rice.LocateEmbedded}}
	embeddedBox, err := config.FindBox(embeddedName)
	if err != nil {
		t.Fatal(err)
	}
	oldCache := diskFileCache
	t.Cleanup(func() { diskFileCache = oldCache })
	for _, backend := range []struct {
		name string
		fs   http.FileSystem
	}{
		{"rice_disk", diskBox.HTTPBox()},
		{"http_dir", http.Dir(dashboard)},
		{"rice_embedded", embeddedBox.HTTPBox()},
	} {
		t.Run(backend.name, func(t *testing.T) {
			diskFileCache, err = lru.New(16)
			if err != nil {
				t.Fatal(err)
			}
			fs := &dashboardRecordingFS{FileSystem: backend.fs}
			router := gin.New()
			SetupNoRouteRouter(fs, router)
			for _, requestPath := range []string{
				"/../private.txt", "/%2e%2e/private.txt", "/nested/../../private.txt",
				"/..%5cprivate.txt", "/nested%5c..%5c..%5cprivate.txt", "/%00", "/C:%5cprivate.txt",
			} {
				request := httptest.NewRequest(http.MethodGet, requestPath, nil)
				cacheKey := strings.TrimLeft(request.URL.Path, "/")
				for _, cached := range []bool{false, true} {
					// Even a previously cached invalid path must not be served or revalidated.
					if cached {
						diskFileCache.Add(cacheKey, &DiskFileCache{
							Data: []byte("outside dashboard"), ContentType: "text/plain", ETag: "cached-secret",
						})
					}
					for _, etag := range []string{"", "cached-secret"} {
						request.Header.Set("If-None-Match", etag)
						beforeOpens := len(fs.opens)
						recorder := httptest.NewRecorder()
						router.ServeHTTP(recorder, request.Clone(request.Context()))
						if recorder.Code != http.StatusNotFound || strings.Contains(recorder.Body.String(), "outside dashboard") {
							t.Errorf("GET %s cached=%t etag=%q: status=%d body=%q", requestPath, cached, etag, recorder.Code, recorder.Body.String())
						}
						if len(fs.opens) != beforeOpens {
							t.Errorf("GET %s opened filesystem paths %v", requestPath, fs.opens[beforeOpens:])
						}
					}
					diskFileCache.Remove(cacheKey)
				}
			}
			for _, tc := range []struct{ path, body string }{
				{"/assets/app.js", "dashboard script"},
				{"/nested/../assets/app.js", "dashboard script"},
				{"/account/settings", "dashboard shell"},
			} {
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tc.path, nil))
				if recorder.Code != http.StatusOK || recorder.Body.String() != tc.body {
					t.Errorf("GET %s: status=%d body=%q", tc.path, recorder.Code, recorder.Body.String())
				}
			}
		})
	}
}

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
