package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestBufferedRequestBodyLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const limit = 12
	for _, tc := range []struct {
		name, method, path string
		body               string
		chunked            bool
		wantStatus         int
	}{
		{"action", http.MethodPost, "/action/world/export_data", "1234567890123", false, http.StatusRequestEntityTooLarge},
		{"action GET", http.MethodGet, "/action/world/export_data", "1234567890123", false, http.StatusRequestEntityTooLarge},
		{"action chunked", http.MethodPost, "/action/world/export_data", "1234567890123", true, http.StatusRequestEntityTooLarge},
		{"JSON API create", http.MethodPost, "/api/world", "1234567890123", false, http.StatusRequestEntityTooLarge},
		{"JSON API update", http.MethodPatch, "/api/world/x", "1234567890123", true, http.StatusRequestEntityTooLarge},
		{"integration", http.MethodPost, "/integration/test/run", "1234567890123", false, http.StatusRequestEntityTooLarge},
		{"event start", http.MethodPost, "/track/start/test", "1234567890123", false, http.StatusRequestEntityTooLarge},
		{"at limit", http.MethodPost, "/action/world/export_data", "123456789012", false, http.StatusOK},
		{"chunked at limit", http.MethodPost, "/api/world", "123456789012", true, http.StatusOK},
		{"asset upload", http.MethodPost, "/asset/world/x/file/upload", "1234567890123", false, http.StatusOK},
		{"other protocol", http.MethodPost, "/caldav/user", "1234567890123", false, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			router := gin.New()
			router.Use(bufferedRequestBodyMiddleware(limit))
			router.Any("/*path", func(c *gin.Context) {
				called = true
				body, err := io.ReadAll(c.Request.Body)
				if err != nil || string(body) != tc.body {
					t.Errorf("downstream body = %q, error = %v", body, err)
				}
				c.Status(http.StatusOK)
			})
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.chunked {
				request.ContentLength = -1
				request.TransferEncoding = []string{"chunked"}
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tc.wantStatus || called != (tc.wantStatus == http.StatusOK) {
				t.Fatalf("status = %d, handler called = %t; want %d", response.Code, called, tc.wantStatus)
			}
		})
	}
}

func TestBufferedRequestBodyLimitConfiguration(t *testing.T) {
	for _, value := range []string{"1", "10485760", "16777216"} {
		if _, err := parseBufferedRequestBodyLimit(value); err != nil {
			t.Fatalf("valid limit %q: %v", value, err)
		}
	}
	for _, value := range []string{"", "0", "-1", "16777217", "unlimited"} {
		if _, err := parseBufferedRequestBodyLimit(value); err == nil {
			t.Fatalf("invalid limit %q was accepted", value)
		}
	}
}
