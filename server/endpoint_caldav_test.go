package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDAVWellKnownPropfindRedirect(t *testing.T) {
	router := gin.New()
	InitializeCaldavResources(nil, nil, router)

	for _, test := range []struct {
		path     string
		location string
	}{
		{"/.well-known/caldav", "/caldav/"},
		{"/.well-known/carddav", "/carddav/"},
	} {
		t.Run(test.path, func(t *testing.T) {
			request := httptest.NewRequest("PROPFIND", test.path, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusMovedPermanently || response.Header().Get("Location") != test.location {
				t.Fatalf("PROPFIND %s: status %d, Location %q", test.path, response.Code, response.Header().Get("Location"))
			}
		})
	}
}
