package server

import (
	"net/http"

	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/resource"
	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// InitializeCaldavResources sets up CalDAV/CardDAV endpoints
// Pattern: like InitializeImapResources - receives cruds parameter
func InitializeCaldavResources(
	authMiddleware *auth.AuthMiddleware,
	cruds map[string]*resource.DbResource,
	defaultRouter *gin.Engine) {

	logrus.Printf("[CALDAV ENDPOINT] InitializeCaldavResources called - starting CalDAV setup")
	logrus.Tracef("Process caldav")

	davHandler := func(protocol string) gin.HandlerFunc {
		return func(c *gin.Context) {
			authRequest := c.Request
			if c.Request.Method == http.MethodOptions {
				authRequest = c.Request.Clone(c.Request.Context())
				authRequest.Method = http.MethodGet
			}
			ok, abort, authenticatedRequest := authMiddleware.AuthCheckMiddlewareWithHttp(authRequest, c.Writer, true)
			if !ok || abort {
				c.Header("WWW-Authenticate", `Basic realm="`+protocol+`"`)
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}

			sessionUser, ok := authenticatedRequest.Context().Value("user").(*auth.SessionUser)
			if !ok || sessionUser == nil {
				c.Header("WWW-Authenticate", `Basic realm="`+protocol+`"`)
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			modifiedRequest := c.Request.WithContext(authenticatedRequest.Context())

			if protocol == "caldav" {
				(&caldav.Handler{Backend: resource.NewCalDAVBackend(cruds, sessionUser), Prefix: "/caldav"}).ServeHTTP(c.Writer, modifiedRequest)
			} else {
				(&carddav.Handler{Backend: resource.NewCardDAVBackend(cruds, sessionUser), Prefix: "/carddav"}).ServeHTTP(c.Writer, modifiedRequest)
			}
		}
	}

	methods := []string{"OPTIONS", "HEAD", "GET", "PUT", "PROPFIND", "REPORT", "DELETE", "MKCOL", "PROPPATCH", "COPY", "MOVE"}
	for _, method := range methods {
		defaultRouter.Handle(method, "/caldav/*path", davHandler("caldav"))
		defaultRouter.Handle(method, "/carddav/*path", davHandler("carddav"))
	}

	// Well-known URIs for service discovery (RFC 6764)
	// Allows clients to auto-discover CalDAV/CardDAV endpoints
	defaultRouter.GET("/.well-known/caldav", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/caldav/")
	})
	defaultRouter.GET("/.well-known/carddav", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/carddav/")
	})

	logrus.Printf("[CALDAV ENDPOINT] CalDAV/CardDAV routes registered")
	logrus.Tracef("CalDAV/CardDAV resources initialized")
}
