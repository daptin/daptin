package server

import (
	"bytes"
	"compress/gzip"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/daptin/daptin/server/assetcachepojo"
	"github.com/daptin/daptin/server/subsite"
	"github.com/gin-gonic/gin"
)

// IndexCacheEntry holds cached index.html content with TTL
type IndexCacheEntry struct {
	Content     []byte
	GzipContent []byte
	ETag        string
	GzipETag    string
	ModTime     time.Time
	ExpiresAt   time.Time
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
	IndexCacheTTL    = 5 * time.Minute // 5 minutes for index.html
	NegativeCacheTTL = 2 * time.Minute // 2 minutes for 404s
)

func SubsiteRequestHandler(site subsite.SubSite, assetCache *assetcachepojo.AssetFolderCache, gzipEnabled ...bool) func(c *gin.Context) {
	enableGzip := len(gzipEnabled) == 0 || gzipEnabled[0]
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		host := c.Request.Host
		var filePath string

		if site.SiteType == "hugo" {
			filePath = filepath.Join("public", path)
		} else {
			filePath = path
		}

		// Strip traversal from path
		filePath = filepath.Clean(filePath)
		for strings.HasPrefix(filePath, "..") {
			filePath = strings.TrimPrefix(strings.TrimPrefix(filePath, ".."), string(filepath.Separator))
		}

		// Check negative cache first to avoid redundant cloud requests
		negativeKey := host + ":" + filePath
		if isNegativelyCached(negativeKey) {
			// File known to be missing, serve root index.html instead of cloud request
			serveRootIndexHtml(c, host, assetCache, enableGzip)
			return
		}

		// Handle directory paths by appending index.html
		if strings.HasSuffix(path, "/") || path == "" {
			filePath = filepath.Join(filePath, "index.html")
		}

		// Check if this is an index.html request
		if isIndexFile(filePath) {
			serveIndexWithMemoryCache(c, host, filePath, assetCache, enableGzip)
			return
		}

		// Regular static asset serving with smart caching
		serveStaticAsset(c, filePath, assetCache, negativeKey, enableGzip)
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
func serveIndexWithMemoryCache(c *gin.Context, host, filePath string, assetCache *assetcachepojo.AssetFolderCache, enableGzip bool) {
	// Check memory cache first
	if entry, exists := indexCache.Load(host); exists {
		cacheEntry := entry.(*IndexCacheEntry)
		if time.Now().Before(cacheEntry.ExpiresAt) {
			// Serve from memory cache
			serveIndexFromCache(c, cacheEntry, enableGzip)
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
		serveRootIndexHtml(c, host, assetCache, enableGzip)
		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	// Read index.html content into memory
	content, err := io.ReadAll(file)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	// Create cache entry
	etag := generateETagFromStat(fileInfo)
	cacheEntry := &IndexCacheEntry{
		Content:   content,
		ETag:      etag,
		GzipETag:  gzipETag(etag),
		ModTime:   fileInfo.ModTime(),
		ExpiresAt: time.Now().Add(IndexCacheTTL),
	}
	if enableGzip && len(content) >= 1024 {
		var compressed bytes.Buffer
		writer := gzip.NewWriter(&compressed)
		if _, err := writer.Write(content); err == nil {
			if err := writer.Close(); err == nil {
				cacheEntry.GzipContent = compressed.Bytes()
			}
		} else {
			_ = writer.Close()
		}
	}

	// Store in cache
	indexCache.Store(host, cacheEntry)

	// Serve from cache
	serveIndexFromCache(c, cacheEntry, enableGzip)
}

// serveIndexFromCache serves index.html from memory cache
func serveIndexFromCache(c *gin.Context, entry *IndexCacheEntry, enableGzip bool) {
	content := entry.Content
	etag := entry.ETag
	useGzip := enableGzip && len(entry.GzipContent) > 0 && c.Request.Header.Get("Range") == "" && acceptsGzip(c.Request.Header.Get("Accept-Encoding"))
	if enableGzip {
		c.Header("Vary", appendVary(c.Writer.Header().Get("Vary"), "Accept-Encoding"))
	}
	if useGzip {
		content = entry.GzipContent
		etag = entry.GzipETag
		c.Header("Content-Encoding", "gzip")
	}
	// Set cache headers for index.html (5 minutes)
	c.Header("ETag", etag)
	c.Header("Cache-Control", "public, max-age=300") // 5 minutes
	c.Header("Last-Modified", entry.ModTime.Format(http.TimeFormat))
	c.Header("Content-Type", "text/html; charset=utf-8")
	if clientHasCurrentSubsiteRepresentation(c, etag, entry.ModTime) {
		return
	}
	if c.Request.Method == http.MethodGet && c.Request.Header.Get("Range") == "" {
		c.Data(http.StatusOK, "text/html; charset=utf-8", content)
		return
	}
	http.ServeContent(c.Writer, c.Request, "index.html", entry.ModTime, bytes.NewReader(content))
}

func clientHasCurrentSubsiteRepresentation(c *gin.Context, etag string, modTime time.Time) bool {
	if clientETag := c.Request.Header.Get("If-None-Match"); clientETag == etag {
		c.Status(http.StatusNotModified)
		return true
	}
	if modSince := c.Request.Header.Get("If-Modified-Since"); modSince != "" {
		if parsed, err := time.Parse(http.TimeFormat, modSince); err == nil && !modTime.After(parsed) {
			c.Status(http.StatusNotModified)
			return true
		}
	}
	return false
}

// serveStaticAsset serves regular static assets with zero-copy
func serveStaticAsset(c *gin.Context, filePath string, assetCache *assetcachepojo.AssetFolderCache, negativeKey string, enableGzip bool) {
	// Try to get file (cloud sync)
	file, err := assetCache.GetFileByName(filePath)
	if err != nil {
		// Add to negative cache and serve root index.html
		addToNegativeCache(negativeKey)
		serveRootIndexHtml(c, c.Request.Host, assetCache, enableGzip)
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
		serveIndexWithMemoryCache(c, c.Request.Host, indexPath, assetCache, enableGzip)
		return
	}

	serveStaticFileOptimal(c, file, filePath, fileInfo, assetCache.LocalSyncPath, enableGzip)
}

// serveStaticFileOptimal serves static files with zero-copy and long cache
func serveStaticFileOptimal(c *gin.Context, file *os.File, filePath string, fileInfo os.FileInfo, cacheRoot string, enableGzip bool) {
	etag := generateETagFromStat(fileInfo)
	servedFile := file
	compressible := enableGzip && fileInfo.Size() >= 1024 && shouldGzipSubsitePath(filePath)
	if compressible {
		c.Header("Vary", appendVary(c.Writer.Header().Get("Vary"), "Accept-Encoding"))
	}
	if compressible && c.Request.Header.Get("Range") == "" && acceptsGzip(c.Request.Header.Get("Accept-Encoding")) {
		if compressed, err := openPrecompressedSubsiteFile(file, fileInfo, cacheRoot, filePath); err == nil {
			servedFile = compressed
			defer compressed.Close()
			etag = gzipETag(etag)
			c.Header("Content-Encoding", "gzip")
		}
	}

	// Set long cache headers for static assets
	c.Header("ETag", etag)
	if clientHasCurrentSubsiteRepresentation(c, etag, fileInfo.ModTime()) {
		return
	}
	c.Header("Cache-Control", "public, max-age=31536000") // 1 year
	c.Header("Last-Modified", fileInfo.ModTime().Format(http.TimeFormat))

	// Set content type
	if contentType := mime.TypeByExtension(filepath.Ext(filePath)); contentType != "" {
		c.Header("Content-Type", contentType)
	}
	http.ServeContent(c.Writer, c.Request, filepath.Base(filePath), fileInfo.ModTime(), servedFile)
}

// serveRootIndexHtml serves the root index.html as fallback (for SPA routing)
func serveRootIndexHtml(c *gin.Context, host string, assetCache *assetcachepojo.AssetFolderCache, enableGzip bool) {
	// Always serve root index.html for missing files (SPA compatibility)
	rootIndexPath := "index.html"

	// Check if root index.html exists in cache first
	if entry, exists := indexCache.Load(host); exists {
		cacheEntry := entry.(*IndexCacheEntry)
		if time.Now().Before(cacheEntry.ExpiresAt) {
			serveIndexFromCache(c, cacheEntry, enableGzip)
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
	serveIndexWithMemoryCache(c, host, rootIndexPath, assetCache, enableGzip)
}
