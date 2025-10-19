package server

import (
	"fmt"
	"github.com/daptin/daptin/server/assetcachepojo"
	"github.com/daptin/daptin/server/subsite"
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// contentTypes maps file extensions to MIME types for zero-allocation content-type lookup
var contentTypes = map[string]string{
	".js":    "application/javascript",
	".css":   "text/css",
	".html":  "text/html; charset=utf-8",
	".htm":   "text/html; charset=utf-8",
	".png":   "image/png",
	".jpg":   "image/jpeg",
	".jpeg":  "image/jpeg",
	".gif":   "image/gif",
	".svg":   "image/svg+xml",
	".ico":   "image/x-icon",
	".json":  "application/json",
	".xml":   "application/xml",
	".txt":   "text/plain",
	".pdf":   "application/pdf",
	".woff":  "font/woff",
	".woff2": "font/woff2",
	".ttf":   "font/ttf",
	".otf":   "font/otf",
}

// getContentTypeByExtension returns the MIME type for a file extension
func getContentTypeByExtension(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	if contentType, ok := contentTypes[ext]; ok {
		return contentType
	}
	return "application/octet-stream"
}

// generateETag creates a simple ETag from file info for client caching
func generateETag(fileInfo os.FileInfo) string {
	return fmt.Sprintf(`"%x-%x"`, fileInfo.Size(), fileInfo.ModTime().Unix())
}

func SubsiteRequestHandler(site subsite.SubSite, assetCache *assetcachepojo.AssetFolderCache) func(c *gin.Context) {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		var filePath string

		if site.SiteType == "hugo" {
			filePath = filepath.Join("public", path)
		} else {
			filePath = path
		}

		// Handle directory paths by appending index.html
		if strings.HasSuffix(path, "/") || path == "" {
			filePath = filepath.Join(filePath, "index.html")
		}

		// Build full file path
		fullPath := filepath.Join(assetCache.LocalSyncPath, filePath)
		
		// Get file info (no content read - zero allocation)
		fileInfo, err := os.Stat(fullPath)
		if err != nil {
			// Try index.html for files that don't exist
			if !strings.HasSuffix(filePath, "index.html") {
				indexPath := filepath.Join(filepath.Dir(filePath), "index.html")
				fullIndexPath := filepath.Join(assetCache.LocalSyncPath, indexPath)
				if indexInfo, indexErr := os.Stat(fullIndexPath); indexErr == nil {
					fullPath = fullIndexPath
					fileInfo = indexInfo
					filePath = indexPath
				} else {
					c.Status(http.StatusNotFound)
					return
				}
			} else {
				c.Status(http.StatusNotFound)
				return
			}
		}

		// Generate ETag from file info for client caching
		etag := generateETag(fileInfo)
		
		// Check if client has current version (304 optimization - zero CPU/network)
		if c.GetHeader("If-None-Match") == etag {
			c.Status(http.StatusNotModified)
			return
		}
		
		// Set content type based on file extension
		contentType := getContentTypeByExtension(filePath)
		
		// Set caching headers for aggressive client caching
		c.Header("Content-Type", contentType)
		c.Header("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))
		c.Header("Cache-Control", "public, max-age=3600") // 1 hour cache
		c.Header("ETag", etag)
		c.Header("Last-Modified", fileInfo.ModTime().UTC().Format(http.TimeFormat))
		
		// Zero-copy serve using sendfile() - no memory allocation
		c.File(fullPath)
	}
}