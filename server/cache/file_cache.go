package cache

import (
	"compress/gzip"
	"context"
	"crypto/md5"
	"fmt"
	"github.com/buraksezer/olric"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// Cache settings
	MaxFileCacheSize = 2000 << 10 // 8MB max file size for caching

	// Compression threshold - only compress files larger than this
	CompressionThreshold = 5 << 10 // 5KB

	// Default cache expiration times
	DefaultCacheExpiry = 24 * time.Hour      // Default expiry time for cached files
	ImageCacheExpiry   = 7 * 24 * time.Hour  // 7 days for images
	TextCacheExpiry    = 24 * time.Hour      // 1 day for text files
	VideoCacheExpiry   = 14 * 24 * time.Hour // 14 days for videos

	// Olric cache namespace
	AssetsCacheNamespace = "assets-cache"
)

// FileCache implements a file caching system using Olric distributed cache
type FileCache struct {
	cache      olric.DMap
	cacheMutex sync.RWMutex
	closed     bool
	closeMutex sync.RWMutex
}

// Get retrieves a file from cache if it exists and is valid
func (fc *FileCache) Get(key string) (*CachedFile, bool) {
	// First check if cache is closed
	fc.closeMutex.RLock()
	if fc.closed {
		fc.closeMutex.RUnlock()
		return nil, false
	}
	fc.closeMutex.RUnlock()

	// Read lock for cache access
	fc.cacheMutex.RLock()
	defer fc.cacheMutex.RUnlock()

	// Get from Olric cache
	response, err := fc.cache.Get(context.Background(), key)
	if err != nil {
		if err != olric.ErrKeyNotFound {
			log.Printf("Error getting key %s from Olric cache: %v", key, err)
		}
		return nil, false
	}

	// Extract value from response using Scan
	var cachedFile CachedFile
	err = response.Scan(&cachedFile)
	if err != nil {
		log.Printf("Error scanning cached file from Olric: %v", err)
		return nil, false
	}

	// Check if expired (this should not happen with Olric's TTL, but just in case)
	if time.Now().After(cachedFile.ExpiresAt) {
		// Remove expired entry asynchronously
		go fc.Remove(key)
		return nil, false
	}

	return &cachedFile, true
}

// Set adds a file to the cache with appropriate expiry
func (fc *FileCache) Set(key string, file *CachedFile) {
	// First check if cache is closed
	fc.closeMutex.RLock()
	if fc.closed {
		fc.closeMutex.RUnlock()
		return
	}
	fc.closeMutex.RUnlock()

	// Check file size - don't cache very large files
	if len(file.Data) > MaxFileCacheSize {
		return
	}

	// Write lock for cache access
	fc.cacheMutex.Lock()
	defer fc.cacheMutex.Unlock()

	// Set the expiry time based on file type if not already set
	if file.ExpiresAt.IsZero() {
		file.ExpiresAt = CalculateExpiry(file.MimeType, file.Path)
	}

	// Calculate TTL duration from ExpiresAt
	ttl := file.ExpiresAt.Sub(time.Now())
	if ttl <= 0 {
		// Don't cache expired files
		return
	}

	// Add to Olric cache with expiry
	err := fc.cache.Put(context.Background(), key, file, olric.EX(ttl))
	if err != nil {
		log.Printf("[117] Error setting key %s in Olric cache: %v", key, err)
	}
}

// CalculateExpiry determines expiry time based on file type
func CalculateExpiry(mimeType, path string) time.Time {
	now := time.Now()

	// Determine file type from extension
	filename := filepath.Base(path)
	ext := strings.ToLower(filepath.Ext(filename))

	// Images
	if strings.HasPrefix(mimeType, "image/") ||
		ext == ".jpg" || ext == ".jpeg" || ext == ".png" ||
		ext == ".gif" || ext == ".webp" || ext == ".svg" {
		return now.Add(ImageCacheExpiry)
	}

	// Text files
	if strings.HasPrefix(mimeType, "text/") ||
		strings.Contains(mimeType, "javascript") ||
		strings.Contains(mimeType, "json") ||
		ext == ".html" || ext == ".css" || ext == ".js" ||
		ext == ".txt" || ext == ".md" || ext == ".xml" {
		return now.Add(TextCacheExpiry)
	}

	// Video files
	if strings.HasPrefix(mimeType, "video/") ||
		ext == ".mp4" || ext == ".webm" || ext == ".ogg" {
		return now.Add(VideoCacheExpiry)
	}

	// Default for other files
	return now.Add(DefaultCacheExpiry)
}

func GenerateETag(content []byte, modTime time.Time) string {
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

// FileStat stores file information for validation
type FileStat struct {
	ModTime time.Time
	Size    int64
	Exists  bool
}

// GetFileStat gets file info for validation
func GetFileStat(path string) (FileStat, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return FileStat{Exists: false}, nil
		}
		return FileStat{}, err
	}

	return FileStat{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		Exists:  true,
	}, nil
}

// NewFileCache creates a new file cache using Olric
func NewFileCache(olricClient *olric.EmbeddedClient, namespace string) (*FileCache, error) {
	if olricClient == nil {
		return nil, fmt.Errorf("olric client is nil")
	}

	dmap, err := olricClient.NewDMap(namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to create Olric DMap for assets cache: %v", err)
	}

	fc := &FileCache{
		cache: dmap,
	}

	return fc, nil
}

// CompressData compresses data using gzip
func CompressData(data []byte) ([]byte, error) {
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

// Close properly shuts down the cache
func (fc *FileCache) Close() {
	fc.closeMutex.Lock()
	defer fc.closeMutex.Unlock()

	if fc.closed {
		return
	}

	fc.closed = true

	// No need to explicitly clear the cache as Olric manages this
	// Just mark as closed to prevent further operations
}

// Remove removes an entry from cache
func (fc *FileCache) Remove(key string) {
	// First check if cache is closed
	fc.closeMutex.RLock()
	if fc.closed {
		fc.closeMutex.RUnlock()
		return
	}
	fc.closeMutex.RUnlock()

	// Write lock for cache access
	fc.cacheMutex.Lock()
	defer fc.cacheMutex.Unlock()

	// Remove from Olric cache
	_, err := fc.cache.Delete(context.Background(), key)
	if err != nil && err != olric.ErrKeyNotFound {
		log.Printf("Error removing key %s from Olric cache: %v", key, err)
	}
}
