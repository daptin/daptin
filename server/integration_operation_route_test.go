package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestIntegrationOperationRoutesAllowSlashesInOperationID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/integration/:providerName/operations", func(c *gin.Context) {
		c.String(http.StatusOK, "list")
	})
	router.GET("/integration/:providerName/operations/*operationName", func(c *gin.Context) {
		c.String(http.StatusOK, integrationOperationNameParam(c))
	})
	router.POST("/integration/:providerName/*operationName", func(c *gin.Context) {
		c.String(http.StatusOK, integrationOperationNameParam(c))
	})

	tests := []struct {
		name   string
		method string
		path   string
		want   string
	}{
		{
			name:   "operations list keeps exact route",
			method: http.MethodGet,
			path:   "/integration/github.com/operations",
			want:   "list",
		},
		{
			name:   "describe accepts escaped slash operation id",
			method: http.MethodGet,
			path:   "/integration/github.com/operations/repos%2Fget",
			want:   "repos/get",
		},
		{
			name:   "execute accepts escaped slash operation id",
			method: http.MethodPost,
			path:   "/integration/github.com/repos%2Fget",
			want:   "repos/get",
		},
		{
			name:   "execute keeps legacy one segment operation id",
			method: http.MethodPost,
			path:   "/integration/asana.com/getWorkspaces",
			want:   "getWorkspaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
			}
			if recorder.Body.String() != tt.want {
				t.Fatalf("body = %q, want %q", recorder.Body.String(), tt.want)
			}
		})
	}
}

func TestSanitizeProviderScopedIntegrationInputRemovesRuntimeFields(t *testing.T) {
	input := map[string]interface{}{
		"oauth_token_id":      "input-oauth",
		"credential_id":       "input-credential",
		"sessionUser":         "input-session",
		"requestSessionUser":  "input-request-session",
		"httpRequest":         "input-request",
		"httpRequestHeaders":  "input-headers",
		"provider_field":      "kept",
		"provider_credential": "kept",
	}

	sanitizeProviderScopedIntegrationInput(input)

	for _, key := range []string{"oauth_token_id", "credential_id", "sessionUser", "requestSessionUser", "httpRequest", "httpRequestHeaders"} {
		if _, ok := input[key]; ok {
			t.Fatalf("runtime key %q was not removed: %#v", key, input)
		}
	}
	if input["provider_field"] != "kept" || input["provider_credential"] != "kept" {
		t.Fatalf("provider fields were not preserved: %#v", input)
	}
}
