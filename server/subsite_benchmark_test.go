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
	"github.com/daptin/daptin/server/rootpojo"
	"github.com/daptin/daptin/server/subsite"
	"github.com/gin-gonic/gin"
)

const benchmarkHost = "benchmark.example"

func newSubsiteBenchmarkEngine(b *testing.B, gzipEnabled bool) *gin.Engine {
	b.Helper()
	gin.SetMode(gin.ReleaseMode)
	previousWriter := gin.DefaultWriter
	gin.DefaultWriter = io.Discard
	b.Cleanup(func() { gin.DefaultWriter = previousWriter })

	root := b.TempDir()
	assets := map[string]string{
		"index.html": strings.Repeat("<main>Daptin benchmark</main>", 256),
		"app.js":     strings.Repeat("const benchmarkValue = 'daptin';\n", 2048),
		"audio.ogg":  strings.Repeat("0123456789abcdef", 4096),
	}
	for name, content := range assets {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			b.Fatal(err)
		}
	}

	cache := &assetcachepojo.AssetFolderCache{
		LocalSyncPath: root,
		CloudStore: rootpojo.CloudStore{
			StoreProvider: "benchmark-cache",
		},
	}
	return CreateSubsiteEngine(subsite.SubSite{}, cache, nil, gzipEnabled)
}

func runSubsiteBenchmark(b *testing.B, path string, headers map[string]string, gzipEnabled bool) {
	b.Helper()
	engine := newSubsiteBenchmarkEngine(b, gzipEnabled)
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.Host = benchmarkHost
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	// Warm caches and capture headers such as ETag for conditional workloads.
	warmResponse := httptest.NewRecorder()
	engine.ServeHTTP(warmResponse, request.Clone(request.Context()))
	if warmResponse.Code < 200 || warmResponse.Code >= 400 {
		b.Fatalf("warm-up returned status %d", warmResponse.Code)
	}
	if headers["If-None-Match"] == "warm" {
		request.Header.Set("If-None-Match", warmResponse.Header().Get("ETag"))
	}
	measurementResponse := httptest.NewRecorder()
	engine.ServeHTTP(measurementResponse, request.Clone(request.Context()))

	b.ReportAllocs()
	b.SetBytes(int64(measurementResponse.Body.Len()))
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request.Clone(request.Context()))
			_, _ = io.Copy(io.Discard, response.Result().Body)
		}
	})
}

func BenchmarkSubsiteIndexCached(b *testing.B) {
	runSubsiteBenchmark(b, "/", nil, false)
}

func BenchmarkSubsiteStatic64K(b *testing.B) {
	runSubsiteBenchmark(b, "/app.js", nil, false)
}

func BenchmarkSubsiteStatic64KGzip(b *testing.B) {
	runSubsiteBenchmark(b, "/app.js", map[string]string{"Accept-Encoding": "gzip"}, true)
}

func BenchmarkSubsiteNotModified(b *testing.B) {
	runSubsiteBenchmark(b, "/app.js", map[string]string{"If-None-Match": "warm"}, false)
}

func BenchmarkSubsiteRange1K(b *testing.B) {
	runSubsiteBenchmark(b, "/audio.ogg", map[string]string{
		"Accept-Encoding": "gzip",
		"Range":           "bytes=0-1023",
	}, true)
}
