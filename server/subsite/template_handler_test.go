package subsite

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/daptin/daptin/server/cache"
	"github.com/gin-gonic/gin"
)

func templateTestContext(headers map[string]string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/page", nil)
	for key, value := range headers {
		c.Request.Header.Set(key, value)
	}
	return c, recorder
}

func TestTemplateResponseHeadersExcludeMiddlewareValues(t *testing.T) {
	c, _ := templateTestContext(nil)
	c.Header("X-Ratelimit-Remaining", "499")
	c.Header("Cache-Control", "public, max-age=60, must-revalidate")
	c.Header("X-Frame-Options", "DENY")
	c.Header("Content-Security-Policy", "default-src 'self'")
	c.Header("Content-Type", "text/html")
	result := templateResponseHeaders(c, &CacheConfig{CustomHeaders: map[string]string{"X-Frame-Options": "DENY"}}, map[string]string{"Content-Security-Policy": "default-src 'self'"})
	if result["Cache-Control"] != "public, max-age=60, must-revalidate" || result["X-Frame-Options"] != "DENY" || result["Content-Security-Policy"] != "default-src 'self'" {
		t.Fatalf("route headers missing: %#v", result)
	}
	if _, found := result["X-Ratelimit-Remaining"]; found {
		t.Fatalf("middleware header was cached: %#v", result)
	}
}

func TestCachedTemplateHeadersAndValidators(t *testing.T) {
	entry := &cache.CachedFile{
		Data: []byte("page"), Modtime: time.Now().Add(-2 * time.Second),
		Headers: map[string]string{
			"Content-Type": "text/html", "Cache-Control": "public, max-age=60, must-revalidate",
			"ETag": `"page-tag"`, "X-Frame-Options": "DENY",
			"Content-Security-Policy": "default-src 'self'", "Vary": "Origin",
		},
	}
	for _, tc := range []struct {
		name, match string
		status      int
	}{
		{name: "hit", status: http.StatusOK},
		{name: "matching weak validator", match: `W/"page-tag"`, status: http.StatusNotModified},
		{name: "nonmatching validator", match: `"other"`, status: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, recorder := templateTestContext(map[string]string{"If-None-Match": tc.match})
			c.Header("X-Ratelimit-Remaining", "498")
			serveCachedTemplate(c, entry, true, false)
			if recorder.Code != tc.status {
				t.Fatalf("status = %d, want %d", recorder.Code, tc.status)
			}
			for key, want := range entry.Headers {
				if got := recorder.Header().Get(key); got != want {
					t.Errorf("%s = %q, want %q", key, got, want)
				}
			}
			if recorder.Header().Get("X-Cache") != "HIT" || recorder.Header().Get("Age") == "" {
				t.Errorf("cache metadata missing: %#v", recorder.Header())
			}
			if recorder.Header().Get("Content-Disposition") != "" {
				t.Errorf("unexpected disposition: %q", recorder.Header().Get("Content-Disposition"))
			}
			if recorder.Header().Get("X-Ratelimit-Remaining") != "498" {
				t.Errorf("middleware header changed: %q", recorder.Header().Get("X-Ratelimit-Remaining"))
			}
			if tc.status == http.StatusNotModified && recorder.Body.Len() != 0 {
				t.Errorf("304 has a body: %q", recorder.Body.String())
			}
		})
	}
}

func TestCachedTemplateGzipPreservesVaryAndUsesDistinctETag(t *testing.T) {
	entry := &cache.CachedFile{
		Data: []byte("page"), GzipData: []byte("compressed"), Modtime: time.Now(),
		Headers: map[string]string{"Content-Type": "text/html", "Vary": "Origin, Accept-Encoding", "ETag": `"page-tag"`},
	}
	c, recorder := templateTestContext(map[string]string{"Accept-Encoding": "gzip", "If-None-Match": `"page-tag"`})
	serveCachedTemplate(c, entry, true, false)
	if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Encoding") != "gzip" || recorder.Header().Get("Vary") != "Origin, Accept-Encoding" {
		t.Fatalf("gzip hit headers: status=%d headers=%#v", recorder.Code, recorder.Header())
	}
	if tag := recorder.Header().Get("ETag"); tag == "" || tag == `"page-tag"` {
		t.Fatalf("gzip representation ETag = %q", tag)
	}
	if !strings.Contains(recorder.Body.String(), "compressed") {
		t.Fatalf("gzip bytes not served: %q", recorder.Body.String())
	}
}

func TestETagMatches(t *testing.T) {
	if !etagMatches(`"other", W/"matching"`, `"matching"`) || !etagMatches("*", `"matching"`) {
		t.Fatal("matching validator was rejected")
	}
	if etagMatches(`"other"`, `"matching"`) || etagMatches(`"matching"`, "") {
		t.Fatal("nonmatching validator was accepted")
	}
}

func TestTemplateExpiresAtUsesGMT(t *testing.T) {
	localTime := time.Date(2026, 9, 20, 15, 30, 0, 0, time.FixedZone("IST", 5*60*60+30*60))
	c, recorder := templateTestContext(nil)
	applyCacheHeaders(c, &CacheConfig{Enable: true, ExpiresAt: &localTime})
	if got, want := recorder.Header().Get("Expires"), localTime.UTC().Format(http.TimeFormat); got != want {
		t.Fatalf("Expires = %q, want %q", got, want)
	}
}
