package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestParseCorsConfig(t *testing.T) {
	if _, err := ParseCorsConfig(defaultCorsConfigJSON); err != nil {
		t.Fatalf("default cors config is invalid: %v", err)
	}
	config, err := ParseCorsConfig(`{
		"version":"1",
		"allowed_origins":["https://API.Example.com:8443","http://localhost:3000","https://Default.Example.com:443","http://Plain.Example.com:80"],
		"allowed_methods":["post","GET"],
		"allowed_headers":["content-type","authorization"],
		"exposed_headers":["x-request-id"],
		"allow_credentials":true,
		"max_age":600
	}`)
	if err != nil {
		t.Fatalf("ParseCorsConfig: %v", err)
	}
	if got := strings.Join(config.AllowedOrigins, ","); got != "http://localhost:3000,http://plain.example.com,https://api.example.com:8443,https://default.example.com" {
		t.Fatalf("origins = %q", got)
	}
	if got := strings.Join(config.AllowedMethods, ","); got != "GET,POST" {
		t.Fatalf("methods = %q", got)
	}
	if got := strings.Join(config.AllowedHeaders, ","); got != "Authorization,Content-Type" {
		t.Fatalf("headers = %q", got)
	}
	if !config.AllowCredentials || config.MaxAge != 600 {
		t.Fatalf("unexpected config: %#v", config)
	}
}

func TestParseCorsConfigRejectsInvalidValues(t *testing.T) {
	tests := map[string]string{
		"malformed":                  `not-json`,
		"unknown field":              `{"version":"1","extra":true}`,
		"unsupported version":        `{"version":"2"}`,
		"wildcard origin":            `{"version":"1","allowed_origins":["*"]}`,
		"null origin":                `{"version":"1","allowed_origins":["null"]}`,
		"non-http origin":            `{"version":"1","allowed_origins":["file://example"]}`,
		"origin path":                `{"version":"1","allowed_origins":["https://example.com/path"]}`,
		"origin query":               `{"version":"1","allowed_origins":["https://example.com?q=1"]}`,
		"origin fragment":            `{"version":"1","allowed_origins":["https://example.com#fragment"]}`,
		"origin credentials":         `{"version":"1","allowed_origins":["https://user@example.com"]}`,
		"origin trailing slash":      `{"version":"1","allowed_origins":["https://example.com/"]}`,
		"duplicate origin":           `{"version":"1","allowed_origins":["https://EXAMPLE.com","https://example.com"]}`,
		"duplicate default port":     `{"version":"1","allowed_origins":["https://example.com","https://example.com:443"]}`,
		"invalid method":             `{"version":"1","allowed_methods":["GET POST"]}`,
		"unsafe method":              `{"version":"1","allowed_methods":["TRACE"]}`,
		"unsupported custom method":  `{"version":"1","allowed_methods":["CUSTOM"]}`,
		"duplicate method":           `{"version":"1","allowed_methods":["get","GET"]}`,
		"invalid allowed header":     `{"version":"1","allowed_headers":["Bad Header"]}`,
		"duplicate allowed header":   `{"version":"1","allowed_headers":["authorization","Authorization"]}`,
		"invalid exposed header":     `{"version":"1","exposed_headers":["Bad Header"]}`,
		"credentials without origin": `{"version":"1","allow_credentials":true}`,
		"negative max age":           `{"version":"1","max_age":-1}`,
		"excessive max age":          fmt.Sprintf(`{"version":"1","max_age":%d}`, maxCorsMaxAge+1),
	}
	for name, value := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseCorsConfig(value); err == nil {
				t.Fatalf("ParseCorsConfig(%q) unexpectedly succeeded", value)
			}
		})
	}
}

