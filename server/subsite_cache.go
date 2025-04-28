package server

import (
	"bytes"
	"compress/gzip"
	log "github.com/sirupsen/logrus"
	"os"
	"strings"
	"sync"
	"time"
)

// SubsiteCacheEntry represents a cached file with its etag, content, and expiration
type SubsiteCacheEntry struct {
	ETag              string
	Content           []byte
	CompressedContent []byte // Gzip compressed content
	ContentType       string
	LastModified      time.Time
	FilePath          string    // Store the actual file path for checking modifications
	ExpiresAt         time.Time // When this cache entry expires
}

// CacheConfig holds configuration for the cache
var CacheConfig = struct {
	DefaultTTL    time.Duration // Default time-to-live for cache entries
	CheckInterval time.Duration // How often to check for file modifications
	MaxCacheSize  int64         // Maximum size of the cache in bytes (0 for unlimited)
	EnableCache   bool          // Toggle to enable/disable caching
}{
	DefaultTTL:    time.Minute * 30,  // Default to 30 minutes
	CheckInterval: time.Minute * 5,   // Check every 5 minutes
	MaxCacheSize:  100 * 1024 * 1024, // 100 MB max cache size
	EnableCache:   true,
}

// SubsiteFileCache is a global in-memory cache for subsite files
var SubsiteFileCache sync.Map
var CacheSizeCount int64
var CacheSizeMutex sync.Mutex

// compressContent compresses content using gzip with best compression
func compressContent(content []byte) ([]byte, error) {
	var b bytes.Buffer
	gw, err := gzip.NewWriterLevel(&b, gzip.BestCompression)
	if err != nil {
		return nil, err
	}

	if _, err := gw.Write(content); err != nil {
		return nil, err
	}

	if err := gw.Close(); err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

// shouldCompress determines if a file should be compressed based on its content type
func shouldCompress(contentType string) bool {
	compressibleTypes := []string{
		"text/",
		"application/javascript",
		"application/json",
		"application/xml",
		"application/xhtml+xml",
		"image/svg+xml",
		"application/font-woff",
		"application/font-woff2",
		"application/vnd.ms-fontobject",
		"application/x-font-ttf",
		"font/opentype",
		"application/octet-stream",
	}

	// Don't compress already compressed formats
	alreadyCompressed := []string{
		"image/jpeg",
		"image/png",
		"image/gif",
		"image/webp",
		"audio/",
		"video/",
		"application/zip",
		"application/gzip",
		"application/x-gzip",
		"application/x-compressed",
		"application/x-zip-compressed",
	}

	// Check if content is already compressed
	for _, t := range alreadyCompressed {
		if strings.Contains(contentType, t) {
			return false
		}
	}

	// Check if content is compressible
	for _, t := range compressibleTypes {
		if strings.Contains(contentType, t) {
			return true
		}
	}

	return false
}

// getSubsiteCacheKey generates a unique cache key for a file path and host
func getSubsiteCacheKey(host, path string) string {
	return host + "::" + path
}

// isFileModified checks if the file on disk has been modified compared to cache
func isFileModified(filePath string, cacheEntry *SubsiteCacheEntry) bool {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		// If we can't stat the file, consider it modified
		return true
	}

	// Check modification time
	if fileInfo.ModTime().After(cacheEntry.LastModified) {
		return true
	}

	// For extra verification, we could also check file size
	if fileInfo.Size() != int64(len(cacheEntry.Content)) {
		return true
	}

	return false
}

// isCacheExpired checks if a cache entry is expired
func isCacheExpired(entry *SubsiteCacheEntry) bool {
	// Check if the entry has expired based on time
	if time.Now().After(entry.ExpiresAt) {
		return true
	}

	// Check if the file has been modified
	if entry.FilePath != "" && isFileModified(entry.FilePath, entry) {
		return true
	}

	return false
}

