package server

import (
	"compress/gzip"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/artpar/stats"
	"github.com/daptin/daptin/server/assetcachepojo"
	"github.com/daptin/daptin/server/subsite"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

var subsiteGzipExcludedExtensions = map[string]struct{}{
	".pdf": {}, ".mp4": {}, ".jpg": {}, ".png": {},
	".wav": {}, ".gif": {}, ".mp3": {},
}

type subsiteGzipWriter struct {
	gin.ResponseWriter
	writer io.Writer
}

func (w *subsiteGzipWriter) Write(data []byte) (int, error) {
	w.Header().Del("Content-Length")
	return w.writer.Write(data)
}

func (w *subsiteGzipWriter) WriteString(data string) (int, error) {
	w.Header().Del("Content-Length")
	return io.WriteString(w.writer, data)
}

func (w *subsiteGzipWriter) WriteHeader(code int) {
	w.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(code)
}

func subsiteGzipMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		request := c.Request
		_, excludedExtension := subsiteGzipExcludedExtensions[strings.ToLower(filepath.Ext(request.URL.Path))]
		if request.Header.Get("Range") != "" ||
			!strings.Contains(request.Header.Get("Accept-Encoding"), "gzip") ||
			strings.Contains(request.Header.Get("Connection"), "Upgrade") ||
			strings.Contains(request.Header.Get("Accept"), "text/event-stream") ||
			excludedExtension {
			c.Next()
			return
		}

		writer := gzip.NewWriter(c.Writer)
		defer writer.Close()
		c.Header("Content-Encoding", "gzip")
		c.Header("Vary", "Accept-Encoding")
		c.Writer = &subsiteGzipWriter{ResponseWriter: c.Writer, writer: writer}
		c.Next()
	}
}

func CreateSubsiteEngine(site subsite.SubSite, assetCache *assetcachepojo.AssetFolderCache, middlewares []gin.HandlerFunc, gzipEnabled ...bool) *gin.Engine {
	subsiteStats := stats.New()
	hostRouter := gin.New()
	enableGzip := len(gzipEnabled) == 0 || gzipEnabled[0]

	if enableGzip {
		hostRouter.Use(subsiteGzipMiddleware())
	}

	hostRouter.Use(func() gin.HandlerFunc {
		return func(c *gin.Context) {
			beginning, recorder := subsiteStats.Begin(c.Writer)
			defer Stats.End(beginning, stats.WithRecorder(recorder))
			c.Next()
		}
	}())

	hostRouter.Use(gin.Logger())

	for _, mid := range middlewares {
		hostRouter.Use(mid)
	}

	hostRouter.GET("/stats", func(c *gin.Context) {
		c.JSON(200, subsiteStats.Data())
	})

	log.Tracef("Serve subsite[%s] from source [%s]", site.Name, assetCache.LocalSyncPath)

	// Create a custom middleware for serving static files with aggressive caching
	//hostRouter.Any("/", SubsiteRequestHandler(site, tempDirectoryPath))
	hostRouter.NoRoute(SubsiteRequestHandler(site, assetCache))

	hostRouter.Handle("GET", "/statistics", func(c *gin.Context) {
		c.JSON(http.StatusOK, Stats.Data())
	})
	return hostRouter
}
