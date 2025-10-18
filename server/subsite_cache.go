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
	"strings"
	"sync"
	"time"
)

// SubsiteCacheEntry represents a cached file with its etag, content, and expiration
type SubsiteCacheEntry struct {
	ETag         string
	Content      []byte // Stores either compressed or uncompressed content
	IsCompressed bool   // Indicates if Content is compressed
	ContentType  string
	LastModified time.Time
	FilePath     string    // Store the actual file path for checking modifications
	ExpiresAt    time.Time // When this cache entry expires
	Size         int64     // Size of the content for memory tracking
}

// MarshalBinary implements encoding.BinaryMarshaler interface for Olric compatibility
func (sce *SubsiteCacheEntry) MarshalBinary() ([]byte, error) {
	// Calculate the total size needed for the buffer
	bufSize := 4 + // Size for ETag length
		len(sce.ETag) + // Size for ETag
		8 + // Size for Content length
		len(sce.Content) + // Size for Content
		1 + // Size for IsCompressed bool
		4 + // Size for ContentType length
		len(sce.ContentType) + // Size for ContentType
		8 + // Size for LastModified (Unix timestamp)
		4 + // Size for FilePath length
		len(sce.FilePath) + // Size for FilePath
		8 + // Size for ExpiresAt (Unix timestamp)
		8 // Size for Size int64

	// Create a buffer with the calculated size
	buf := bytes.NewBuffer(make([]byte, 0, bufSize))

	// Write ETag length and ETag
	binary.Write(buf, binary.LittleEndian, int32(len(sce.ETag)))
	buf.WriteString(sce.ETag)

	// Write Content length and Content
	binary.Write(buf, binary.LittleEndian, int64(len(sce.Content)))
	buf.Write(sce.Content)

	// Write IsCompressed
	if sce.IsCompressed {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}

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

	// Write Size
	binary.Write(buf, binary.LittleEndian, sce.Size)

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

	// Read IsCompressed
	isCompressedByte, err := buf.ReadByte()
	if err != nil {
		return fmt.Errorf("failed to read IsCompressed: %v", err)
	}
	sce.IsCompressed = isCompressedByte == 1

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

	// Read Size
	if err := binary.Read(buf, binary.LittleEndian, &sce.Size); err != nil {
		return fmt.Errorf("failed to read Size: %v", err)
	}

	return nil
}

// Note: Removed MarshalBinary and UnmarshalBinary methods
// Olric handles serialization automatically for struct types

// CacheConfig holds configuration for the cache
var CacheConfig = struct {
	DefaultTTL   time.Duration // Default time-to-live for cache entries
	MaxEntrySize int64         // Maximum size of a single cache entry (100KB)
	EnableCache  bool          // Toggle to enable/disable caching
	Namespace    string        // Olric cache namespace
}{
	DefaultTTL:   time.Minute * 30, // Default to 30 minutes
	MaxEntrySize: 100 * 1024,       // 100KB max entry size - cache small to medium assets
	EnableCache:  true,
	Namespace:    "subsite-cache", // Separate namespace from assets cache
}

// SubsiteCache is a global cache for subsite files using Olric
var SubsiteCache olric.DMap
var olricClient *olric.EmbeddedClient
var subsiteCacheInitialized bool
var subsiteCacheMutex sync.Mutex

// Track in-flight cache loading to prevent race conditions
var inFlightLoads = &sync.Map{}

// Cache metrics for monitoring
var (
	cacheHits         int64
	cacheMisses       int64
	cacheBypassed     int64 // Files too large for cache
	cacheRejected     int64 // Files rejected due to size
	cacheAdded        int64 // Successfully cached files
	cacheMetricsMutex sync.RWMutex
)

