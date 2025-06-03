package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"fmt"
	"github.com/buraksezer/olric"
	log "github.com/sirupsen/logrus"
	"os"
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

// MarshalBinary implements encoding.BinaryMarshaler interface for Olric compatibility
func (sce *SubsiteCacheEntry) MarshalBinary() ([]byte, error) {
	// Calculate the total size needed for the buffer
	bufSize := 4 + // Size for ETag length
		len(sce.ETag) + // Size for ETag
		8 + // Size for Content length
		len(sce.Content) + // Size for Content
		8 + // Size for CompressedContent length
		len(sce.CompressedContent) + // Size for CompressedContent
		4 + // Size for ContentType length
		len(sce.ContentType) + // Size for ContentType
		8 + // Size for LastModified (Unix timestamp)
		4 + // Size for FilePath length
		len(sce.FilePath) + // Size for FilePath
		8 // Size for ExpiresAt (Unix timestamp)

	// Create a buffer with the calculated size
	buf := bytes.NewBuffer(make([]byte, 0, bufSize))

	// Write ETag length and ETag
	binary.Write(buf, binary.LittleEndian, int32(len(sce.ETag)))
	buf.WriteString(sce.ETag)

	// Write Content length and Content
	binary.Write(buf, binary.LittleEndian, int64(len(sce.Content)))
	buf.Write(sce.Content)

	// Write CompressedContent length and CompressedContent
	binary.Write(buf, binary.LittleEndian, int64(len(sce.CompressedContent)))
	buf.Write(sce.CompressedContent)

	// Write ContentType length and ContentType
	binary.Write(buf, binary.LittleEndian, int32(len(sce.ContentType)))
	buf.WriteString(sce.ContentType)

	// Write LastModified as Unix timestamp
	binary.Write(buf, binary.LittleEndian, sce.LastModified.Unix())

	// Write FilePath length and FilePath
	binary.Write(buf, binary.LittleEndian, int32(len(sce.FilePath)))
	buf.WriteString(sce.FilePath)

	// Write ExpiresAt as Unix timestamp
	binary.Write(buf, binary.LittleEndian, sce.ExpiresAt.Unix())

	return buf.Bytes(), nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler interface for Olric compatibility
func (sce *SubsiteCacheEntry) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)

	// Read ETag length and ETag
	var etagLen int32
	if err := binary.Read(buf, binary.LittleEndian, &etagLen); err != nil {
		return fmt.Errorf("failed to read ETag length: %v", err)
	}
	etagBytes := make([]byte, etagLen)
	if _, err := buf.Read(etagBytes); err != nil {
		return fmt.Errorf("failed to read ETag: %v", err)
	}
	sce.ETag = string(etagBytes)

	// Read Content length and Content
	var contentLen int64
	if err := binary.Read(buf, binary.LittleEndian, &contentLen); err != nil {
		return fmt.Errorf("failed to read Content length: %v", err)
	}
	sce.Content = make([]byte, contentLen)
	if _, err := buf.Read(sce.Content); err != nil {
		return fmt.Errorf("failed to read Content: %v", err)
	}

	// Read CompressedContent length and CompressedContent
	var compressedContentLen int64
	if err := binary.Read(buf, binary.LittleEndian, &compressedContentLen); err != nil {
		return fmt.Errorf("failed to read CompressedContent length: %v", err)
	}
	sce.CompressedContent = make([]byte, compressedContentLen)
	if _, err := buf.Read(sce.CompressedContent); err != nil {
		return fmt.Errorf("failed to read CompressedContent: %v", err)
	}

	// Read ContentType length and ContentType
	var contentTypeLen int32
	if err := binary.Read(buf, binary.LittleEndian, &contentTypeLen); err != nil {
		return fmt.Errorf("failed to read ContentType length: %v", err)
	}
	contentTypeBytes := make([]byte, contentTypeLen)
	if _, err := buf.Read(contentTypeBytes); err != nil {
		return fmt.Errorf("failed to read ContentType: %v", err)
	}
	sce.ContentType = string(contentTypeBytes)

	// Read LastModified
	var lastModifiedUnix int64
	if err := binary.Read(buf, binary.LittleEndian, &lastModifiedUnix); err != nil {
		return fmt.Errorf("failed to read LastModified: %v", err)
	}
	sce.LastModified = time.Unix(lastModifiedUnix, 0)

	// Read FilePath length and FilePath
	var filePathLen int32
	if err := binary.Read(buf, binary.LittleEndian, &filePathLen); err != nil {
		return fmt.Errorf("failed to read FilePath length: %v", err)
	}
	filePathBytes := make([]byte, filePathLen)
	if _, err := buf.Read(filePathBytes); err != nil {
		return fmt.Errorf("failed to read FilePath: %v", err)
	}
	sce.FilePath = string(filePathBytes)

	// Read ExpiresAt
	var expiresAtUnix int64
	if err := binary.Read(buf, binary.LittleEndian, &expiresAtUnix); err != nil {
		return fmt.Errorf("failed to read ExpiresAt: %v", err)
	}
	sce.ExpiresAt = time.Unix(expiresAtUnix, 0)

	return nil
}

