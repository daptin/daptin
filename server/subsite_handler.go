package server

import (
	"github.com/daptin/daptin/server/assetcachepojo"
	"github.com/daptin/daptin/server/cache"
	"github.com/daptin/daptin/server/subsite"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"io"
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

		// Handle directory paths by appending index.html
		file, err := assetCache.GetFileByName(filePath)
		fileInfo, err := file.Stat()
		if err == nil && fileInfo.IsDir() {
			filePath = filepath.Join(filePath, "index.html")
			file, err = assetCache.GetFileByName(filePath)
		}
		defer file.Close()

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

			// Check if we need to decompress or serve as-is
			clientAcceptsGzip := strings.Contains(c.Request.Header.Get("Accept-Encoding"), "gzip")

			if entry.IsCompressed {
				if clientAcceptsGzip {
					// Client accepts gzip and we have compressed content
					c.Writer.Header().Set("Content-Encoding", "gzip")
					c.Writer.Header().Set("Vary", "Accept-Encoding")
					c.Writer.WriteHeader(http.StatusOK)
					c.Writer.Write(entry.Content)
				} else {
					// Need to decompress for client
					decompressed, err := decompressContent(entry.Content)
					if err != nil {
						log.Errorf("Failed to decompress content: %v", err)
						c.Writer.WriteHeader(http.StatusInternalServerError)
						return
					}
					c.Writer.WriteHeader(http.StatusOK)
					c.Writer.Write(decompressed)
				}
			} else {
				// Content is not compressed, serve as-is
				c.Writer.WriteHeader(http.StatusOK)
				c.Writer.Write(entry.Content)
			}
			c.Abort()
			return
		}

		// If not in cache or expired, try to read the file
		content, err := io.ReadAll(file)
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
			etag := cache.GenerateETag(content, fileInfo.ModTime())
			lastModified := time.Now()

			// Decide whether to store compressed or uncompressed based on content type and size
			clientAcceptsGzip := strings.Contains(c.Request.Header.Get("Accept-Encoding"), "gzip")
			var cacheContent []byte
			var isCompressed bool

			if shouldCompress(content, contentType) && clientAcceptsGzip {
				// Compress and store compressed version
				compressed, err := compressContent(content)
				if err == nil && len(compressed) < len(content) {
					// Only use compressed if it's actually smaller
					cacheContent = compressed
					isCompressed = true
				} else {
					cacheContent = content
					isCompressed = false
				}
			} else {
				// Store uncompressed
				cacheContent = content
				isCompressed = false
			}

			// Cache the file with expiration
			cacheEntry := &SubsiteCacheEntry{
				ETag:         etag,
				Content:      cacheContent,
				IsCompressed: isCompressed,
				ContentType:  contentType,
				LastModified: lastModified,
				FilePath:     filePath,
				ExpiresAt:    time.Now().Add(CacheConfig.DefaultTTL),
			}

			// Add to cache with size management
			err = addToCache(cacheKey, cacheEntry)
			if err != nil {
				log.Debugf("Failed to add to cache: %v", err)
			}

			// Set cache headers
			c.Writer.Header().Set("ETag", etag)
			c.Writer.Header().Set("Cache-Control", "public, max-age=3600") // 1 hour
			c.Writer.Header().Set("Last-Modified", lastModified.Format(http.TimeFormat))
			c.Writer.Header().Set("Content-Type", contentType)
			c.Writer.Header().Set("Vary", "Accept-Encoding")

			// Serve the content based on what we cached
			if isCompressed && clientAcceptsGzip {
				c.Writer.Header().Set("Content-Encoding", "gzip")
				c.Writer.WriteHeader(http.StatusOK)
				c.Writer.Write(cacheContent)
			} else if isCompressed && !clientAcceptsGzip {
				// We compressed but client doesn't accept gzip, serve original
				c.Writer.WriteHeader(http.StatusOK)
				c.Writer.Write(content)
			} else {
				c.Writer.WriteHeader(http.StatusOK)
				c.Writer.Write(content)
			}
			return
		}

		// Fallback to standard file serving if reading fails
		// Try to read and cache index.html with compression
		indexPath := filepath.Join(assetCache.LocalSyncPath, "index.html")
		indexContent, err := os.ReadFile(indexPath)
		fileinfo, err := os.Stat(indexPath)
		if err == nil {
			// Generate ETag
			indexEtag := cache.GenerateETag(indexContent, fileinfo.ModTime())
			indexLastModified := time.Now()

			// Decide whether to cache compressed or uncompressed
			clientAcceptsGzip := strings.Contains(c.Request.Header.Get("Accept-Encoding"), "gzip")
			var indexCacheContent []byte
			var indexIsCompressed bool

			if shouldCompress(indexContent, "text/html") && clientAcceptsGzip {
				compressedIndex, err := compressContent(indexContent)
				if err == nil && len(compressedIndex) < len(indexContent) {
					indexCacheContent = compressedIndex
					indexIsCompressed = true
				} else {
					indexCacheContent = indexContent
					indexIsCompressed = false
				}
			} else {
				indexCacheContent = indexContent
				indexIsCompressed = false
			}

			// Cache the index.html with expiration
			indexCacheKey := getSubsiteCacheKey(c.Request.Host, "/index.html")
			indexCacheEntry := &SubsiteCacheEntry{
				ETag:         indexEtag,
				Content:      indexCacheContent,
				IsCompressed: indexIsCompressed,
				ContentType:  "text/html; charset=utf-8",
				LastModified: indexLastModified,
				FilePath:     indexPath,
				ExpiresAt:    time.Now().Add(CacheConfig.DefaultTTL),
			}

			// Add to cache with size management
			err = addToCache(indexCacheKey, indexCacheEntry)
			if err != nil {
				log.Debugf("Failed to add index.html to cache: %v", err)
			}

			// Set cache headers
			c.Writer.Header().Set("ETag", indexEtag)
			c.Writer.Header().Set("Cache-Control", "public, max-age=3600") // 1 hour
			c.Writer.Header().Set("Last-Modified", indexLastModified.Format(http.TimeFormat))
			c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			c.Writer.Header().Set("Vary", "Accept-Encoding")

			// Serve content based on what we cached
			if indexIsCompressed && clientAcceptsGzip {
				c.Writer.Header().Set("Content-Encoding", "gzip")
				c.Writer.WriteHeader(http.StatusOK)
				c.Writer.Write(indexCacheContent)
			} else if indexIsCompressed && !clientAcceptsGzip {
				// Serve original uncompressed
				c.Writer.WriteHeader(http.StatusOK)
				c.Writer.Write(indexContent)
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
