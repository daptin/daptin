package server

import (
	"compress/gzip"
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/resource"

	"github.com/gin-gonic/gin"
	"github.com/golang/groupcache/lru"
	log "github.com/sirupsen/logrus"
)

const (
	// Cache settings
	MaxCacheSize     = 200        // Maximum number of files to cache
	MaxFileCacheSize = 4000 << 10 // 10MB max file size for caching

	// Compression threshold - only compress files larger than this
	CompressionThreshold = 10 << 10 // 10KB
)

// FileCache implements a simple file caching system
type FileCache struct {
	cache      *lru.Cache
	cacheMutex sync.RWMutex
}

// CachedFile represents a cached file with its metadata
type CachedFile struct {
	Data     []byte
	ETag     string
	Modtime  time.Time
	MimeType string
	Path     string
	Size     int
}

// NewFileCache creates a new file cache
func NewFileCache(maxEntries int) *FileCache {
	return &FileCache{
		cache: lru.New(maxEntries),
	}
}

// Get retrieves a file from cache if it exists
func (fc *FileCache) Get(key string) (*CachedFile, bool) {
	fc.cacheMutex.RLock()
	defer fc.cacheMutex.RUnlock()

	if val, ok := fc.cache.Get(key); ok {
		return val.(*CachedFile), true
	}
	return nil, false
}

// Set adds a file to the cache
func (fc *FileCache) Set(key string, file *CachedFile) {
	fc.cacheMutex.Lock()
	defer fc.cacheMutex.Unlock()

	fc.cache.Add(key, file)
}

// Generates ETag for content
func generateETag(content []byte) string {
	hash := md5.New()
	hash.Write(content)
	return fmt.Sprintf("\"%x\"", hash.Sum(nil))
}

// GetMimeType determines the MIME type based on file extension
func GetMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))

	switch ext {
	case ".png":
		return "image/png"
	case ".jpeg", ".jpg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain; charset=utf-8"
	case ".xml":
		return "application/xml"
	default:
		return "application/octet-stream"
	}
}

// Global file cache
var fileCache = NewFileCache(MaxCacheSize)

