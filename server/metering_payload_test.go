package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/daptin/daptin/server/resource"
	"github.com/gin-gonic/gin"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"
)

func TestGraphQLBodyLimitRejectsOversizedRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	query := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"ping": &graphql.Field{Type: graphql.String, Resolve: func(graphql.ResolveParams) (interface{}, error) {
				return "pong", nil
			}},
		},
	})
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: query})
	if err != nil {
		t.Fatal(err)
	}
	graphqlHandler := handler.New(&handler.Config{Schema: &schema})
	cruds := map[string]*resource.DbResource{}
	router := gin.New()
	router.Use(meteringPayloadMiddleware(&cruds, defaultGraphQLRequestBodyLimit))
	router.POST("/graphql", func(c *gin.Context) { graphqlHandler.ServeHTTP(c.Writer, c.Request) })
	router.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })

	for _, tc := range []struct {
		name       string
		size       int
		chunked    bool
		wantStatus int
	}{
		{name: "valid", size: 128, wantStatus: http.StatusOK},
		{name: "at limit", size: defaultGraphQLRequestBodyLimit, wantStatus: http.StatusOK},
		{name: "known length over limit", size: defaultGraphQLRequestBodyLimit + 1, wantStatus: http.StatusRequestEntityTooLarge},
		{name: "chunked over limit", size: defaultGraphQLRequestBodyLimit + 1, chunked: true, wantStatus: http.StatusRequestEntityTooLarge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const prefix = `{"query":"`
			const suffix = `query { ping }"}`
			body := prefix + strings.Repeat(" ", tc.size-len(prefix)-len(suffix)) + suffix
			request := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			if tc.chunked {
				request.ContentLength = -1
				request.TransferEncoding = []string{"chunked"}
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d: %s", response.Code, tc.wantStatus, response.Body.String())
			}
			if tc.wantStatus == http.StatusOK && !strings.Contains(response.Body.String(), `"ping"`) {
				t.Fatalf("GraphQL response = %s", response.Body.String())
			}
			if tc.wantStatus == http.StatusRequestEntityTooLarge && response.Body.Len() != 0 {
				t.Fatalf("oversized body reached GraphQL: %s", response.Body.String())
			}
			ping := httptest.NewRecorder()
			router.ServeHTTP(ping, httptest.NewRequest(http.MethodGet, "/ping", nil))
			if ping.Code != http.StatusOK || ping.Body.String() != "pong" {
				t.Fatalf("/ping unavailable after request: %d %q", ping.Code, ping.Body.String())
			}
		})
	}
}

func TestGraphQLOperationSelectionLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resolved := 0
	query := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"ping": &graphql.Field{Type: graphql.String, Resolve: func(graphql.ResolveParams) (interface{}, error) {
				resolved++
				return "pong", nil
			}},
		},
	})
	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: query})
	if err != nil {
		t.Fatal(err)
	}
	graphqlHandler := handler.New(&handler.Config{Schema: &schema})
	cruds := map[string]*resource.DbResource{}
	router := gin.New()
	router.Use(meteringPayloadMiddleware(&cruds, defaultGraphQLRequestBodyLimit))
	router.POST("/graphql", func(c *gin.Context) { graphqlHandler.ServeHTTP(c.Writer, c.Request) })
	router.GET("/graphql", func(c *gin.Context) { graphqlHandler.ServeHTTP(c.Writer, c.Request) })

	aliasFields := make([]string, maxGraphQLOperationSelections+1)
	for i := range aliasFields {
		aliasFields[i] = fmt.Sprintf("a%d: ping", i)
	}
	for _, tc := range []struct {
		name        string
		method      string
		contentType string
		query       string
		wantStatus  int
		wantResolve int
	}{
		{"at limit", http.MethodPost, "application/json", "{" + strings.Join(aliasFields[:maxGraphQLOperationSelections], " ") + "}", http.StatusOK, maxGraphQLOperationSelections},
		{"aliases over limit", http.MethodPost, "application/json", "{" + strings.Join(aliasFields, " ") + "}", http.StatusBadRequest, 0},
		{"repeated fragment over limit", http.MethodPost, "application/json", "{ " + strings.Repeat("...P ", 129) + "} fragment P on Query { ping }", http.StatusBadRequest, 0},
		{"nested fragment over limit", http.MethodPost, "application/json", "{ ...P } fragment P on Query { " + strings.Repeat("...Q ", 129) + "} fragment Q on Query { ping }", http.StatusBadRequest, 0},
		{"inline fragment over limit", http.MethodPost, "application/json", "{ ... on Query { " + strings.Join(aliasFields, " ") + " } }", http.StatusBadRequest, 0},
		{"raw GraphQL over limit", http.MethodPost, "application/graphql", "{" + strings.Join(aliasFields, " ") + "}", http.StatusBadRequest, 0},
		{"form GraphQL over limit", http.MethodPost, "application/x-www-form-urlencoded", "{" + strings.Join(aliasFields, " ") + "}", http.StatusBadRequest, 0},
		{"GET aliases over limit", http.MethodGet, "", "{" + strings.Join(aliasFields, " ") + "}", http.StatusBadRequest, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resolved = 0
			var request *http.Request
			if tc.method == http.MethodGet {
				request = httptest.NewRequest(http.MethodGet, "/graphql?query="+url.QueryEscape(tc.query), nil)
			} else {
				body := fmt.Sprintf(`{"query":%q}`, tc.query)
				if tc.contentType == "application/graphql" {
					body = tc.query
				} else if tc.contentType == "application/x-www-form-urlencoded" {
					body = url.Values{"query": {tc.query}}.Encode()
				}
				request = httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
				request.Header.Set("Content-Type", tc.contentType)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tc.wantStatus || resolved != tc.wantResolve {
				t.Fatalf("status = %d, resolved = %d, want %d and %d; body = %s", response.Code, resolved, tc.wantStatus, tc.wantResolve, response.Body.String())
			}
		})
	}
}

func TestGraphQLBodyLimitConfiguration(t *testing.T) {
	for _, value := range []string{"1", "10485760", "67108864"} {
		if _, err := parseGraphQLRequestBodyLimit(value); err != nil {
			t.Fatalf("valid limit %q: %v", value, err)
		}
	}
	for _, value := range []string{"", "0", "-1", "67108865", "unlimited"} {
		if _, err := parseGraphQLRequestBodyLimit(value); err == nil {
			t.Fatalf("invalid limit %q was accepted", value)
		}
	}

	cruds := map[string]*resource.DbResource{}
	router := gin.New()
	router.Use(meteringPayloadMiddleware(&cruds, 32))
	router.POST("/graphql", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	for _, tc := range []struct {
		size int
		want int
	}{{32, http.StatusNoContent}, {33, http.StatusRequestEntityTooLarge}} {
		request := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(strings.Repeat("x", tc.size)))
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != tc.want {
			t.Fatalf("size %d: status = %d, want %d", tc.size, response.Code, tc.want)
		}
	}
}
