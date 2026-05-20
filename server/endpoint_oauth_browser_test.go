package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/daptin/daptin/server/actionresponse"
	"github.com/gin-gonic/gin"
)

func TestSafeSameOriginPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		fallback string
		want     string
	}{
		{
			name:     "authorize path with query",
			input:    "/oauth/authorize?client_id=dapc&redirect_uri=https%3A%2F%2Fapp.example%2Fcallback",
			fallback: "/",
			want:     "/oauth/authorize?client_id=dapc&redirect_uri=https%3A%2F%2Fapp.example%2Fcallback",
		},
		{
			name:     "absolute url rejected",
			input:    "https://evil.example/callback",
			fallback: "/",
			want:     "/",
		},
		{
			name:     "protocol relative rejected",
			input:    "//evil.example/callback",
			fallback: "/safe",
			want:     "/safe",
		},
		{
			name:     "relative path rejected",
			input:    "oauth/authorize",
			fallback: "/safe",
			want:     "/safe",
		},
		{
			name:     "backslash rejected",
			input:    "/\\evil.example",
			fallback: "/safe",
			want:     "/safe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := safeSameOriginPath(tt.input, tt.fallback); got != tt.want {
				t.Fatalf("safeSameOriginPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOAuthSigninPageHandlerSanitizesReturnTo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/auth/signin", oauthSigninPageHandler())

	req := httptest.NewRequest(http.MethodGet, "/auth/signin?return_to=https%3A%2F%2Fevil.example%2Fcallback", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if strings.Contains(body, "evil.example") {
		t.Fatalf("unsafe return_to leaked into form: %s", body)
	}
	if !strings.Contains(body, `name="return_to" value="/"`) {
		t.Fatalf("expected sanitized return_to hidden field, got: %s", body)
	}
}

func TestApplyCookieResponsesSetsHttpOnlyCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/oauth/response", nil)

	applyCookieResponses(c, []actionresponse.ActionResponse{{
		ResponseType: "client.cookie.set",
		Attributes: map[string]interface{}{
			"key":   "token",
			"value": "jwt-value; SameSite=Strict",
		},
	}})

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookie count = %d, want 1", len(cookies))
	}
	if cookies[0].Name != "token" || cookies[0].Value != "jwt-value" {
		t.Fatalf("unexpected cookie: %#v", cookies[0])
	}
	if !cookies[0].HttpOnly {
		t.Fatalf("expected HttpOnly cookie")
	}
	if cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("SameSite = %v, want strict", cookies[0].SameSite)
	}
}

func TestLastSafeRedirectUsesLastSafeClientRedirect(t *testing.T) {
	responses := []actionresponse.ActionResponse{
		{
			ResponseType: "client.redirect",
			Attributes:   map[string]interface{}{"location": "/in/item/oauth_token"},
		},
		{
			ResponseType: "client.redirect",
			Attributes:   map[string]interface{}{"location": "https://evil.example"},
		},
		{
			ResponseType: "client.redirect",
			Attributes:   map[string]interface{}{"location": "/"},
		},
	}

	if got := lastSafeRedirect(responses, "/fallback"); got != "/" {
		t.Fatalf("lastSafeRedirect() = %q, want /", got)
	}
}

func TestLastRedirectAllowsProviderURL(t *testing.T) {
	responses := []actionresponse.ActionResponse{
		{
			ResponseType: "client.redirect",
			Attributes:   map[string]interface{}{"location": "https://provider.example/oauth/authorize?state=abc"},
		},
	}

	if got := lastRedirect(responses); got != "https://provider.example/oauth/authorize?state=abc" {
		t.Fatalf("lastRedirect() = %q", got)
	}
}

func TestOAuthBrowserBool(t *testing.T) {
	for _, value := range []interface{}{true, 1, int64(1), "true", "yes", []byte("on")} {
		if !oauthBrowserBool(value) {
			t.Fatalf("oauthBrowserBool(%#v) = false, want true", value)
		}
	}
	for _, value := range []interface{}{false, 0, int64(0), "false", "", nil} {
		if oauthBrowserBool(value) {
			t.Fatalf("oauthBrowserBool(%#v) = true, want false", value)
		}
	}
}
