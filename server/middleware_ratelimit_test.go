package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/buraksezer/olric"
	olricConfig "github.com/buraksezer/olric/config"
	"github.com/gin-gonic/gin"
)

func TestParseRateConfig(t *testing.T) {
	config, err := ParseRateConfig(`{"version":"1","limits":{"/api/widgets":25,"example.com/assets/app.js":10,"example.com/":5}}`)
	if err != nil {
		t.Fatalf("ParseRateConfig: %v", err)
	}
	if config.Version != "1" || config.Limits["/api/widgets"] != 25 {
		t.Fatalf("unexpected config: %#v", config)
	}
}

func TestParseRateConfigRejectsInvalidValues(t *testing.T) {
	tests := []string{
		`{"version":"2","limits":{}}`,
		`{"version":"1","limits":{"widgets":10}}`,
		`{"version":"1","limits":{"/api/widgets?admin=true":10}}`,
		`{"version":"1","limits":{"/api/widgets":0}}`,
		`{"version":"1","limits":{"/api/widgets":-1}}`,
		fmt.Sprintf(`{"version":"1","limits":{"/api/widgets":%d}}`, maxRatePerSecond+1),
		`not-json`,
	}
	for _, value := range tests {
		if _, err := ParseRateConfig(value); err == nil {
			t.Errorf("ParseRateConfig(%q) unexpectedly succeeded", value)
		}
	}
}

func TestRateLimiterEnforcesConfiguredPathAndIgnoresQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(newTestRateLimiter(RateConfig{
		Version: "1",
		Limits:  map[string]int{"/rate-test-backend": 1},
	}, nil, false))
	router.GET("/rate-test-backend", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	first := performRateLimitRequest(router, "example.test", "/rate-test-backend?request=1")
	if first.Code != http.StatusNoContent {
		t.Fatalf("first status = %d, want %d", first.Code, http.StatusNoContent)
	}
	second := performRateLimitRequest(router, "example.test", "/rate-test-backend?request=2")
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want %d", second.Code, http.StatusTooManyRequests)
	}
	if got := second.Header().Get("X-RateLimit-Remaining"); got != "0" {
		t.Fatalf("X-RateLimit-Remaining = %q, want 0", got)
	}
	if got := second.Header().Get("Retry-After"); got != "1" {
		t.Fatalf("Retry-After = %q, want 1", got)
	}
	if got := second.Header().Get("X-RateLimit-Limit"); got != "1" {
		t.Fatalf("X-RateLimit-Limit = %q, want 1", got)
	}
	if got := second.Header().Get("X-RateLimit-Reset"); got == "" {
		t.Fatal("X-RateLimit-Reset is empty")
	}
	if !strings.Contains(second.Body.String(), `"status":"429"`) {
		t.Fatalf("429 body = %q", second.Body.String())
	}
}

func TestSubsiteRateLimiterUsesHostAndPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(newTestRateLimiter(RateConfig{
		Version: "1",
		Limits:  map[string]int{"limited.example/rate-test-subsite": 1},
	}, nil, true))
	router.GET("/rate-test-subsite", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	if got := performRateLimitRequest(router, "limited.example", "/rate-test-subsite").Code; got != http.StatusNoContent {
		t.Fatalf("first limited-host status = %d, want %d", got, http.StatusNoContent)
	}
	if got := performRateLimitRequest(router, "limited.example", "/rate-test-subsite").Code; got != http.StatusTooManyRequests {
		t.Fatalf("second limited-host status = %d, want %d", got, http.StatusTooManyRequests)
	}
	if got := performRateLimitRequest(router, "other.example", "/rate-test-subsite").Code; got != http.StatusNoContent {
		t.Fatalf("other-host status = %d, want %d", got, http.StatusNoContent)
	}
}

func TestRateLimiterSharesAllowanceThroughOlric(t *testing.T) {
	client := startRateLimitTestOlric(t)
	config := RateConfig{Version: "1", Limits: map[string]int{"/rate-test-cluster": 1}}
	newRouter := func() *gin.Engine {
		router := gin.New()
		router.Use(newTestRateLimiter(config, client, false))
		router.GET("/rate-test-cluster", func(c *gin.Context) { c.Status(http.StatusNoContent) })
		return router
	}

	if got := performRateLimitRequest(newRouter(), "example.test", "/rate-test-cluster").Code; got != http.StatusNoContent {
		t.Fatalf("first member status = %d, want %d", got, http.StatusNoContent)
	}
	if got := performRateLimitRequest(newRouter(), "example.test", "/rate-test-cluster").Code; got != http.StatusTooManyRequests {
		t.Fatalf("second member status = %d, want %d", got, http.StatusTooManyRequests)
	}
}

func newTestRateLimiter(config RateConfig, client *olric.EmbeddedClient, subsite bool) gin.HandlerFunc {
	routeKey := func(c *gin.Context) string {
		return strings.Split(c.Request.RequestURI, "?")[0]
	}
	if subsite {
		routeKey = func(c *gin.Context) string {
			return c.Request.Host + strings.Split(c.Request.RequestURI, "?")[0]
		}
	}
	fixedNow := func() time.Time { return time.Unix(1_785_432_000, 0) }
	return createRateLimiterMiddleware(config, client, routeKey, fixedNow)
}

func performRateLimitRequest(handler http.Handler, host, target string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, target, nil)
	request.Host = host
	request.RemoteAddr = "192.0.2.1:1234"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func startRateLimitTestOlric(t *testing.T) *olric.EmbeddedClient {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	started := make(chan struct{})
	config := olricConfig.New("local")
	config.BindAddr = "127.0.0.1"
	config.BindPort = port
	config.MemberlistConfig.BindAddr = "127.0.0.1"
	config.MemberlistConfig.BindPort = 0
	config.MemberlistConfig.Name = net.JoinHostPort(config.BindAddr, strconv.Itoa(config.BindPort))
	config.LogOutput = nil
	config.Started = func() { close(started) }
	embedded, err := olric.New(config)
	if err != nil {
		t.Fatal(err)
	}
	errCh := make(chan error, 1)
	go func() { errCh <- embedded.Start() }()
	select {
	case <-started:
	case err := <-errCh:
		t.Fatalf("Olric startup: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Olric")
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = embedded.Shutdown(ctx)
	})
	return embedded.NewEmbeddedClient()
}
