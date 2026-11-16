package subsite

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/daptin/daptin/server/dbresourceinterface"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

func routeTestRow(name, paths string) map[string]interface{} {
	return map[string]interface{}{"name": name, "reference_id": name + "-reference", "url_pattern": paths}
}

func TestTemplateRoutesCompileBeforeMounting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name string
		rows []map[string]interface{}
	}{
		{"built-in GET", []map[string]interface{}{routeTestRow("shell", `["/ok","/meta"]`)}},
		{"later Any method conflict", []map[string]interface{}{routeTestRow("shell", `["/ok","/post-only"]`)}},
		{"other router conflict", []map[string]interface{}{routeTestRow("shell", `["/ok","/stats"]`)}},
		{"duplicate within template", []map[string]interface{}{routeTestRow("shell", `["/ok","/ok"]`)}},
		{"duplicate across templates", []map[string]interface{}{routeTestRow("first", `["/ok"]`), routeTestRow("second", `["/ok"]`)}},
		{"catch-all conflicts with built-in", []map[string]interface{}{routeTestRow("shell", `["/*path"]`)}},
		{"different wildcard names", []map[string]interface{}{routeTestRow("first", `["/users/:name"]`), routeTestRow("second", `["/users/:id"]`)}},
		{"malformed wildcard", []map[string]interface{}{routeTestRow("shell", `["/ok","/bad/*path/more"]`)}},
		{"relative path", []map[string]interface{}{routeTestRow("shell", `["relative"]`)}},
		{"empty path", []map[string]interface{}{routeTestRow("shell", `[""]`)}},
		{"JSON object", []map[string]interface{}{routeTestRow("shell", `{}`)}},
		{"JSON null", []map[string]interface{}{routeTestRow("shell", `null`)}},
		{"non-string element", []map[string]interface{}{routeTestRow("shell", `[42]`)}},
		{"invalid JSON", []map[string]interface{}{routeTestRow("shell", `["/ok"`)}},
		{"missing pattern", []map[string]interface{}{{"name": "broken"}}},
		{"missing name", []map[string]interface{}{{"url_pattern": `["/ok"]`}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			dashboard, site := gin.New(), gin.New()
			dashboard.GET("/meta", func(c *gin.Context) { c.String(200, "builtin") })
			dashboard.POST("/post-only", func(c *gin.Context) { c.String(200, "builtin") })
			site.GET("/stats", func(c *gin.Context) { c.String(200, "site builtin") })
			signature := func(router *gin.Engine) []string {
				var result []string
				for _, route := range router.Routes() {
					result = append(result, route.Method+" "+route.Path+" "+route.Handler)
				}
				return result
			}
			beforeDashboard, beforeSite := signature(dashboard), signature(site)
			built := 0
			err := registerTemplateRoutes(test.rows, []*gin.Engine{dashboard, site}, func(row map[string]interface{}) func(*gin.Context) { built++; return func(*gin.Context) {} })
			if !errors.Is(err, ErrInvalidTemplateRoutes) {
				t.Fatalf("error=%v, want invalid configuration", err)
			}
			if built != 0 {
				t.Fatal("constructed handlers before full validation")
			}
			if !reflect.DeepEqual(signature(dashboard), beforeDashboard) || !reflect.DeepEqual(signature(site), beforeSite) {
				t.Fatal("invalid batch changed live routers")
			}
			if !strings.Contains(err.Error(), "template") {
				t.Fatalf("error lacks template identity: %v", err)
			}
		})
	}
}

func TestTemplateRoutesUseGinSemanticsAndUniqueRouters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dashboard, site := gin.New(), gin.New()
	for _, router := range []*gin.Engine{dashboard, site} {
		router.NoRoute(func(c *gin.Context) { c.String(200, "fallback") })
	}
	dashboard.GET("/meta", func(c *gin.Context) { c.String(200, "builtin") })
	rows := []map[string]interface{}{routeTestRow("root", `["/"]`), routeTestRow("users", `["/users/:name"]`), routeTestRow("docs", `["/docs/*path"]`), routeTestRow("action-only", `[]`)}
	built := 0
	err := registerTemplateRoutes(rows, []*gin.Engine{dashboard, site, dashboard}, func(row map[string]interface{}) func(*gin.Context) {
		built++
		return func(c *gin.Context) {
			c.String(200, fmt.Sprintf("%s:%s:%s", row["name"], c.Param("name"), c.Param("path")))
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if built != 3 {
		t.Fatalf("created %d handlers, want one per routed template", built)
	}
	for _, router := range []*gin.Engine{dashboard, site} {
		for _, test := range []struct{ method, path, body string }{{"GET", "/", "root::"}, {"POST", "/", "root::"}, {"GET", "/users/alice", "users:alice:"}, {"DELETE", "/docs/guide", "docs::/guide"}, {"GET", "/unmatched/deep", "fallback"}} {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))
			if response.Code != 200 || response.Body.String() != test.body {
				t.Fatalf("%s %s: status=%d body=%q", test.method, test.path, response.Code, response.Body.String())
			}
		}
	}
	response := httptest.NewRecorder()
	dashboard.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/meta", nil))
	if response.Body.String() != "builtin" {
		t.Fatal("template replaced builtin")
	}
}

// Embed the shared interface so this stub overrides only the read exercised by
// startup; infrastructure failure must not be treated as bad route metadata.
type templateRouteLoadFailure struct {
	dbresourceinterface.DbResourceInterface
	cause error
}

func (failure templateRouteLoadFailure) GetAllObjects(string, *sqlx.Tx) ([]map[string]interface{}, error) {
	return nil, failure.cause
}

func TestTemplateHooksPropagateLoadFailure(t *testing.T) {
	cause := errors.New("database unavailable")
	err := CreateTemplateHooks(nil, map[string]dbresourceinterface.DbResourceInterface{"template": templateRouteLoadFailure{cause: cause}}, nil, nil, false)
	if !errors.Is(err, cause) || errors.Is(err, ErrInvalidTemplateRoutes) {
		t.Fatalf("infrastructure error misclassified: %v", err)
	}
}
