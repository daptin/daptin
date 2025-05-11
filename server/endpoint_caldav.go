package server

import (
	"github.com/daptin/daptin/server/auth"
	"github.com/emersion/go-webdav"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"net/http"
)

func InitializeCaldavResources(authMiddleware *auth.AuthMiddleware, defaultRouter *gin.Engine) {
	logrus.Tracef("Process caldav")

	//caldavStorage, err := resource.NewCaldavStorage(cruds, certificateManager)
	caldavHandler := webdav.Handler{
		FileSystem: webdav.LocalFileSystem("./storage"),
	}
	caldavHttpHandler := func(c *gin.Context) {
		ok, abort, modifiedRequest := authMiddleware.AuthCheckMiddlewareWithHttp(c.Request, c.Writer, true)
		if !ok || abort {
			c.Header("WWW-Authenticate", "Basic realm='caldav'")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
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
}
