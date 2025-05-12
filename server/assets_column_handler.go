package server

import (
	"github.com/buraksezer/olric"
	"github.com/daptin/daptin/server/resource"
	"github.com/gin-gonic/gin"
	"log"
)

// Global file cache - will be initialized in CreateDbAssetHandler
var fileCache *FileCache

// ShutdownFileCache properly shuts down the global file cache
// This should be called during application shutdown
func ShutdownFileCache() {
	if fileCache != nil {
		fileCache.Close()
	}
}

// CreateDbAssetHandler optimized for static file serving with aggressive caching
func CreateDbAssetHandler(cruds map[string]*resource.DbResource, olricClient *olric.EmbeddedClient) func(*gin.Context) {
	// Initialize the global file cache with Olric
	var err error
	fileCache, err = NewFileCache(olricClient)
	if err != nil {
		log.Printf("Failed to initialize Olric file cache: %v. Using nil cache.", err)
		// Continue without cache
	}
	return AssetRouteHandler(cruds)
}
