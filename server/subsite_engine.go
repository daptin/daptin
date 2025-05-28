package server

import (
	"github.com/artpar/stats"
	"github.com/daptin/daptin/server/assetcachepojo"
	"github.com/daptin/daptin/server/subsite"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"net/http"
)

func CreateSubsiteEngine(site subsite.SubSite, assetCache *assetcachepojo.AssetFolderCache, middlewares []gin.HandlerFunc) *gin.Engine {
	subsiteStats := stats.New()
	hostRouter := gin.New()

	// We're using our own compression implementation instead of middleware

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