func TestCorsMiddlewareRequests(t *testing.T) {
	config := mustParseCorsConfig(t, `{
		"version":"1",
		"allowed_origins":["https://app.example.com"],
		"allowed_methods":["GET","POST"],
		"allowed_headers":["Authorization","Content-Type"],
		"exposed_headers":["X-Request-Id"],
		"allow_credentials":true,
		"max_age":600
	}`)

	tests := []struct {
		name            string
		origin          string
		wantOrigin      string
		wantCredentials string
		wantExpose      string
	}{
		{name: "missing origin"},
		{name: "allowed exact origin", origin: "https://app.example.com", wantOrigin: "https://app.example.com", wantCredentials: "true", wantExpose: "X-Request-Id"},
		{name: "allowed origin case normalized", origin: "https://APP.EXAMPLE.COM", wantOrigin: "https://APP.EXAMPLE.COM", wantCredentials: "true", wantExpose: "X-Request-Id"},
		{name: "disallowed origin", origin: "https://attacker.example"},
		{name: "lookalike origin", origin: "https://app.example.com.attacker.example"},
		{name: "opaque origin", origin: "null"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router, calls := newCorsTestRouter(config, "Accept-Encoding")
			request := httptest.NewRequest(http.MethodGet, "/resource", nil)
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusNoContent || *calls != 1 {
				t.Fatalf("status/calls = %d/%d, want 204/1", response.Code, *calls)
			}
			if got := response.Header().Get("Access-Control-Allow-Origin"); got != test.wantOrigin {
				t.Fatalf("allow origin = %q, want %q", got, test.wantOrigin)
			}
			if got := response.Header().Get("Access-Control-Allow-Credentials"); got != test.wantCredentials {
				t.Fatalf("credentials = %q, want %q", got, test.wantCredentials)
			}
			if got := response.Header().Get("Access-Control-Expose-Headers"); got != test.wantExpose {
				t.Fatalf("expose headers = %q, want %q", got, test.wantExpose)
			}
			if !headerContainsToken(response.Header(), "Vary", "Origin") {
				t.Fatal("Vary does not contain Origin")
			}
			if !headerContainsToken(response.Header(), "Vary", "Accept-Encoding") {
				t.Fatal("existing Vary value was lost")
			}
		})
	}
}

func TestCorsMiddlewarePreflight(t *testing.T) {
	config := mustParseCorsConfig(t, `{
		"version":"1",
		"allowed_origins":["https://app.example.com"],
		"allowed_methods":["GET","POST"],
		"allowed_headers":["Authorization","Content-Type"],
		"allow_credentials":true,
		"max_age":600
	}`)
	tests := []struct {
		name       string
		origin     string
		method     string
		headers    string
		wantStatus int
	}{
		{name: "allowed", origin: "https://app.example.com", method: "post", headers: "content-type, AUTHORIZATION", wantStatus: http.StatusNoContent},
		{name: "allowed without headers", origin: "https://app.example.com", method: "GET", wantStatus: http.StatusNoContent},
		{name: "disallowed origin", origin: "https://attacker.example", method: "POST", headers: "Content-Type", wantStatus: http.StatusForbidden},
		{name: "null origin", origin: "null", method: "POST", wantStatus: http.StatusForbidden},
		{name: "disallowed method", origin: "https://app.example.com", method: "DELETE", wantStatus: http.StatusForbidden},
		{name: "missing method", origin: "https://app.example.com", wantStatus: http.StatusForbidden},
		{name: "one disallowed header", origin: "https://app.example.com", method: "POST", headers: "Content-Type, X-Admin", wantStatus: http.StatusForbidden},
		{name: "malformed requested header", origin: "https://app.example.com", method: "POST", headers: "Bad Header", wantStatus: http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router, calls := newCorsTestRouter(config, "Accept-Encoding")
			request := httptest.NewRequest(http.MethodOptions, "/resource", nil)
			request.Header.Set("Origin", test.origin)
			request.Header.Set("Access-Control-Request-Method", test.method)
			if test.headers != "" {
				request.Header.Set("Access-Control-Request-Headers", test.headers)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if *calls != 0 {
				t.Fatalf("downstream calls = %d, want 0", *calls)
			}
			if test.wantStatus == http.StatusNoContent {
				if got := response.Header().Get("Access-Control-Allow-Origin"); got != test.origin {
					t.Fatalf("allow origin = %q, want %q", got, test.origin)
				}
				if got := response.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST" {
					t.Fatalf("allow methods = %q", got)
				}
				if got := response.Header().Get("Access-Control-Allow-Headers"); got != "Authorization, Content-Type" {
					t.Fatalf("allow headers = %q", got)
				}
				if got := response.Header().Get("Access-Control-Max-Age"); got != "600" {
					t.Fatalf("max age = %q", got)
				}
				for _, vary := range []string{"Origin", "Access-Control-Request-Method", "Access-Control-Request-Headers", "Accept-Encoding"} {
					if !headerContainsToken(response.Header(), "Vary", vary) {
						t.Fatalf("Vary does not contain %q: %v", vary, response.Header().Values("Vary"))
					}
				}
			} else if response.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("rejected preflight emitted allow-origin")
			}
		})
	}
}

