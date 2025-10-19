package server

import (
	"fmt"
	"github.com/daptin/daptin/server/assetcachepojo"
	"github.com/daptin/daptin/server/subsite"
	"github.com/gin-gonic/gin"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func SubsiteRequestHandler(site subsite.SubSite, assetCache *assetcachepojo.AssetFolderCache) func(c *gin.Context) {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		var filePath string

		if site.SiteType == "hugo" {
			filePath = filepath.Join("public", path)
		} else {
			filePath = path
		}

		// Ensure cloud-to-disk sync by calling GetFileByName (preserve existing business logic)
		file, err := assetCache.GetFileByName(filePath)
		if err != nil {
			// Try index.html for directory paths
			if strings.HasSuffix(path, "/") || path == "" {
				filePath = filepath.Join(filePath, "index.html")
				file, err = assetCache.GetFileByName(filePath)
			}
			if err != nil {
				// Final fallback to index.html
				serveIndexFallback(c, assetCache)
				return
			}
		}
		
		// Get file info for ETag generation (don't read content)
		fileInfo, err := file.Stat()
		file.Close() // Close immediately - we don't need to read it
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}

		if fileInfo.IsDir() {
			// Handle directory requests
			filePath = filepath.Join(filePath, "index.html")
			file, err = assetCache.GetFileByName(filePath)
			if err != nil {
				serveIndexFallback(c, assetCache)
				return
			}
			fileInfo, err = file.Stat()
			file.Close()
			if err != nil {
				c.Status(http.StatusInternalServerError)
				return
			}
		}

		// Serve file with zero-copy and client-side caching
		fullPath := filepath.Join(assetCache.LocalSyncPath, filePath)
		serveStaticFile(c, fullPath, fileInfo)
	}
}

// serveStaticFile serves a static file with zero-copy and optimal client-side caching
func serveStaticFile(c *gin.Context, fullPath string, fileInfo os.FileInfo) {
	// Generate ETag from file metadata (no content reading required)
	etag := generateETagFromStat(fileInfo)
	lastModified := fileInfo.ModTime()

	// Check client cache - ETag based conditional request
	if clientETag := c.Request.Header.Get("If-None-Match"); clientETag == etag {
		c.Status(http.StatusNotModified)
		return
	}

	// Check client cache - Last-Modified based conditional request
	if modSince := c.Request.Header.Get("If-Modified-Since"); modSince != "" {
		if t, err := time.Parse(http.TimeFormat, modSince); err == nil {
			if !lastModified.After(t) {
				c.Status(http.StatusNotModified)
				return
			}
		}
	}

	// Set optimal cache headers for static assets
	c.Header("ETag", etag)
	c.Header("Cache-Control", "public, max-age=31536000") // 1 year for static assets
	c.Header("Last-Modified", lastModified.Format(http.TimeFormat))
	
	// Set content type without reading file
	if contentType := mime.TypeByExtension(filepath.Ext(fullPath)); contentType != "" {
		c.Header("Content-Type", contentType)
	}

	// Zero-copy file serving using sendfile()
	c.File(fullPath)
}

// generateETagFromStat creates an ETag from file metadata without reading content
func generateETagFromStat(info os.FileInfo) string {
	return fmt.Sprintf(`"%x-%x"`, info.ModTime().Unix(), info.Size())
}

// serveIndexFallback handles fallback to index.html
func serveIndexFallback(c *gin.Context, assetCache *assetcachepojo.AssetFolderCache) {
	indexPath := "index.html"
	
	// Ensure index.html is synced from cloud
	file, err := assetCache.GetFileByName(indexPath)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	
	fileInfo, err := file.Stat()
	file.Close()
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	// Serve index.html with zero-copy
	fullPath := filepath.Join(assetCache.LocalSyncPath, indexPath)
	serveStaticFile(c, fullPath, fileInfo)
}
