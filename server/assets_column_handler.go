package server

import (
	"compress/gzip"
	"crypto/md5"
	"fmt"
	"os"
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
	MaxCacheSize     = 500        // Maximum number of cached files
	MaxFileCacheSize = 8000 << 10 // 8MB max file size for caching

	// Compression threshold - only compress files larger than this
	CompressionThreshold = 5 << 10 // 5KB

	// Default cache expiration times
	DefaultCacheExpiry = 24 * time.Hour      // Default expiry time for cached files
	ImageCacheExpiry   = 7 * 24 * time.Hour  // 7 days for images
	TextCacheExpiry    = 24 * time.Hour      // 1 day for text files
	VideoCacheExpiry   = 14 * 24 * time.Hour // 14 days for videos

	// Cache cleanup interval
	CacheCleanupInterval = 1 * time.Hour
)

// FileCache implements a simple file caching system with expiry
type FileCache struct {
	cache        *lru.Cache
	cacheMutex   sync.RWMutex
	cleanupTimer *time.Timer
}

// CachedFile represents a cached file with its metadata
type CachedFile struct {
	Data       []byte
	ETag       string
	Modtime    time.Time
	MimeType   string
	Path       string
	Size       int
	GzipData   []byte    // Pre-compressed version for text files
	IsDownload bool      // Whether file should be downloaded or displayed inline
	ExpiresAt  time.Time // When this cache entry expires
	FileStat   FileStat  // File stat information for validation
}

// FileStat stores file information for validation
type FileStat struct {
	ModTime time.Time
	Size    int64
	Exists  bool
}

// NewFileCache creates a new file cache with cleanup
func NewFileCache(maxEntries int) *FileCache {
	fc := &FileCache{
		cache: lru.New(maxEntries),
	}

	// Start the cleanup timer
	fc.startCleanupTimer()

	return fc
}

// startCleanupTimer initializes and starts the cleanup timer
func (fc *FileCache) startCleanupTimer() {
	fc.cleanupTimer = time.AfterFunc(CacheCleanupInterval, func() {
		fc.cleanup()
		fc.cleanupTimer.Reset(CacheCleanupInterval)
	})
}

// cleanup removes expired entries from the cache
func (fc *FileCache) cleanup() {
	fc.cacheMutex.Lock()
	defer fc.cacheMutex.Unlock()

	// This is inefficient but the LRU cache doesn't provide a better way
	// We'll need to get all keys and check each one
	//var keysToRemove []string

	// Iterate through all entries (would be better with a method to access all keys)
	// This is a hacky workaround since lru.Cache doesn't expose its keys
	tempCache := lru.New(MaxCacheSize)

	//now := time.Now()

	for {
		if fc.cache.Len() == 0 {
			break
		}

		// Remove oldest item and check if expired
		//fc.cache.RemoveOldest()
		//if !ok {
		//	break
		//}

		//entry := value.(*CachedFile)

		// If not expired, we'll add it back
		//if entry.ExpiresAt.After(now) {
		//	tempCache.Add(key, entry)
		//} else {
		//	keysToRemove = append(keysToRemove, key.(string))
		//}
	}

	// Now restore non-expired entries
	for {
		if tempCache.Len() == 0 {
			break
		}

		tempCache.RemoveOldest()
		//if !ok {
		//	break
		//}

		//fc.cache.Add(key, value)
	}

	// Log removed entries if any
	//if len(keysToRemove) > 0 {
	// Maybe log how many items were cleaned up
	//log.Infof("Cleaned up %d expired cache entries", len(keysToRemove))
	//}
}

// Get retrieves a file from cache if it exists and is valid
func (fc *FileCache) Get(key string) (*CachedFile, bool) {
	fc.cacheMutex.RLock()
	defer fc.cacheMutex.RUnlock()

	if val, ok := fc.cache.Get(key); ok {
		cachedFile := val.(*CachedFile)

		// Check if expired
		if time.Now().After(cachedFile.ExpiresAt) {
			return nil, false
		}

		// Validate file stat if path exists
		if cachedFile.Path != "" {
			currentStat, err := getFileStat(cachedFile.Path)
			if err != nil || !isSameFile(currentStat, cachedFile.FileStat) {
				return nil, false
			}
		}

		return cachedFile, true
	}
	return nil, false
}

// Set adds a file to the cache with appropriate expiry
func (fc *FileCache) Set(key string, file *CachedFile) {
	fc.cacheMutex.Lock()
	defer fc.cacheMutex.Unlock()

	// If expiry isn't set, set a default based on file type
	if file.ExpiresAt.IsZero() {
		file.ExpiresAt = calculateExpiry(file.MimeType, file.Path)
	}

	// If path exists, get file stat for future validation
	if file.Path != "" {
		if stat, err := getFileStat(file.Path); err == nil {
			file.FileStat = stat
		}
	}

	fc.cache.Add(key, file)
}

// Remove removes an entry from cache
func (fc *FileCache) Remove(key string) {
	fc.cacheMutex.Lock()
	defer fc.cacheMutex.Unlock()

	// The LRU cache doesn't have a Remove method, so we can't directly implement this
	// We could work around this by creating a new cache and copying all except this key
	// But that's inefficient, so for now we just implement a simple version

	// For now, we'll just use a hack - set it to an expired entry
	if val, ok := fc.cache.Get(key); ok {
		cachedFile := val.(*CachedFile)
		cachedFile.ExpiresAt = time.Now().Add(-1 * time.Second)
		fc.cache.Add(key, cachedFile)
	}
}

// getFileStat gets file info for validation
func getFileStat(path string) (FileStat, error) {
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

// isSameFile checks if file has changed
func isSameFile(current, cached FileStat) bool {
	if !current.Exists || !cached.Exists {
		return current.Exists == cached.Exists
	}

	// Check if size or modification time changed
	return current.Size == cached.Size && current.ModTime.Equal(cached.ModTime)
}

// calculateExpiry determines expiry time based on file type
func calculateExpiry(mimeType, path string) time.Time {
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

// Global file cache
var fileCache = NewFileCache(MaxCacheSize)

// CreateDbAssetHandler optimized for static file serving with aggressive caching
func CreateDbAssetHandler(cruds map[string]*resource.DbResource) func(*gin.Context) {
	return AssetRouteHandler(cruds)
}
