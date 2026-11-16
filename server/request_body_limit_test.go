package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
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
			router.Use(bufferedRequestBodyMiddleware(limit, 20))
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

func TestMailDraftRequestLimitUsesAuthenticatedPostRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const path = "/action/mail_account/draft_save"
	for _, test := range []struct {
		name, method, path string
		length             int
		user, chunked      bool
		want               int
	}{
		{"authorized draft uses its budget", "POST", path, 20, true, false, 200},
		{"authorized draft still bounded", "POST", path, 21, true, false, 413},
		{"authorized chunked draft still bounded", "POST", path, 21, true, true, 413},
		{"guest uses general budget", "POST", path, 13, false, false, 413},
		{"other method uses general budget", "PUT", path, 13, true, false, 413},
		{"other action uses general budget", "POST", "/action/mail_account/submit", 13, true, false, 413},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			router.Use(bufferedRequestBodyMiddleware(12, 20))
			router.Any("/*path", func(c *gin.Context) {
				body, err := io.ReadAll(c.Request.Body)
				if err != nil || string(body) != strings.Repeat("x", test.length) {
					t.Fatalf("handler received %q: %v", body, err)
				}
				c.Status(200)
			})
			req := httptest.NewRequest(test.method, test.path, strings.NewReader(strings.Repeat("x", test.length)))
			if test.user {
				req = req.WithContext(context.WithValue(req.Context(), "user", &auth.SessionUser{UserId: 42, UserReferenceId: daptinid.DaptinReferenceId{1}}))
			}
			if test.chunked {
				req.ContentLength = -1
				req.TransferEncoding = []string{"chunked"}
			}
			result := httptest.NewRecorder()
			router.ServeHTTP(result, req)
			if result.Code != test.want {
				t.Fatalf("status = %d, want %d", result.Code, test.want)
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
