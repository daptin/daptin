package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/daptin/daptin/server/assetcachepojo"
	"github.com/daptin/daptin/server/subsite"
	"github.com/gin-gonic/gin"
)

func TestSubsiteEngineGzipEnabledByConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := CreateSubsiteEngine(subsite.SubSite{}, &assetcachepojo.AssetFolderCache{}, nil, true)
	engine.GET("/compressed", func(c *gin.Context) {
		c.String(http.StatusOK, strings.Repeat("compressible content ", 100))
	})

	request := httptest.NewRequest(http.MethodGet, "/compressed", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	if got := response.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("expected gzip content encoding, got %q", got)
	}
}

func TestSubsiteEngineGzipEnabledByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := CreateSubsiteEngine(subsite.SubSite{}, &assetcachepojo.AssetFolderCache{}, nil)
	engine.GET("/compressed", func(c *gin.Context) {
		c.String(http.StatusOK, strings.Repeat("compressible content ", 100))
	})

	request := httptest.NewRequest(http.MethodGet, "/compressed", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	if got := response.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("expected gzip content encoding by default, got %q", got)
	}
}

func TestSubsiteEngineGzipCanBeDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := CreateSubsiteEngine(subsite.SubSite{}, &assetcachepojo.AssetFolderCache{}, nil, false)
	engine.GET("/uncompressed", func(c *gin.Context) {
		c.String(http.StatusOK, strings.Repeat("compressible content ", 100))
	})

	request := httptest.NewRequest(http.MethodGet, "/uncompressed", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	if got := response.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("expected no content encoding, got %q", got)
	}
}

func TestSubsiteEngineCompressesPathsExcludedByAPIRouter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := CreateSubsiteEngine(subsite.SubSite{}, &assetcachepojo.AssetFolderCache{}, nil, true)
	engine.GET("/asset/app.js", func(c *gin.Context) {
		c.String(http.StatusOK, strings.Repeat("const value = true;", 100))
	})

	request := httptest.NewRequest(http.MethodGet, "/asset/app.js", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	if got := response.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("expected subsite asset path to be compressed, got %q", got)
	}
}

func TestSubsiteEngineDoesNotCompressRangeResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)

	filePath := filepath.Join(t.TempDir(), "audio.ogg")
	content := strings.Repeat("range content ", 100)
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	engine := CreateSubsiteEngine(subsite.SubSite{}, &assetcachepojo.AssetFolderCache{}, nil, true)
	engine.GET("/audio.ogg", func(c *gin.Context) { c.File(filePath) })

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
