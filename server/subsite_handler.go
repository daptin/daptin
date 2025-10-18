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

		// Check for pre-compressed .gz version first
		gzFilePath := filePath + ".gz"
		gzFile, gzErr := assetCache.GetFileByName(gzFilePath)
		if gzErr == nil {
			defer gzFile.Close()
			// Serve pre-compressed file if client accepts gzip
			if strings.Contains(c.Request.Header.Get("Accept-Encoding"), "gzip") {
				log.Debugf("Serving pre-compressed file: %s", gzFilePath)
				c.Header("Content-Encoding", "gzip")
				c.Header("Vary", "Accept-Encoding")
				c.File(filepath.Join(assetCache.LocalSyncPath, gzFilePath))
				return
			}
		}

		// Handle directory paths by appending index.html
		file, err := assetCache.GetFileByName(filePath)
		if err != nil {
			// Try index.html for directory paths
			if strings.HasSuffix(path, "/") || path == "" {
				filePath = filepath.Join(filePath, "index.html")
				file, err = assetCache.GetFileByName(filePath)
			}
			if err != nil {
				c.Status(http.StatusNotFound)
				return
			}
		}
		fileInfo, err := file.Stat()
		if err != nil {
			file.Close()
			c.Status(http.StatusInternalServerError)
			return
		}

		if fileInfo.IsDir() {
			file.Close()
			filePath = filepath.Join(filePath, "index.html")
			file, err = assetCache.GetFileByName(filePath)
			if err != nil {
				c.Status(http.StatusNotFound)
				return
			}
			fileInfo, err = file.Stat()
			if err != nil {
				file.Close()
				c.Status(http.StatusInternalServerError)
				return
			}
		}
		defer file.Close()

		// For large files (>100KB), use efficient file serving
		if fileInfo.Size() > CacheConfig.MaxEntrySize {
			TrackCacheBypassed()
			log.Debugf("Serving large file directly: %s (size: %d bytes)", filePath, fileInfo.Size())
			fullPath := filepath.Join(assetCache.LocalSyncPath, filePath)
			c.File(fullPath)
			return
		}

		// Generate a cache key for this request
		cacheKey := getSubsiteCacheKey(c.Request.Host, path)

		// Check if another goroutine is already loading this file
		loadChan := make(chan struct{})
		if existingChan, loaded := inFlightLoads.LoadOrStore(cacheKey, loadChan); loaded {
			// Another goroutine is loading this file, wait for it
			<-existingChan.(chan struct{})
			// Now check the cache again
			if entry, found := getFromCache(cacheKey); found {
				// Serve from cache after the other goroutine loaded it
				serveFromCache(c, entry)
				return
			}
			// If still not in cache, continue to load it ourselves
		}
		defer func() {
			// Signal that we're done loading
			close(loadChan)
			inFlightLoads.Delete(cacheKey)
		}()

		// Check if we have this file in cache and if it's still valid
		if entry, found := getFromCache(cacheKey); found {
			// Serve from cache
			serveFromCache(c, entry)
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

		// Fallback: try to serve index.html
		indexPath := filepath.Join(assetCache.LocalSyncPath, "index.html")

		// Check for pre-compressed index.html.gz
		gzIndexPath := indexPath + ".gz"
		if _, err := os.Stat(gzIndexPath); err == nil {
			if strings.Contains(c.Request.Header.Get("Accept-Encoding"), "gzip") {
				log.Debugf("Serving pre-compressed index.html.gz")
				c.Header("Content-Encoding", "gzip")
				c.Header("Content-Type", "text/html; charset=utf-8")
				c.Header("Vary", "Accept-Encoding")
				c.File(gzIndexPath)
				return
			}
		}

		fileinfo, err := os.Stat(indexPath)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}

		// For large index.html (>100KB), serve directly
		if fileinfo.Size() > CacheConfig.MaxEntrySize {
			TrackCacheBypassed()
			log.Debugf("Serving large index.html directly: %d bytes", fileinfo.Size())
			c.File(indexPath)
			return
		}

		// For small index.html, read and cache
		indexContent, err := os.ReadFile(indexPath)
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

		// If all else fails, try to serve index.html directly
		c.File(indexPath)
	}
}

// serveFromCache serves a cached entry to the client with proper headers
func serveFromCache(c *gin.Context, entry *SubsiteCacheEntry) {
	// Check if client has a valid cached version
	clientETag := c.Request.Header.Get("If-None-Match")
	if clientETag == entry.ETag {
		c.Writer.WriteHeader(http.StatusNotModified)
		c.Abort()
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
}
