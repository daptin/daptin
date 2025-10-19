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
	"sync"
	"time"
)

// IndexCacheEntry holds cached index.html content with TTL
type IndexCacheEntry struct {
	Content   []byte
	ETag      string
	ModTime   time.Time
	ExpiresAt time.Time
}

// NegativeCacheEntry holds 404 cache information
type NegativeCacheEntry struct {
	ExpiresAt time.Time
}

// Cache management
var (
	indexCache    sync.Map // map[string]*IndexCacheEntry (keyed by host)
	negativeCache sync.Map // map[string]*NegativeCacheEntry (keyed by host:path)
)

// Cache durations
const (
	IndexCacheTTL    = 5 * time.Minute  // 5 minutes for index.html
	NegativeCacheTTL = 2 * time.Minute  // 2 minutes for 404s
)

func SubsiteRequestHandler(site subsite.SubSite, assetCache *assetcachepojo.AssetFolderCache) func(c *gin.Context) {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		host := c.Request.Host
		var filePath string

		if site.SiteType == "hugo" {
			filePath = filepath.Join("public", path)
		} else {
			filePath = path
		}

		// Check negative cache first to avoid redundant cloud requests
		negativeKey := host + ":" + filePath
		if isNegativelyCached(negativeKey) {
			// File known to be missing, serve root index.html instead of cloud request
			serveRootIndexHtml(c, host, assetCache)
			return
		}

		// Handle directory paths by appending index.html
		if strings.HasSuffix(path, "/") || path == "" {
			filePath = filepath.Join(filePath, "index.html")
		}

		// Check if this is an index.html request
		if isIndexFile(filePath) {
			serveIndexWithMemoryCache(c, host, filePath, assetCache)
			return
		}

		// Regular static asset serving with smart caching
		serveStaticAsset(c, filePath, assetCache, negativeKey)
	}
}

// isNegativelyCached checks if a file is in the negative cache
func isNegativelyCached(key string) bool {
	if entry, exists := negativeCache.Load(key); exists {
		negEntry := entry.(*NegativeCacheEntry)
		if time.Now().Before(negEntry.ExpiresAt) {
			return true
		}
		// Expired entry, remove it
		negativeCache.Delete(key)
	}
	return false
}

// addToNegativeCache adds a 404 response to the negative cache
func addToNegativeCache(key string) {
	entry := &NegativeCacheEntry{
		ExpiresAt: time.Now().Add(NegativeCacheTTL),
	}
	negativeCache.Store(key, entry)
}

// isIndexFile checks if the file path is for index.html
func isIndexFile(filePath string) bool {
	return strings.HasSuffix(filePath, "index.html") || 
		   strings.HasSuffix(filePath, "/index.html") ||
		   filePath == "index.html"
}

