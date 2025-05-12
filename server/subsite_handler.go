package server

import (
	"github.com/daptin/daptin/server/subsite"
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func SubsiteRequestHandler(site subsite.SubSite, tempDirectoryPath string) func(c *gin.Context) {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		var filePath string

		if site.SiteType == "hugo" {
			filePath = filepath.Join(tempDirectoryPath, "public", path)
		} else {
			filePath = filepath.Join(tempDirectoryPath, path)
		}

		// Handle directory paths by appending index.html
		fileInfo, err := os.Stat(filePath)
		if err == nil && fileInfo.IsDir() {
			filePath = filepath.Join(filePath, "index.html")
		}

		// Generate a cache key for this request
		cacheKey := getSubsiteCacheKey(c.Request.Host, path)

		// Check if we have this file in cache and if it's still valid
		if entry, found := getFromCache(cacheKey); found {
			// Valid cache entry, check if client has a valid cached version
			clientETag := c.Request.Header.Get("If-None-Match")
			if clientETag == entry.ETag {
				c.Writer.WriteHeader(http.StatusNotModified)
				return
			}

			// Set cache headers
			c.Writer.Header().Set("ETag", entry.ETag)
			c.Writer.Header().Set("Cache-Control", "public, max-age=3600") // 1 hour
			c.Writer.Header().Set("Last-Modified", entry.LastModified.Format(http.TimeFormat))
			c.Writer.Header().Set("Content-Type", entry.ContentType)

			// Check if client accepts gzip encoding and we have compressed content
			if entry.CompressedContent != nil && len(entry.CompressedContent) > 0 &&
				strings.Contains(c.Request.Header.Get("Accept-Encoding"), "gzip") {
				c.Writer.Header().Set("Content-Encoding", "gzip")
				c.Writer.Header().Set("Vary", "Accept-Encoding")
				c.Writer.WriteHeader(http.StatusOK)
				c.Writer.Write(entry.CompressedContent)
			} else {
				c.Writer.WriteHeader(http.StatusOK)
				c.Writer.Write(entry.Content)
			}
			c.Abort()
			return
		}

		// If not in cache or expired, try to read the file
		content, err := os.ReadFile(filePath)
		fileInfo1, _ := os.Stat(filePath)
		if err == nil {
			// Determine content type
			contentType := http.DetectContentType(content)
			if strings.HasSuffix(filePath, ".css") {
				contentType = "text/css"
			} else if strings.HasSuffix(filePath, ".js") {
				contentType = "application/javascript"
			} else if strings.HasSuffix(filePath, ".html") {
				contentType = "text/html; charset=utf-8"
			}

			// Generate ETag
			etag := generateETag(content, fileInfo1.ModTime())
			lastModified := time.Now()

			// Compress content if it's a compressible type
			var compressedContent []byte
			if ShouldCompress(contentType) {
				compressed, err := compressContent(content)
				if err == nil {
					compressedContent = compressed
				}
			}

			// Cache the file with expiration
			cacheEntry := &SubsiteCacheEntry{
				ETag:              etag,
				Content:           content,
				CompressedContent: compressedContent,
				ContentType:       contentType,
				LastModified:      lastModified,
				FilePath:          filePath,
				ExpiresAt:         time.Now().Add(CacheConfig.DefaultTTL),
			}

			// Add to cache with size management
			addToCache(cacheKey, cacheEntry)

			// Set cache headers
			c.Writer.Header().Set("ETag", etag)
			c.Writer.Header().Set("Cache-Control", "public, max-age=3600") // 1 hour
			c.Writer.Header().Set("Last-Modified", lastModified.Format(http.TimeFormat))
			c.Writer.Header().Set("Content-Type", contentType)
			c.Writer.Header().Set("Vary", "Accept-Encoding")

			// Check if client accepts gzip encoding and we have compressed content
			if compressedContent != nil && len(compressedContent) > 0 &&
				strings.Contains(c.Request.Header.Get("Accept-Encoding"), "gzip") {
				c.Writer.Header().Set("Content-Encoding", "gzip")
				c.Writer.WriteHeader(http.StatusOK)
				c.Writer.Write(compressedContent)
			} else {
				c.Writer.WriteHeader(http.StatusOK)
				c.Writer.Write(content)
			}
			return
		}

		// Fallback to standard file serving if reading fails
		// Try to read and cache index.html with compression
		indexPath := filepath.Join(tempDirectoryPath, "index.html")
		indexContent, err := os.ReadFile(indexPath)
		fileinfo, err := os.Stat(indexPath)
		if err == nil {
			// Generate ETag
			indexEtag := generateETag(indexContent, fileinfo.ModTime())
			indexLastModified := time.Now()

			// Compress the index.html content
			var compressedIndexContent []byte
			compressedIndex, err := compressContent(indexContent)
			if err == nil {
				compressedIndexContent = compressedIndex
			}

			// Cache the index.html with expiration
			indexCacheKey := getSubsiteCacheKey(c.Request.Host, "/index.html")
			indexCacheEntry := &SubsiteCacheEntry{
				ETag:              indexEtag,
				Content:           indexContent,
				CompressedContent: compressedIndexContent,
				ContentType:       "text/html; charset=utf-8",
				LastModified:      indexLastModified,
				FilePath:          indexPath,
				ExpiresAt:         time.Now().Add(CacheConfig.DefaultTTL),
			}

			// Add to cache with size management
			addToCache(indexCacheKey, indexCacheEntry)

			// Set cache headers
			c.Writer.Header().Set("ETag", indexEtag)
			c.Writer.Header().Set("Cache-Control", "public, max-age=3600") // 1 hour
			c.Writer.Header().Set("Last-Modified", indexLastModified.Format(http.TimeFormat))
			c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			c.Writer.Header().Set("Vary", "Accept-Encoding")

			// Serve compressed content if client accepts it
			if compressedIndexContent != nil && strings.Contains(c.Request.Header.Get("Accept-Encoding"), "gzip") {
				c.Writer.Header().Set("Content-Encoding", "gzip")
				c.Writer.WriteHeader(http.StatusOK)
				c.Writer.Write(compressedIndexContent)
			} else {
				c.Writer.WriteHeader(http.StatusOK)
				c.Writer.Write(indexContent)
			}
			return
		}

		// If reading fails, fallback to standard file serving
		c.File(indexPath)
	}
}