// compressContent compresses content using gzip with default compression level
func compressContent(content []byte) ([]byte, error) {
	var b bytes.Buffer
	gw, err := gzip.NewWriterLevel(&b, gzip.DefaultCompression)
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

// decompressContent decompresses gzip compressed content
func decompressContent(compressed []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var b bytes.Buffer
	if _, err := b.ReadFrom(r); err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

// shouldCompress determines if content should be compressed based on type and size
func shouldCompress(content []byte, contentType string) bool {
	// Don't compress small files (less than 1KB)
	if len(content) < 1024 {
		return false
	}

	// Only compress text-based content types
	return strings.HasPrefix(contentType, "text/") ||
		strings.Contains(contentType, "javascript") ||
		strings.Contains(contentType, "json") ||
		strings.Contains(contentType, "xml") ||
		strings.Contains(contentType, "svg")
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

	// Start periodic metrics logging
	go logCacheMetricsPeriodically()

	log.Infof("Subsite cache initialized with max entry size: %d KB", CacheConfig.MaxEntrySize/1024)
	subsiteCacheInitialized = true
	return nil
}

// addToCache adds an entry to the cache with TTL
func addToCache(cacheKey string, entry *SubsiteCacheEntry) error {
	if !CacheConfig.EnableCache || !subsiteCacheInitialized {
		return fmt.Errorf("cache not enabled or initialized")
	}

	// Check if entry exceeds max size
	entrySize := int64(len(entry.Content))
	if entrySize > CacheConfig.MaxEntrySize {
		log.Infof("Cache rejected: %s (size: %d bytes > max: %d bytes)", cacheKey, entrySize, CacheConfig.MaxEntrySize)
		return fmt.Errorf("entry too large: %d bytes", entrySize)
	}

	// Calculate TTL duration from ExpiresAt
	ttl := entry.ExpiresAt.Sub(time.Now())
	if ttl <= 0 {
		// Use default TTL if expiry is in the past
		ttl = CacheConfig.DefaultTTL
		entry.ExpiresAt = time.Now().Add(ttl)
	}

	// Add to Olric cache with expiry - use NX to prevent duplicates
	err := SubsiteCache.Put(context.Background(), cacheKey, entry, olric.NX(), olric.EX(ttl))
	if err != nil {
		if err == olric.ErrKeyFound {
			// Key already exists, this is fine - another goroutine beat us to it
			log.Debugf("Cache key already exists (prevented duplicate): %s", cacheKey)
			return nil
		}
		log.Errorf("Error setting key %s in Olric subsite cache: %v", cacheKey, err)
		return err
	}

	cacheMetricsMutex.Lock()
	cacheAdded++
	cacheMetricsMutex.Unlock()
	log.Debugf("Cache added: %s (size: %d bytes, TTL: %v)", cacheKey, entrySize, ttl)
	return nil
}

// Note: Removed evictOldEntries function - Olric handles eviction via TTL

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
		// Track cache miss
		cacheMetricsMutex.Lock()
		cacheMisses++
		cacheMetricsMutex.Unlock()
		return nil, false
	}

	// Extract value from response and convert to SubsiteCacheEntry
	var entry SubsiteCacheEntry
	err = response.Scan(&entry)
	if err != nil {
		log.Errorf("Error scanning cached entry from Olric: %v", err)
		cacheMetricsMutex.Lock()
		cacheMisses++
		cacheMetricsMutex.Unlock()
		return nil, false
	}

	// Check if the file has been modified on disk
	if entry.FilePath != "" && isFileModified(entry.FilePath, &entry) {
		// Remove the stale entry
		removeFromCache(cacheKey)
		cacheMetricsMutex.Lock()
		cacheMisses++
		cacheMetricsMutex.Unlock()
		return nil, false
	}

	// Track cache hit
	cacheMetricsMutex.Lock()
	cacheHits++
	cacheMetricsMutex.Unlock()

	return &entry, true
}

// removeFromCache removes an entry from the cache
func removeFromCache(cacheKey string) {
	if !CacheConfig.EnableCache || !subsiteCacheInitialized {
		return
	}

	// Simply remove from Olric cache
	_, err := SubsiteCache.Delete(context.Background(), cacheKey)
	if err != nil && err != olric.ErrKeyNotFound {
		log.Errorf("Error removing key %s from Olric subsite cache: %v", cacheKey, err)
	}
}

// GetCacheMetrics returns current cache metrics for monitoring
func GetCacheMetrics() map[string]interface{} {
	cacheMetricsMutex.RLock()
	defer cacheMetricsMutex.RUnlock()

	hitRate := float64(0)
	totalRequests := cacheHits + cacheMisses
	if totalRequests > 0 {
		hitRate = float64(cacheHits) / float64(totalRequests)
	}

	return map[string]interface{}{
		"cache_hits":     cacheHits,
		"cache_misses":   cacheMisses,
		"cache_bypassed": cacheBypassed,
		"cache_rejected": cacheRejected,
		"cache_added":    cacheAdded,
		"hit_rate":       hitRate,
		"total_requests": totalRequests,
		"enabled":        CacheConfig.EnableCache,
		"max_entry_size": CacheConfig.MaxEntrySize,
	}
}

// TrackCacheBypassed increments the cache bypassed counter for large files
func TrackCacheBypassed() {
	cacheMetricsMutex.Lock()
	cacheBypassed++
	cacheMetricsMutex.Unlock()
}

// logCacheMetricsPeriodically logs cache performance metrics every 5 minutes
func logCacheMetricsPeriodically() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		metrics := GetCacheMetrics()
		log.WithFields(log.Fields{
			"hits":       metrics["cache_hits"],
			"misses":     metrics["cache_misses"],
			"bypassed":   metrics["cache_bypassed"],
			"rejected":   metrics["cache_rejected"],
			"added":      metrics["cache_added"],
			"hit_rate":   fmt.Sprintf("%.2f%%", metrics["hit_rate"].(float64)*100),
			"total_reqs": metrics["total_requests"],
		}).Info("Cache performance metrics")
	}
}
