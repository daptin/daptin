package server

import (
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/resource"
	"github.com/emersion/go-webdav"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"net/http"
)

// InitializeCaldavResources sets up CalDAV/CardDAV endpoints
// Pattern: like InitializeImapResources - receives cruds parameter
func InitializeCaldavResources(
	authMiddleware *auth.AuthMiddleware,
	cruds map[string]*resource.DbResource,
	certManager *resource.CertificateManager,
	defaultRouter *gin.Engine) {

	logrus.Printf("[CALDAV ENDPOINT] InitializeCaldavResources called - starting CalDAV setup")
	logrus.Tracef("Process caldav")

	// Create backend (like NewImapServer - imap_backend.go:93-97)
	logrus.Printf("[CALDAV ENDPOINT] Creating CalDAV backend...")
	caldavBackend := resource.NewCaldavBackend(cruds, certManager)
	logrus.Printf("[CALDAV ENDPOINT] CalDAV backend created successfully")

	caldavHttpHandler := func(c *gin.Context) {
		logrus.Printf("[CALDAV HANDLER] Request received: %s %s", c.Request.Method, c.Request.URL.Path)
		// Auth via middleware
		ok, abort, modifiedRequest := authMiddleware.AuthCheckMiddlewareWithHttp(c.Request, c.Writer, true)
		logrus.Printf("[CALDAV HANDLER] Auth check: ok=%v, abort=%v", ok, abort)
		if !ok || abort {
			c.Header("WWW-Authenticate", "Basic realm='caldav'")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		// Extract session user from context
		sessionUser := modifiedRequest.Context().Value("user").(*auth.SessionUser)

		// Create per-user filesystem (like IMAP creates DaptinImapUser)
		caldavFileSystem := caldavBackend.CreateFileSystemForUser(sessionUser)

		// Route to WebDAV handler
		caldavHandler := webdav.Handler{FileSystem: caldavFileSystem}
		caldavHandler.ServeHTTP(c.Writer, modifiedRequest)
	}
	defaultRouter.Handle("OPTIONS", "/caldav/*path", caldavHttpHandler)
	defaultRouter.Handle("HEAD", "/caldav/*path", caldavHttpHandler)
	defaultRouter.Handle("GET", "/caldav/*path", caldavHttpHandler)
	defaultRouter.Handle("POST", "/caldav/*path", caldavHttpHandler)
	defaultRouter.Handle("PUT", "/caldav/*path", caldavHttpHandler)
	defaultRouter.Handle("PATCH", "/caldav/*path", caldavHttpHandler)
	defaultRouter.Handle("PROPFIND", "/caldav/*path", caldavHttpHandler)
	defaultRouter.Handle("DELETE", "/caldav/*path", caldavHttpHandler)
	defaultRouter.Handle("COPY", "/caldav/*path", caldavHttpHandler)
	defaultRouter.Handle("MOVE", "/caldav/*path", caldavHttpHandler)
	defaultRouter.Handle("MKCOL", "/caldav/*path", caldavHttpHandler)
	defaultRouter.Handle("PROPPATCH", "/caldav/*path", caldavHttpHandler)

	defaultRouter.Handle("OPTIONS", "/carddav/*path", caldavHttpHandler)
	defaultRouter.Handle("HEAD", "/carddav/*path", caldavHttpHandler)
	defaultRouter.Handle("GET", "/carddav/*path", caldavHttpHandler)
	defaultRouter.Handle("POST", "/carddav/*path", caldavHttpHandler)
	defaultRouter.Handle("PUT", "/carddav/*path", caldavHttpHandler)
	defaultRouter.Handle("PATCH", "/carddav/*path", caldavHttpHandler)
	defaultRouter.Handle("PROPFIND", "/carddav/*path", caldavHttpHandler)
	defaultRouter.Handle("DELETE", "/carddav/*path", caldavHttpHandler)
	defaultRouter.Handle("COPY", "/carddav/*path", caldavHttpHandler)
	defaultRouter.Handle("MOVE", "/carddav/*path", caldavHttpHandler)
	defaultRouter.Handle("MKCOL", "/carddav/*path", caldavHttpHandler)
	defaultRouter.Handle("PROPPATCH", "/carddav/*path", caldavHttpHandler)

	// Well-known URIs for service discovery (RFC 6764)
	// Allows clients to auto-discover CalDAV/CardDAV endpoints
	defaultRouter.GET("/.well-known/caldav", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/caldav/")
	})
	defaultRouter.GET("/.well-known/carddav", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/carddav/")
	})

	logrus.Printf("[CALDAV ENDPOINT] All CalDAV/CardDAV routes registered successfully!")
	logrus.Printf("[CALDAV ENDPOINT] Routes: MKCOL, OPTIONS, GET, PUT, PROPFIND, DELETE, COPY, MOVE, PROPPATCH")
	logrus.Tracef("CalDAV/CardDAV resources initialized")
}