// addToCache adds an entry to the cache, managing cache size
func addToCache(cacheKey string, entry *SubsiteCacheEntry) {
	if !CacheConfig.EnableCache {
		return
	}

	// Calculate memory size of this entry
	entrySize := int64(len(entry.Content))
	if entry.CompressedContent != nil {
		entrySize += int64(len(entry.CompressedContent))
	}

	// Check if adding this would exceed the max cache size
	if CacheConfig.MaxCacheSize > 0 {
		CacheSizeMutex.Lock()
		defer CacheSizeMutex.Unlock()

		// If adding this would exceed the cache size, try to remove some entries
		if CacheSizeCount+entrySize > CacheConfig.MaxCacheSize {
			evictCacheEntries(entrySize)
		}

		// Update cache size
		SubsiteFileCache.Store(cacheKey, entry)
		CacheSizeCount += entrySize
	} else {
		// No max size, just add it
		SubsiteFileCache.Store(cacheKey, entry)
	}
}

// evictCacheEntries removes entries to make room for new ones
func evictCacheEntries(requiredSpace int64) {
	// Simple LRU-like eviction - remove oldest entries first
	// In a production system, you'd want a proper LRU implementation
	var entriesToRemove []string
	var removedSize int64

	// Find entries to remove
	SubsiteFileCache.Range(func(key, value interface{}) bool {
		entry := value.(*SubsiteCacheEntry)

		// Calculate size of this entry
		entrySize := int64(len(entry.Content))
		if entry.CompressedContent != nil {
			entrySize += int64(len(entry.CompressedContent))
		}

		entriesToRemove = append(entriesToRemove, key.(string))
		removedSize += entrySize

		// Stop iteration once we have enough space
		return removedSize < requiredSpace
	})

	// Remove the entries
	for _, key := range entriesToRemove {
		if val, ok := SubsiteFileCache.Load(key); ok {
			entry := val.(*SubsiteCacheEntry)
			entrySize := int64(len(entry.Content))
			if entry.CompressedContent != nil {
				entrySize += int64(len(entry.CompressedContent))
			}

			SubsiteFileCache.Delete(key)
			CacheSizeCount -= entrySize
		}
	}
}

// startCacheCleanupRoutine starts a background goroutine that cleans up expired cache entries
func startCacheCleanupRoutine() {
	go func() {
		for {
			time.Sleep(CacheConfig.CheckInterval)
			cleanupExpiredEntries()
		}
	}()
}

// cleanupExpiredEntries removes expired entries from the cache
func cleanupExpiredEntries() {
	var entriesToRemove []string
	var removedSize int64

	// Find expired entries
	SubsiteFileCache.Range(func(key, value interface{}) bool {
		cacheKey := key.(string)
		entry := value.(*SubsiteCacheEntry)

		if isCacheExpired(entry) {
			entriesToRemove = append(entriesToRemove, cacheKey)

			// Calculate size for bookkeeping
			entrySize := int64(len(entry.Content))
			if entry.CompressedContent != nil {
				entrySize += int64(len(entry.CompressedContent))
			}
			removedSize += entrySize
		}
		return true // continue iteration
	})

	// Remove expired entries
	if len(entriesToRemove) > 0 {
		CacheSizeMutex.Lock()
		defer CacheSizeMutex.Unlock()

		for _, key := range entriesToRemove {
			if val, ok := SubsiteFileCache.Load(key); ok {
				entry := val.(*SubsiteCacheEntry)
				entrySize := int64(len(entry.Content))
				if entry.CompressedContent != nil {
					entrySize += int64(len(entry.CompressedContent))
				}

				SubsiteFileCache.Delete(key)
				CacheSizeCount -= entrySize
			}
		}

		log.Infof("Removed %d expired entries from cache, freed %d bytes", len(entriesToRemove), removedSize)
	}
}

// invalidateSiteCache removes all cache entries for a given site
func invalidateSiteCache(hostname string) {
	var removedEntries int
	var removedSize int64

	SubsiteFileCache.Range(func(key, value interface{}) bool {
		cacheKey := key.(string)
		if strings.HasPrefix(cacheKey, hostname+"::") {
			entry := value.(*SubsiteCacheEntry)

			// Calculate size for bookkeeping
			entrySize := int64(len(entry.Content))
			if entry.CompressedContent != nil {
				entrySize += int64(len(entry.CompressedContent))
			}

			SubsiteFileCache.Delete(key)
			removedEntries++
			removedSize += entrySize
		}
		return true // continue iteration
	})

	if removedEntries > 0 {
		CacheSizeMutex.Lock()
		CacheSizeCount -= removedSize
		CacheSizeMutex.Unlock()
		log.Infof("Invalidated cache for site %s: removed %d entries, freed %d bytes",
			hostname, removedEntries, removedSize)
	}
}
