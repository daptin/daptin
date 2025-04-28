package server

import (
	"compress/gzip"
	"crypto/md5"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/daptin/daptin/server/resource"

	"github.com/gin-gonic/gin"
	"github.com/golang/groupcache/lru"
)

const (
	// Cache settings
	MaxCacheSize     = 500        // Increased from 200
	MaxFileCacheSize = 8000 << 10 // 8MB max file size for caching (increased from 4MB)

	// Compression threshold - only compress files larger than this
	CompressionThreshold = 5 << 10 // 5KB (reduced from 10KB)
)

// FileCache implements a simple file caching system
type FileCache struct {
	cache      *lru.Cache
	cacheMutex sync.RWMutex
}

// CachedFile represents a cached file with its metadata
type CachedFile struct {
	Data       []byte
	ETag       string
	Modtime    time.Time
	MimeType   string
	Path       string
	Size       int
	GzipData   []byte // Pre-compressed version for text files
	IsDownload bool   // Whether file should be downloaded or displayed inline
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
func generateETag(content []byte, modTime time.Time) string {
	hash := md5.New()
	hash.Write(content)
	// Include modTime in the hash to ensure we catch file changes
	timeBytes := []byte(modTime.UTC().Format(time.RFC3339Nano))
	hash.Write(timeBytes)
	return fmt.Sprintf("\"%x\"", hash.Sum(nil))
}

// GetMimeType determines the MIME type based on file extension with additional types
func GetMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))

	switch ext {
	case ".png":
		return "image/png"
	case ".jpeg", ".jpg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain; charset=utf-8"
	case ".xml":
		return "application/xml; charset=utf-8"
	case ".md":
		return "text/markdown; charset=utf-8"
	case ".mp4":
		return "video/mp4"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".zip":
		return "application/zip"
	case ".doc", ".docx":
		return "application/msword"
	case ".xls", ".xlsx":
		return "application/vnd.ms-excel"
	case ".ppt", ".pptx":
		return "application/vnd.ms-powerpoint"
	default:
		return "application/octet-stream"
	}
}

// ShouldBeDownloaded determines if the file should be served as a download
func ShouldBeDownloaded(mimeType string, filename string) bool {
	// Common download types
	if strings.HasPrefix(mimeType, "application/") &&
		mimeType != "application/javascript" &&
		mimeType != "application/json" &&
		mimeType != "application/xml" {
		return true
	}

	// Add specific extensions that should always be downloads
	downloadExtensions := []string{".zip", ".rar", ".7z", ".tar", ".gz", ".exe", ".msi", ".dmg", ".apk"}
	ext := strings.ToLower(filepath.Ext(filename))
	for _, downloadExt := range downloadExtensions {
		if ext == downloadExt {
			return true
		}
	}

	return false
}

// Global file cache
var fileCache = NewFileCache(MaxCacheSize)

// compressData compresses data using gzip
func compressData(data []byte) ([]byte, error) {
	var b strings.Builder
	gz := gzip.NewWriter(&b)
	if _, err := gz.Write(data); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

// CreateDbAssetHandler optimized for static file serving with aggressive caching
func CreateDbAssetHandler(cruds map[string]*resource.DbResource) func(*gin.Context) {
	return AssetRouteHandler(cruds)
}
