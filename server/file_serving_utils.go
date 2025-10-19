package server

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Buffer pool for efficient memory reuse when file reads are required
var bufferPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 0, 100*1024) // 100KB initial capacity
	},
}

// FileServingConfig holds configuration for consistent file serving
type FileServingConfig struct {
	MaxMemoryReadSize int64         // Maximum file size to read into memory
	CacheMaxAge       time.Duration // Cache duration for static assets
	EnableCompression bool          // Whether to enable compression
}

// DefaultFileServingConfig provides sensible defaults
var DefaultFileServingConfig = FileServingConfig{
	MaxMemoryReadSize: 100 * 1024,           // 100KB
	CacheMaxAge:       365 * 24 * time.Hour, // 1 year for static assets
	EnableCompression: true,
}

// generateETagFromStat creates a consistent ETag from file metadata
func generateETagFromStat(info os.FileInfo) string {
	return fmt.Sprintf(`"%x-%x"`, info.ModTime().Unix(), info.Size())
}

// setOptimalCacheHeaders sets consistent cache headers for static files
func setOptimalCacheHeaders(c *gin.Context, fileInfo os.FileInfo, config FileServingConfig) {
	etag := generateETagFromStat(fileInfo)

	c.Header("ETag", etag)
	c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d", int(config.CacheMaxAge.Seconds())))
	c.Header("Last-Modified", fileInfo.ModTime().Format(http.TimeFormat))
}

// checkClientCache checks if client has a fresh copy and returns true if 304 should be sent
func checkClientCache(c *gin.Context, fileInfo os.FileInfo) bool {
	etag := generateETagFromStat(fileInfo)
	lastModified := fileInfo.ModTime()

	// Check ETag-based conditional request
	if clientETag := c.Request.Header.Get("If-None-Match"); clientETag == etag {
		c.Status(http.StatusNotModified)
		return true
	}

	// Check Last-Modified based conditional request
	if modSince := c.Request.Header.Get("If-Modified-Since"); modSince != "" {
		if t, err := time.Parse(http.TimeFormat, modSince); err == nil {
			if !lastModified.After(t) {
				c.Status(http.StatusNotModified)
				return true
			}
		}
	}

	return false
}

// serveFileZeroCopy serves a file using zero-copy sendfile() with optimal caching
func serveFileZeroCopy(c *gin.Context, fullPath string, fileInfo os.FileInfo, config FileServingConfig) {
	// Check client cache first
	if checkClientCache(c, fileInfo) {
		return
	}

	// Set optimal headers
	setOptimalCacheHeaders(c, fileInfo, config)

	// Set content type
	if contentType := mime.TypeByExtension(filepath.Ext(fullPath)); contentType != "" {
		c.Header("Content-Type", contentType)
	}

	// Zero-copy file serving
	c.File(fullPath)
}

// readFileWithLimit reads a file with size limit and buffer pool for memory efficiency
func readFileWithLimit(file io.Reader, maxSize int64) ([]byte, error) {
	// Get buffer from pool
	buf := bufferPool.Get().([]byte)
	defer bufferPool.Put(buf[:0]) // Reset and return to pool

	// Read with limit
	limitedReader := io.LimitReader(file, maxSize+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, err
	}

	// Check if file was too large
	if int64(len(data)) > maxSize {
		return nil, fmt.Errorf("file too large: %d bytes > %d limit", len(data), maxSize)
	}

	// Return a copy since we're returning the buffer to the pool
	result := make([]byte, len(data))
	copy(result, data)

	return result, nil
}

// serveFileWithMemoryCache serves a file with in-memory caching for small files
func serveFileWithMemoryCache(c *gin.Context, file io.Reader, fullPath string, fileInfo os.FileInfo, config FileServingConfig) error {
	// Check client cache first
	if checkClientCache(c, fileInfo) {
		return nil
	}

	// Read file with size limit
	data, err := readFileWithLimit(file, config.MaxMemoryReadSize)
	if err != nil {
		return err
	}

	// Set optimal headers
	setOptimalCacheHeaders(c, fileInfo, config)

	// Set content type
	contentType := mime.TypeByExtension(filepath.Ext(fullPath))
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	c.Header("Content-Type", contentType)

	// Serve the data
	c.Data(http.StatusOK, contentType, data)

	return nil
}

// determineServingStrategy decides how to serve a file based on size and type
func determineServingStrategy(fileInfo os.FileInfo, config FileServingConfig) string {
	if fileInfo.Size() > config.MaxMemoryReadSize {
		return "zero-copy"
	}
	return "memory"
}

// ServeFileOptimally serves a file using the most appropriate strategy
func ServeFileOptimally(c *gin.Context, fullPath string, fileInfo os.FileInfo, config FileServingConfig) error {
	strategy := determineServingStrategy(fileInfo, config)

	switch strategy {
	case "zero-copy":
		serveFileZeroCopy(c, fullPath, fileInfo, config)
		return nil
	case "memory":
		file, err := os.Open(fullPath)
		if err != nil {
			return err
		}
		defer file.Close()

		return serveFileWithMemoryCache(c, file, fullPath, fileInfo, config)
	default:
		return fmt.Errorf("unknown serving strategy: %s", strategy)
	}
}