// CacheConfig holds configuration for the cache
var CacheConfig = struct {
	DefaultTTL    time.Duration // Default time-to-live for cache entries
	CheckInterval time.Duration // How often to check for file modifications
	MaxCacheSize  int64         // Maximum size of the cache in bytes (0 for unlimited)
	EnableCache   bool          // Toggle to enable/disable caching
	Namespace     string        // Olric cache namespace
}{
	DefaultTTL:    time.Minute * 30,  // Default to 30 minutes
	CheckInterval: time.Minute * 5,   // Check every 5 minutes
	MaxCacheSize:  100 * 1024 * 1024, // 100 MB max cache size
	EnableCache:   true,
	Namespace:     "subsite-cache", // Separate namespace from assets cache
}

// SubsiteCache is a global cache for subsite files using Olric
var SubsiteCache olric.DMap
var olricClient *olric.EmbeddedClient
var subsiteCacheInitialized bool
var subsiteCacheMutex sync.Mutex

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

// InitSubsiteCache initializes the Olric cache for subsites
func InitSubsiteCache(client *olric.EmbeddedClient) error {
	subsiteCacheMutex.Lock()
	defer subsiteCacheMutex.Unlock()

	if subsiteCacheInitialized {
		return nil
	}

	if client == nil {
		return fmt.Errorf("olric client is nil")
	}

	olricClient = client
	var err error
	SubsiteCache, err = client.NewDMap(CacheConfig.Namespace)
	if err != nil {
		return fmt.Errorf("failed to create Olric DMap for subsite cache: %v", err)
	}

	subsiteCacheInitialized = true
	return nil
}

// addToCache adds an entry to the cache with TTL
func addToCache(cacheKey string, entry *SubsiteCacheEntry) {
	if !CacheConfig.EnableCache || !subsiteCacheInitialized {
		return
	}

	// Calculate TTL duration from ExpiresAt
	ttl := entry.ExpiresAt.Sub(time.Now())
	if ttl <= 0 {
		// Use default TTL if expiry is in the past
		ttl = CacheConfig.DefaultTTL
		entry.ExpiresAt = time.Now().Add(ttl)
	}

	// Add to Olric cache with expiry
	err := SubsiteCache.Put(context.Background(), cacheKey, entry, olric.EX(ttl))
	if err != nil {
		log.Errorf("[271] Error setting key %s in Olric subsite cache: %v", cacheKey, err)
	}
}

// Note: evictCacheEntries is not needed with Olric as it handles TTL expiration

// startCacheCleanupRoutine is not needed with Olric as it handles TTL expiration

// getFromCache retrieves an entry from the cache
func getFromCache(cacheKey string) (*SubsiteCacheEntry, bool) {
	if !CacheConfig.EnableCache || !subsiteCacheInitialized {
		return nil, false
	}

	// Get from Olric cache
	response, err := SubsiteCache.Get(context.Background(), cacheKey)
	if err != nil {
		if err != olric.ErrKeyNotFound {
			log.Errorf("Error getting key %s from Olric subsite cache: %v", cacheKey, err)
		}
		return nil, false
	}

	// Extract value from response and convert to SubsiteCacheEntry
	var entry SubsiteCacheEntry
	err = response.Scan(&entry)
	if err != nil {
		log.Errorf("Error scanning cached entry from Olric: %v", err)
		return nil, false
	}

	// Check if the file has been modified on disk
	if entry.FilePath != "" && isFileModified(entry.FilePath, &entry) {
		// Remove the stale entry
		removeFromCache(cacheKey)
		return nil, false
	}

	return &entry, true
}

// removeFromCache removes an entry from the cache
func removeFromCache(cacheKey string) {
	if !CacheConfig.EnableCache || !subsiteCacheInitialized {
		return
	}

	// Remove from Olric cache
	_, err := SubsiteCache.Delete(context.Background(), cacheKey)
	if err != nil && err != olric.ErrKeyNotFound {
		log.Errorf("Error removing key %s from Olric subsite cache: %v", cacheKey, err)
	}
}
