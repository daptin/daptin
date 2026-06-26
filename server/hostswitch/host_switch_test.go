package hostswitch

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/permission"
	"github.com/daptin/daptin/server/subsite"
	"github.com/gin-gonic/gin"
)

func TestHostSwitchRoutesWellDefinedApiPathsToDashboardRouter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	hostSwitch := testHostSwitch()

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{
			name:       "integration operation",
			method:     http.MethodPost,
			path:       "/integration/github.com/getRepo",
			wantStatus: http.StatusAccepted,
		},
		{
			name:       "integration operation containing slash",
			method:     http.MethodPost,
			path:       "/integration/github.com/repos/get",
			wantStatus: http.StatusAccepted,
		},
		{
			name:       "config",
			method:     http.MethodGet,
			path:       "/_config",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)
			request.Host = "site.example.test"

			hostSwitch.ServeHTTP(recorder, request)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("well-defined API path %q status = %d, want %d", tt.path, recorder.Code, tt.wantStatus)
			}
			if containsAppShell(recorder.Body.String()) {
				t.Fatalf("well-defined API path %q was served dashboard shell: %s", tt.path, recorder.Body.String())
			}
			if recorder.Header().Get("X-Test-Router") != "dashboard" {
				t.Fatalf("well-defined API path %q routed to %q, want dashboard", tt.path, recorder.Header().Get("X-Test-Router"))
			}
		})
	}
}

func TestHostSwitchSubsiteFallbackStillServesAppShell(t *testing.T) {
	gin.SetMode(gin.TestMode)

	hostSwitch := testHostSwitch()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/static/missing.js", nil)
	request.Host = "site.example.test"

	hostSwitch.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("subsite fallback status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if recorder.Header().Get("X-Test-Router") != "subsite" {
		t.Fatalf("subsite fallback routed to %q, want subsite", recorder.Header().Get("X-Test-Router"))
	}
	if !containsAppShell(recorder.Body.String()) {
		t.Fatalf("subsite fallback body did not include app shell: %s", recorder.Body.String())
	}
}

func testHostSwitch() HostSwitch {
	dashboardRouter := gin.New()
	dashboardRouter.POST("/integration/:providerName/*operationName", func(c *gin.Context) {
		c.Header("X-Test-Router", "dashboard")
		c.JSON(http.StatusAccepted, gin.H{
			"provider":  c.Param("providerName"),
			"operation": strings.TrimPrefix(c.Param("operationName"), "/"),
		})
	})
	dashboardRouter.GET("/_config", func(c *gin.Context) {
		c.Header("X-Test-Router", "dashboard")
		c.JSON(http.StatusOK, gin.H{"config": true})
	})
	dashboardRouter.NoRoute(func(c *gin.Context) {
		c.Header("X-Test-Router", "dashboard")
		c.Data(http.StatusOK, "text/html; charset=UTF-8", []byte("<!doctype html><html><body>dashboard</body></html>"))
	})

	subsiteRouter := gin.New()
	subsiteRouter.NoRoute(func(c *gin.Context) {
		c.Header("X-Test-Router", "subsite")
		c.Data(http.StatusOK, "text/html; charset=UTF-8", []byte("<!doctype html><html><body>subsite</body></html>"))
	})

	return HostSwitch{
		HandlerMap: map[string]*gin.Engine{
			"dashboard":         dashboardRouter,
			"site.example.test": subsiteRouter,
		},
		SiteMap: map[string]subsite.SubSite{
			"site.example.test": {
				Hostname: "site.example.test",
				Permission: permission.PermissionInstance{
					Permission: auth.GuestExecute,
				},
			},
		},
		AuthMiddleware:       &auth.AuthMiddleware{},
		AdministratorGroupId: daptinid.NullReferenceId,
	}
}

func containsAppShell(body string) bool {
	return strings.Contains(strings.ToLower(body), "<!doctype html")
}