func TestCorsMiddlewareCredentialsDisabled(t *testing.T) {
	config := mustParseCorsConfig(t, `{"version":"1","allowed_origins":["https://app.example.com"],"allowed_methods":["GET"]}`)
	router, _ := newCorsTestRouter(config, "")
	request := httptest.NewRequest(http.MethodGet, "/resource", nil)
	request.Header.Set("Origin", "https://app.example.com")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Fatal("credentials header was emitted while disabled")
	}
}

func TestCorsMiddlewareNormalizesDefaultOriginPorts(t *testing.T) {
	config := mustParseCorsConfig(t, `{"version":"1","allowed_origins":["https://secure.example.com:443","http://plain.example.com:80","https://custom.example.com:8443"],"allowed_methods":["GET"]}`)
	tests := []struct {
		name   string
		origin string
	}{
		{name: "https browser serialization", origin: "https://secure.example.com"},
		{name: "https explicit default port", origin: "https://secure.example.com:443"},
		{name: "http browser serialization", origin: "http://plain.example.com"},
		{name: "http explicit default port", origin: "http://plain.example.com:80"},
		{name: "non-default port", origin: "https://custom.example.com:8443"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router, _ := newCorsTestRouter(config, "")
			request := httptest.NewRequest(http.MethodGet, "/resource", nil)
			request.Header.Set("Origin", test.origin)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if got := response.Header().Get("Access-Control-Allow-Origin"); got != test.origin {
				t.Fatalf("allow origin = %q, want %q", got, test.origin)
			}
		})
	}
}

func TestCorsMiddlewareOptionsWithoutRequestedMethodIsRejected(t *testing.T) {
	config := mustParseCorsConfig(t, `{"version":"1","allowed_origins":["https://app.example.com"],"allowed_methods":["GET"]}`)
	router, calls := newCorsTestRouter(config, "")
	request := httptest.NewRequest(http.MethodOptions, "/resource", nil)
	request.Header.Set("Origin", "https://app.example.com")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if *calls != 0 || response.Code != http.StatusForbidden {
		t.Fatalf("plain OPTIONS status/calls = %d/%d, want 403/0", response.Code, *calls)
	}
}

func mustParseCorsConfig(t *testing.T, value string) CorsConfig {
	t.Helper()
	config, err := ParseCorsConfig(value)
	if err != nil {
		t.Fatal(err)
	}
	return config
}

func newCorsTestRouter(config CorsConfig, existingVary string) (*gin.Engine, *int) {
	gin.SetMode(gin.TestMode)
	calls := 0
	router := gin.New()
	if existingVary != "" {
		router.Use(func(c *gin.Context) {
			c.Writer.Header().Add("Vary", existingVary)
			c.Next()
		})
	}
	router.Use(NewCorsMiddleware(config).CorsMiddlewareFunc)
	router.Any("/resource", func(c *gin.Context) {
		calls++
		c.Status(http.StatusNoContent)
	})
	return router, &calls
}

func headerContainsToken(header http.Header, name, token string) bool {
	for _, line := range header.Values(name) {
		for _, value := range strings.Split(line, ",") {
			if strings.EqualFold(strings.TrimSpace(value), token) {
				return true
			}
		}
	}
	return false
}
