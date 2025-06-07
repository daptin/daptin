package server

import (
	"fmt"
	"github.com/daptin/daptin/server/resource"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func SetupNoRouteRouter(boxRoot http.FileSystem, defaultRouter *gin.Engine) {

	indexFile, err := boxRoot.Open("index.html")

	resource.CheckErr(err, "Failed to open index.html file from dashboard directory %v")

	var indexFileContents = []byte("")
	if indexFile != nil && err == nil {
		indexFileContents, err = io.ReadAll(indexFile)
	}
	defaultRouter.GET("", func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=60") // Short cache time for index.html
		c.Data(http.StatusOK, "text/html; charset=UTF-8", indexFileContents)
	})

	// Add cache middleware
	defaultRouter.Use(func(c *gin.Context) {
		// Skip non-GET requests
		if c.Request.Method != http.MethodGet {
			c.Next()
			return
		}

		c.Next()
	})

	defaultRouter.NoRoute(func(c *gin.Context) {
		filePath := strings.TrimLeft(c.Request.URL.Path, "/")

		// Check if we have the file in our cache first
		if cached, found := diskFileCache.Get(filePath); found {
			cachedFile := cached.(*DiskFileCache)

			// Handle conditional requests
			ifModifiedSince := c.GetHeader("If-Modified-Since")
			ifNoneMatch := c.GetHeader("If-None-Match")

			// Check ETag first
			if ifNoneMatch != "" && ifNoneMatch == cachedFile.ETag {
				c.Status(http.StatusNotModified)
				return
			}

			// Then check Last-Modified
			if ifModifiedSince != "" {
				ifModifiedSinceTime, err := http.ParseTime(ifModifiedSince)
				if err == nil && !cachedFile.LastModified.After(ifModifiedSinceTime) {
					c.Status(http.StatusNotModified)
					return
				}
			}

			// Set cache headers
			SetClientCacheHeaders(c, cachedFile)

			// Serve from cache
			c.Data(http.StatusOK, cachedFile.ContentType, cachedFile.Data)
			return
		}

		// File not in cache, try to open it
		file, err := boxRoot.Open(filePath)
		if err == nil && file != nil {
			defer file.Close()

			// For file system, get stats to determine last modified time
			stat, statErr := file.(interface{ Stat() (os.FileInfo, error) }).Stat()
			if statErr != nil {
				logrus.Printf("Error getting file stats: %v", statErr)
				c.FileFromFS(filePath, boxRoot)
				return
			}

			// Don't cache large files
			if stat.Size() > maxFileSizeToCache {
				// Still set client caching headers even if we don't cache it server-side
				setClientCacheHeadersForFile(c, stat.ModTime(), generateETagWithData(filePath, stat.ModTime(), stat.Size()))
				c.FileFromFS(filePath, boxRoot)
				return
			}

			// Read the file content
			content := make([]byte, stat.Size())
			_, readErr := file.Read(content)
			if readErr != nil {
				logrus.Printf("[101] Error reading file [%v]: %v", filePath, readErr)
				c.FileFromFS(filePath, boxRoot)
				return
			}

			// Determine content type
			contentType := getContentType(filePath)
			lastModified := stat.ModTime()
			etag := generateETagWithData(filePath, lastModified, stat.Size())

			// Create cache entry
			cacheEntry := &DiskFileCache{
				Data:         content,
				ContentType:  contentType,
				LastModified: lastModified,
				ETag:         etag,
			}

			// Add to cache
			diskFileCache.Add(filePath, cacheEntry)

			// Set client cache headers
			SetClientCacheHeaders(c, cacheEntry)

			// Serve the file
			c.Data(http.StatusOK, contentType, content)
			return
		}

		// Fallback to serving index.html
		if len(indexFileContents) > 0 {
			// Set minimal caching for index.html
			c.Header("Cache-Control", "public, max-age=60") // Short cache time for index.html
			c.Data(http.StatusOK, "text/html; charset=UTF-8", indexFileContents)
		}
	})
}

// DiskFileCache represents a cached file entry
type DiskFileCache struct {
	Data         []byte
	ContentType  string
	LastModified time.Time
	ETag         string
}

// Set HTTP cache headers based on the cached file
func SetClientCacheHeaders(c *gin.Context, cachedFile *DiskFileCache) {
	// Set ETag
	c.Header("ETag", cachedFile.ETag)

	// Set Last-Modified
	c.Header("Last-Modified", cachedFile.LastModified.UTC().Format(http.TimeFormat))

	// Set Cache-Control with aggressive but sane settings
	c.Header("Cache-Control", fmt.Sprintf(
		"public, max-age=%d, stale-while-revalidate=%d, stale-if-error=%d",
		cacheMaxAge, cacheStaleRevalidate, cacheStaleIfError))

	// Add Expires header as a fallback for older clients
	expiresTime := time.Now().Add(time.Duration(cacheMaxAge) * time.Second)
	c.Header("Expires", expiresTime.UTC().Format(http.TimeFormat))

	// Set Content-Type
	c.Header("Content-Type", cachedFile.ContentType)
}

// Set HTTP cache headers for a file that isn't cached server-side
func setClientCacheHeadersForFile(c *gin.Context, lastModified time.Time, etag string) {
	c.Header("ETag", etag)
	c.Header("Last-Modified", lastModified.UTC().Format(http.TimeFormat))
	c.Header("Cache-Control", fmt.Sprintf(
		"public, max-age=%d, stale-while-revalidate=%d, stale-if-error=%d",
		cacheMaxAge, cacheStaleRevalidate, cacheStaleIfError))
	expiresTime := time.Now().Add(time.Duration(cacheMaxAge) * time.Second)
	c.Header("Expires", expiresTime.UTC().Format(http.TimeFormat))
}

// Generate ETag based on file path, modification time, and size
func generateETagWithData(path string, modTime time.Time, size int64) string {
	etag := fmt.Sprintf("\"%x-%x-%x\"", size, modTime.UnixNano(), hash(path))
	return etag
}

// Simple hash function for ETag generation
func hash(s string) uint32 {
	h := uint32(0)
	for i := 0; i < len(s); i++ {
		h = h*31 + uint32(s[i])
	}
	return h
}

// Get content type based on file extension
func getContentType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".html", ".htm":
		return "text/html; charset=UTF-8"
	case ".css":
		return "text/css; charset=UTF-8"
	case ".js":
		return "application/javascript; charset=UTF-8"
	case ".json":
		return "application/json; charset=UTF-8"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".webp":
		return "image/webp"
	case ".ico":
		return "image/x-icon"
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain; charset=UTF-8"
	case ".xml":
		return "application/xml; charset=UTF-8"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	default:
		return "application/octet-stream"
	}
}