// CreateDbAssetHandler optimized for static file serving using OptimizedFileHandler
func CreateDbAssetHandler(cruds map[string]*resource.DbResource) func(*gin.Context) {
	// Pre-allocate common response headers
	const cacheControl = "public, max-age=86400" // 1 day cache

	// Create a concurrent map to store file handlers
	fileHandlers := sync.Map{}

	// Precompile regex patterns for common file types for faster matching
	imagePattern := regexp.MustCompile(`\.(jpe?g|png|gif|webp|svg)$`)
	textPattern := regexp.MustCompile(`\.(css|js|html?|txt|md|json|xml)$`)

	return func(c *gin.Context) {
		typeName := c.Param("typename")
		resourceUuid := c.Param("resource_id")
		columnNameWithExt := c.Param("columnname")

		// Generate a cache key for this request
		cacheKey := fmt.Sprintf("%s:%s:%s", typeName, resourceUuid, columnNameWithExt)

		// Check if we have a cached handler for this request
		if cachedInfo, found := fileCache.Get(cacheKey); found {
			info := cachedInfo

			if clientEtag := c.GetHeader("If-None-Match"); clientEtag != "" && clientEtag == cachedInfo.ETag {
				c.AbortWithStatus(http.StatusNotModified)
				return
			}

			// Set content type from cache
			c.Header("Content-Type", info.MimeType)
			c.Header("Cache-Control", cacheControl)
			c.Header("ETag", info.ETag)

			// For downloads, add content disposition
			if strings.HasPrefix(info.MimeType, "application/") {
				c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%v\"", filepath.Base(info.Path)))
			}

			// Use optimized file handler
			file, err := os.Open(info.Path)
			if err == nil {
				defer file.Close()
				fileInfo, err := file.Stat()
				if err == nil {
					http.ServeContent(c.Writer, c.Request, filepath.Base(info.Path), fileInfo.ModTime(), file)
					return
				}
			}
			// If we get here, the file may have been deleted or changed
			fileHandlers.Delete(cacheKey)
		}

		parts := strings.SplitN(columnNameWithExt, ".", 2)
		if len(parts) == 0 {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		columnName := parts[0]

		// Fast path: check if the table exists
		table, ok := cruds[typeName]
		if !ok || table == nil {
			log.Errorf("table not found [%v]", typeName)
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		// Fast path: check if the column exists
		colInfo, ok := table.TableInfo().GetColumnByName(columnName)
		if !ok || colInfo == nil || (!colInfo.IsForeignKey && colInfo.ColumnType != "markdown") {
			log.Errorf("column [%v] info not found", columnName)
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		// Handle markdown directly (simple case)
		if colInfo.ColumnType == "markdown" {
			// Fetch data
			pr := &http.Request{
				Method: "GET",
				URL:    c.Request.URL,
			}
			pr = pr.WithContext(c.Request.Context())

			req := api2go.Request{
				PlainRequest: pr,
			}

			obj, err := cruds[typeName].FindOne(resourceUuid, req)
			if err != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}

			row := obj.Result().(api2go.Api2GoModel)
			colData := row.GetAttributes()[columnName]
			if colData == nil {
				c.AbortWithStatus(http.StatusNotFound)
				return
			}

			// Return markdown as HTML
			c.Header("Content-Type", "text/html")
			c.Header("Cache-Control", cacheControl)
			c.Header("ETag", generateETag([]byte(colData.(string))))

			c.String(http.StatusOK, "<pre>%s</pre>", colData.(string))
			return
		}

		// Handle foreign key (file data)
		if colInfo.IsForeignKey {
			// Get cache for this path
			assetCache, ok := cruds["world"].AssetFolderCache[typeName][columnName]
			if !ok {
				c.AbortWithStatus(http.StatusNotFound)
				return
			}

			// Find the file to serve
			pr := &http.Request{
				Method: "GET",
				URL:    c.Request.URL,
			}
			pr = pr.WithContext(c.Request.Context())

			req := api2go.Request{
				PlainRequest: pr,
			}

			obj, err := cruds[typeName].FindOne(resourceUuid, req)
			if err != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}

			row := obj.Result().(api2go.Api2GoModel)
			colData := row.GetAttributes()[columnName]
			if colData == nil {
				c.AbortWithStatus(http.StatusNotFound)
				return
			}

			// Find the correct file
			fileNameToServe := ""
			fileType := "application/octet-stream"

			for _, fileData := range colData.([]map[string]interface{}) {
				fileName := fileData["name"].(string)
				queryFile := c.Query("file")

				if queryFile == fileName || queryFile == "" {
					// Determine filename
					if fileData["path"] != nil && len(fileData["path"].(string)) > 0 {
						fileNameToServe = fileData["path"].(string) + "/" + fileName
					} else {
						fileNameToServe = fileName
					}

					// Determine mime type
					if typFromData, ok := fileData["type"]; ok {
						if typeStr, isStr := typFromData.(string); isStr {
							fileType = typeStr
						} else {
							fileType = GetMimeType(fileNameToServe)
						}
					} else {
						fileType = GetMimeType(fileNameToServe)
					}

					break
				}
			}

			if fileNameToServe == "" {
				c.AbortWithStatus(http.StatusNotFound)
				return
			}

			// Get file path
			filePath := assetCache.LocalSyncPath + string(os.PathSeparator) + fileNameToServe

			// Check if it's an image that needs processing
			if isImage := strings.HasPrefix(fileType, "image/"); isImage && c.Query("processImage") == "true" {
				// Use separate function for image processing to keep this path fast
				file, err := cruds["world"].AssetFolderCache[typeName][columnName].GetFileByName(fileNameToServe)
				if err != nil {
					_ = c.AbortWithError(500, err)
					return
				}
				defer file.Close()
				HandleImageProcessing(c, file)
				return
			}

			// Check if file exists
			fileInfo, err := os.Stat(filePath)
			if err != nil {
				c.AbortWithStatus(http.StatusNotFound)
				return
			}

			// Set response headers
			c.Header("Content-Type", fileType)
			c.Header("Cache-Control", cacheControl)

			// For downloads, add content disposition
			if strings.HasPrefix(fileType, "application/") {
				c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%v\"", fileNameToServe))
			}

			// Fast path for common file types
			filename := filepath.Base(filePath)

			// Apply different caching strategies based on file type
			var maxAge int
			if imagePattern.MatchString(strings.ToLower(filename)) {
				// Images can be cached longer
				maxAge = 604800 // 7 days
				c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d", maxAge))
			} else if textPattern.MatchString(strings.ToLower(filename)) {
				// Text files shorter cache time
				maxAge = 86400 // 1 day
				c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d", maxAge))
			}

			// Use optimized file serving for small files that can be cached
			if fileInfo.Size() <= MaxFileCacheSize {
				file, err := os.Open(filePath)
				if err != nil {
					c.AbortWithStatus(http.StatusInternalServerError)
					return
				}
				defer file.Close()

				data, err := io.ReadAll(file)
				if err != nil {
					c.AbortWithStatus(http.StatusInternalServerError)
					return
				}

				// Generate ETag for client-side caching
				etag := generateETag(data)
				// Add to cache for future requests
				newCachedFile := &CachedFile{
					Data:     data,
					ETag:     etag,
					Modtime:  fileInfo.ModTime(),
					MimeType: fileType,
					Size:     len(data),
				}
				fileCache.Set(cacheKey, newCachedFile)

				// Check if client has fresh copy
				if clientEtag := c.GetHeader("If-None-Match"); clientEtag != "" && clientEtag == etag {
					c.AbortWithStatus(http.StatusNotModified)
					return
				}

				// Add compression support for text files
				if strings.HasPrefix(fileType, "text/") ||
					strings.HasPrefix(fileType, "application/json") ||
					strings.HasPrefix(fileType, "application/javascript") ||
					strings.HasPrefix(fileType, "application/xml") {
					// Only compress if file is larger than threshold and client accepts it
					if len(data) > CompressionThreshold && strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
						c.Header("Content-Encoding", "gzip")
						c.Header("Vary", "Accept-Encoding")
						c.Header("ETag", etag)

						// Use gin's built-in gzip writer
						c.Writer.Header().Set("Content-Type", fileType)
						c.Status(http.StatusOK)
						gzipWriter := gzip.NewWriter(c.Writer)
						defer gzipWriter.Close()
						gzipWriter.Write(data)
						return
					}
				}

				// Set ETag header
				c.Header("ETag", etag)
				c.Data(http.StatusOK, fileType, data)
				return
			}

			// For larger files, use http.ServeContent for efficient range requests
			file, err := os.Open(filePath)
			if err != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			defer file.Close()

			http.ServeContent(c.Writer, c.Request, fileNameToServe, fileInfo.ModTime(), file)
		}
	}
}
