package server

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/daptin/daptin/server/assetcachepojo"
	"github.com/daptin/daptin/server/rootpojo"
	"github.com/daptin/daptin/server/subsite"
	"github.com/gin-gonic/gin"
)

func newTestSubsiteEngine(t *testing.T, gzipEnabled []bool, files map[string]string) *gin.Engine {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cache := &assetcachepojo.AssetFolderCache{
		LocalSyncPath: root,
		CloudStore: rootpojo.CloudStore{
			StoreProvider: "test-cache",
		},
	}
	return CreateSubsiteEngine(subsite.SubSite{}, cache, nil, gzipEnabled...)
}

func readGzipResponse(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()
	reader, err := gzip.NewReader(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func TestSubsiteEngineGzipEnabledByConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	content := strings.Repeat("compressible content ", 100)
	engine := newTestSubsiteEngine(t, []bool{true}, map[string]string{"compressed.js": content})

	request := httptest.NewRequest(http.MethodGet, "/compressed.js", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	if got := response.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("expected gzip content encoding, got %q", got)
	}
	if got := readGzipResponse(t, response); got != content {
		t.Fatal("gzip response did not contain the original asset")
	}
}

func TestSubsiteEngineGzipEnabledByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)

	content := strings.Repeat("compressible content ", 100)
	engine := newTestSubsiteEngine(t, nil, map[string]string{"compressed.js": content})

	request := httptest.NewRequest(http.MethodGet, "/compressed.js", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	if got := response.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("expected gzip content encoding by default, got %q", got)
	}
	if got := readGzipResponse(t, response); got != content {
		t.Fatal("gzip response did not contain the original asset")
	}
}

func TestSubsiteEngineGzipCanBeDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	content := strings.Repeat("compressible content ", 100)
	engine := newTestSubsiteEngine(t, []bool{false}, map[string]string{"uncompressed.js": content})

	request := httptest.NewRequest(http.MethodGet, "/uncompressed.js", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	if got := response.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("expected no content encoding, got %q", got)
	}
	if got := response.Body.String(); got != content {
		t.Fatal("uncompressed response did not contain the original asset")
	}
}

func TestSubsiteEngineCompressesPathsExcludedByAPIRouter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	content := strings.Repeat("const value = true;", 100)
	engine := newTestSubsiteEngine(t, []bool{true}, map[string]string{"asset/app.js": content})

	request := httptest.NewRequest(http.MethodGet, "/asset/app.js", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	if got := response.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("expected subsite asset path to be compressed, got %q", got)
	}
	if got := readGzipResponse(t, response); got != content {
		t.Fatal("gzip response did not contain the original asset")
	}
}

func TestSubsiteEngineDoesNotCompressRangeResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)

	content := strings.Repeat("range content ", 100)
	engine := newTestSubsiteEngine(t, []bool{true}, map[string]string{"audio.ogg": content})

	request := httptest.NewRequest(http.MethodGet, "/audio.ogg", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	request.Header.Set("Range", "bytes=0-9")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	if response.Code != http.StatusPartialContent {
		t.Fatalf("expected status 206, got %d", response.Code)
	}
	if got := response.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("expected range response to remain uncompressed, got %q", got)
	}
	if got := response.Header().Get("Content-Range"); got != "bytes 0-9/1400" {
		t.Fatalf("unexpected content range %q", got)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(body); got != content[:10] {
		t.Fatalf("unexpected range body %q", got)
	}
}

func TestSubsiteEngineRespectsGzipQualityZero(t *testing.T) {
	gin.SetMode(gin.TestMode)

	content := strings.Repeat("const value = true;", 100)
	engine := newTestSubsiteEngine(t, []bool{true}, map[string]string{"app.js": content})
	request := httptest.NewRequest(http.MethodGet, "/app.js", nil)
	request.Header.Set("Accept-Encoding", "br, gzip;q=0")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	if got := response.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("expected gzip;q=0 response to remain uncompressed, got %q", got)
	}
	if got := response.Body.String(); got != content {
		t.Fatal("uncompressed response did not contain the original asset")
	}
}

func TestSubsiteEngineUsesRepresentationSpecificETags(t *testing.T) {
	gin.SetMode(gin.TestMode)

	content := strings.Repeat("const value = true;", 100)
	engine := newTestSubsiteEngine(t, []bool{true}, map[string]string{"app.js": content})

	plainRequest := httptest.NewRequest(http.MethodGet, "/app.js", nil)
	plainResponse := httptest.NewRecorder()
	engine.ServeHTTP(plainResponse, plainRequest)

	gzipRequest := httptest.NewRequest(http.MethodGet, "/app.js", nil)
	gzipRequest.Header.Set("Accept-Encoding", "gzip")
	gzipResponse := httptest.NewRecorder()
	engine.ServeHTTP(gzipResponse, gzipRequest)

	plainETag := plainResponse.Header().Get("ETag")
	gzipETag := gzipResponse.Header().Get("ETag")
	if plainETag == "" || gzipETag == "" || plainETag == gzipETag {
		t.Fatalf("expected distinct non-empty ETags, plain=%q gzip=%q", plainETag, gzipETag)
	}
	if got := gzipResponse.Header().Get("Vary"); !strings.Contains(got, "Accept-Encoding") {
		t.Fatalf("expected Vary: Accept-Encoding, got %q", got)
	}

	conditionalRequest := httptest.NewRequest(http.MethodGet, "/app.js", nil)
	conditionalRequest.Header.Set("Accept-Encoding", "gzip")
	conditionalRequest.Header.Set("If-None-Match", gzipETag)
	conditionalResponse := httptest.NewRecorder()
	engine.ServeHTTP(conditionalResponse, conditionalRequest)
	if conditionalResponse.Code != http.StatusNotModified {
		t.Fatalf("expected gzip conditional request to return 304, got %d", conditionalResponse.Code)
	}
}