// serveIndexWithMemoryCache serves index.html from memory cache with 5-minute TTL
func serveIndexWithMemoryCache(c *gin.Context, host, filePath string, assetCache *assetcachepojo.AssetFolderCache) {
	// Check memory cache first
	if entry, exists := indexCache.Load(host); exists {
		cacheEntry := entry.(*IndexCacheEntry)
		if time.Now().Before(cacheEntry.ExpiresAt) {
			// Serve from memory cache
			serveIndexFromCache(c, cacheEntry)
			return
		}
		// Expired entry, remove it
		indexCache.Delete(host)
	}

	// Cache miss or expired, fetch from cloud/disk
	file, err := assetCache.GetFileByName(filePath)
	if err != nil {
		negativeKey := host + ":" + filePath
		addToNegativeCache(negativeKey)
		// Index.html not found, serve root index.html
		serveRootIndexHtml(c, host, assetCache)
		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	// Read index.html content into memory
	content, err := os.ReadFile(filepath.Join(assetCache.LocalSyncPath, filePath))
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	// Create cache entry
	etag := generateETagFromStat(fileInfo)
	cacheEntry := &IndexCacheEntry{
		Content:   content,
		ETag:      etag,
		ModTime:   fileInfo.ModTime(),
		ExpiresAt: time.Now().Add(IndexCacheTTL),
	}

	// Store in cache
	indexCache.Store(host, cacheEntry)

	// Serve from cache
	serveIndexFromCache(c, cacheEntry)
}

// serveIndexFromCache serves index.html from memory cache
func serveIndexFromCache(c *gin.Context, entry *IndexCacheEntry) {
	// Check client cache with ETag
	if clientETag := c.Request.Header.Get("If-None-Match"); clientETag == entry.ETag {
		c.Status(http.StatusNotModified)
		return
	}

	// Check client cache with Last-Modified
	if modSince := c.Request.Header.Get("If-Modified-Since"); modSince != "" {
		if t, err := time.Parse(http.TimeFormat, modSince); err == nil {
			if !entry.ModTime.After(t) {
				c.Status(http.StatusNotModified)
				return
			}
		}
	}

	// Set cache headers for index.html (5 minutes)
	c.Header("ETag", entry.ETag)
	c.Header("Cache-Control", "public, max-age=300") // 5 minutes
	c.Header("Last-Modified", entry.ModTime.Format(http.TimeFormat))
	c.Header("Content-Type", "text/html; charset=utf-8")

	// Serve from memory
	c.Data(http.StatusOK, "text/html; charset=utf-8", entry.Content)
}

// serveStaticAsset serves regular static assets with zero-copy
func serveStaticAsset(c *gin.Context, filePath string, assetCache *assetcachepojo.AssetFolderCache, negativeKey string) {
	// Try to get file (cloud sync)
	file, err := assetCache.GetFileByName(filePath)
	if err != nil {
		// Add to negative cache and serve root index.html
		addToNegativeCache(negativeKey)
		serveRootIndexHtml(c, c.Request.Host, assetCache)
		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	if fileInfo.IsDir() {
		// Directory request, try index.html
		indexPath := filepath.Join(filePath, "index.html")
		serveIndexWithMemoryCache(c, c.Request.Host, indexPath, assetCache)
		return
	}

	// Serve static file with zero-copy
	fullPath := filepath.Join(assetCache.LocalSyncPath, filePath)
	serveStaticFileOptimal(c, fullPath, fileInfo)
}

// serveStaticFileOptimal serves static files with zero-copy and long cache
func serveStaticFileOptimal(c *gin.Context, fullPath string, fileInfo os.FileInfo) {
	etag := generateETagFromStat(fileInfo)
	lastModified := fileInfo.ModTime()

	// Check client cache - ETag
	if clientETag := c.Request.Header.Get("If-None-Match"); clientETag == etag {
		c.Status(http.StatusNotModified)
		return
	}

	// Check client cache - Last-Modified
	if modSince := c.Request.Header.Get("If-Modified-Since"); modSince != "" {
		if t, err := time.Parse(http.TimeFormat, modSince); err == nil {
			if !lastModified.After(t) {
				c.Status(http.StatusNotModified)
				return
			}
		}
	}

	// Set long cache headers for static assets
	c.Header("ETag", etag)
	c.Header("Cache-Control", "public, max-age=31536000") // 1 year
	c.Header("Last-Modified", lastModified.Format(http.TimeFormat))
	
	// Set content type
	if contentType := mime.TypeByExtension(filepath.Ext(fullPath)); contentType != "" {
		c.Header("Content-Type", contentType)
	}

	// Zero-copy serving
	c.File(fullPath)
}

// generateETagFromStat creates an ETag from file metadata
func generateETagFromStat(info os.FileInfo) string {
	return fmt.Sprintf(`"%x-%x"`, info.ModTime().Unix(), info.Size())
}

// serveRootIndexHtml serves the root index.html as fallback (for SPA routing)
func serveRootIndexHtml(c *gin.Context, host string, assetCache *assetcachepojo.AssetFolderCache) {
	// Always serve root index.html for missing files (SPA compatibility)
	rootIndexPath := "index.html"
	
	// Check if root index.html exists in cache first
	if entry, exists := indexCache.Load(host); exists {
		cacheEntry := entry.(*IndexCacheEntry)
		if time.Now().Before(cacheEntry.ExpiresAt) {
			serveIndexFromCache(c, cacheEntry)
			return
		}
	}
	
	// Try to get root index.html
	file, err := assetCache.GetFileByName(rootIndexPath)
	if err != nil {
		// Even root index.html doesn't exist, return a basic HTML response
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(`<!DOCTYPE html>
<html>
<head><title>Site</title></head>
<body><h1>Welcome</h1><p>Site is loading...</p></body>
</html>`))
		return
	}
	file.Close()
	
	// Serve root index.html with caching
	serveIndexWithMemoryCache(c, host, rootIndexPath, assetCache)
}
