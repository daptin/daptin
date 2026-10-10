package server

import (
	"net/http"

	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/go-webdav/caldav"
	"github.com/daptin/go-webdav/carddav"
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
				backend := resource.NewCalDAVBackend(cruds, sessionUser, modifiedRequest.Header)
				if !backend.ServeScheduling(c.Writer, modifiedRequest) {
					(&caldav.Handler{Backend: backend, Prefix: "/caldav"}).ServeHTTP(c.Writer, modifiedRequest)
				}
			} else {
				(&carddav.Handler{Backend: resource.NewCardDAVBackend(cruds, sessionUser, modifiedRequest.Header), Prefix: "/carddav"}).ServeHTTP(c.Writer, modifiedRequest)
			}
		}
	}

	methods := []string{"OPTIONS", "HEAD", "GET", "PUT", "PROPFIND", "REPORT", "DELETE", "MKCOL", "PROPPATCH", "COPY", "MOVE"}
	for _, method := range methods {
		defaultRouter.Handle(method, "/caldav/*path", davHandler("caldav"))
		defaultRouter.Handle(method, "/carddav/*path", davHandler("carddav"))
	}
	defaultRouter.Handle("MKCALENDAR", "/caldav/*path", davHandler("caldav"))
	defaultRouter.Handle("ACL", "/caldav/*path", davHandler("caldav"))
	defaultRouter.POST("/caldav/*path", davHandler("caldav"))

	// Well-known URIs for service discovery (RFC 6764)
	// Allows clients to auto-discover CalDAV/CardDAV endpoints
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions, "PROPFIND"} {
		defaultRouter.Handle(method, "/.well-known/caldav", func(c *gin.Context) {
			c.Redirect(http.StatusMovedPermanently, "/caldav/")
		})
		defaultRouter.Handle(method, "/.well-known/carddav", func(c *gin.Context) {
			c.Redirect(http.StatusMovedPermanently, "/carddav/")
		})
	}

	logrus.Printf("[CALDAV ENDPOINT] CalDAV/CardDAV routes registered")
	logrus.Tracef("CalDAV/CardDAV resources initialized")
}
