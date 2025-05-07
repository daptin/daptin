package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/buraksezer/olric"
	"github.com/daptin/daptin/server/resource"
	"github.com/gin-gonic/gin"
	"log"
)

const (
	// Cache settings
	MaxFileCacheSize = 8000 << 10 // 8MB max file size for caching

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

// MarshalBinary implements encoding.BinaryMarshaler interface for Olric compatibility
// Custom binary format without using gob or other encoders
func (cf *CachedFile) MarshalBinary() ([]byte, error) {
	// Calculate the total size needed for the buffer
	bufSize := 8 + // Size for Data length
		len(cf.Data) + // Size for Data
		4 + // Size for ETag length
		len(cf.ETag) + // Size for ETag
		8 + // Size for ModTime (Unix timestamp)
		4 + // Size for MimeType length
		len(cf.MimeType) + // Size for MimeType
		4 + // Size for Path length
		len(cf.Path) + // Size for Path
		4 + // Size for Size int
		8 + // Size for GzipData length
		len(cf.GzipData) + // Size for GzipData
		1 + // Size for IsDownload bool
		8 + // Size for ExpiresAt (Unix timestamp)
		8 + // Size for FileStat.ModTime (Unix timestamp)
		8 + // Size for FileStat.Size (int64)
		1 // Size for FileStat.Exists (bool)

	// Create a buffer with the calculated size
	buf := bytes.NewBuffer(make([]byte, 0, bufSize))

	// Write Data length and Data
	binary.Write(buf, binary.LittleEndian, int64(len(cf.Data)))
	buf.Write(cf.Data)

	// Write ETag length and ETag
	binary.Write(buf, binary.LittleEndian, int32(len(cf.ETag)))
	buf.WriteString(cf.ETag)

	// Write ModTime as Unix timestamp
	binary.Write(buf, binary.LittleEndian, cf.Modtime.Unix())

	// Write MimeType length and MimeType
	binary.Write(buf, binary.LittleEndian, int32(len(cf.MimeType)))
	buf.WriteString(cf.MimeType)

	// Write Path length and Path
	binary.Write(buf, binary.LittleEndian, int32(len(cf.Path)))
	buf.WriteString(cf.Path)

	// Write Size
	binary.Write(buf, binary.LittleEndian, int32(cf.Size))

	// Write GzipData length and GzipData
	binary.Write(buf, binary.LittleEndian, int64(len(cf.GzipData)))
	buf.Write(cf.GzipData)

	// Write IsDownload
	if cf.IsDownload {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}

	// Write ExpiresAt as Unix timestamp
	binary.Write(buf, binary.LittleEndian, cf.ExpiresAt.Unix())

	// Write FileStat
	binary.Write(buf, binary.LittleEndian, cf.FileStat.ModTime.Unix())
	binary.Write(buf, binary.LittleEndian, cf.FileStat.Size)
	if cf.FileStat.Exists {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}

	return buf.Bytes(), nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler interface for Olric compatibility
// Custom binary format without using gob or other encoders
func (cf *CachedFile) UnmarshalBinary(data []byte) error {
	buf := bytes.NewBuffer(data)

	// Read Data length and Data
	var dataLen int64
	if err := binary.Read(buf, binary.LittleEndian, &dataLen); err != nil {
		return fmt.Errorf("failed to read Data length: %v", err)
	}
	cf.Data = make([]byte, dataLen)
	if _, err := buf.Read(cf.Data); err != nil {
		return fmt.Errorf("failed to read Data: %v", err)
	}

	// Read ETag length and ETag
	var etagLen int32
	if err := binary.Read(buf, binary.LittleEndian, &etagLen); err != nil {
		return fmt.Errorf("failed to read ETag length: %v", err)
	}
	etagBytes := make([]byte, etagLen)
	if _, err := buf.Read(etagBytes); err != nil {
		return fmt.Errorf("failed to read ETag: %v", err)
	}
	cf.ETag = string(etagBytes)

	// Read ModTime
	var modTimeUnix int64
	if err := binary.Read(buf, binary.LittleEndian, &modTimeUnix); err != nil {
		return fmt.Errorf("failed to read ModTime: %v", err)
	}
	cf.Modtime = time.Unix(modTimeUnix, 0)

	// Read MimeType length and MimeType
	var mimeTypeLen int32
	if err := binary.Read(buf, binary.LittleEndian, &mimeTypeLen); err != nil {
		return fmt.Errorf("failed to read MimeType length: %v", err)
	}
	mimeTypeBytes := make([]byte, mimeTypeLen)
	if _, err := buf.Read(mimeTypeBytes); err != nil {
		return fmt.Errorf("failed to read MimeType: %v", err)
	}
	cf.MimeType = string(mimeTypeBytes)

	// Read Path length and Path
	var pathLen int32
	if err := binary.Read(buf, binary.LittleEndian, &pathLen); err != nil {
		return fmt.Errorf("failed to read Path length: %v", err)
	}
	pathBytes := make([]byte, pathLen)
	if _, err := buf.Read(pathBytes); err != nil {
		return fmt.Errorf("failed to read Path: %v", err)
	}
	cf.Path = string(pathBytes)

	// Read Size
	var size int32
	if err := binary.Read(buf, binary.LittleEndian, &size); err != nil {
		return fmt.Errorf("failed to read Size: %v", err)
	}
	cf.Size = int(size)

	// Read GzipData length and GzipData
	var gzipDataLen int64
	if err := binary.Read(buf, binary.LittleEndian, &gzipDataLen); err != nil {
		return fmt.Errorf("failed to read GzipData length: %v", err)
	}
	cf.GzipData = make([]byte, gzipDataLen)
	if _, err := buf.Read(cf.GzipData); err != nil {
		return fmt.Errorf("failed to read GzipData: %v", err)
	}

	// Read IsDownload
	isDownloadByte, err := buf.ReadByte()
	if err != nil {
		return fmt.Errorf("failed to read IsDownload: %v", err)
	}
	cf.IsDownload = isDownloadByte == 1

	// Read ExpiresAt
	var expiresAtUnix int64
	if err := binary.Read(buf, binary.LittleEndian, &expiresAtUnix); err != nil {
		return fmt.Errorf("failed to read ExpiresAt: %v", err)
	}
	cf.ExpiresAt = time.Unix(expiresAtUnix, 0)

	// Read FileStat
	var fileStatModTimeUnix int64
	if err := binary.Read(buf, binary.LittleEndian, &fileStatModTimeUnix); err != nil {
		return fmt.Errorf("failed to read FileStat.ModTime: %v", err)
	}
	cf.FileStat.ModTime = time.Unix(fileStatModTimeUnix, 0)

	if err := binary.Read(buf, binary.LittleEndian, &cf.FileStat.Size); err != nil {
		return fmt.Errorf("failed to read FileStat.Size: %v", err)
	}

	existsByte, err := buf.ReadByte()
	if err != nil {
		return fmt.Errorf("failed to read FileStat.Exists: %v", err)
	}
	cf.FileStat.Exists = existsByte == 1

	return nil
}

// FileStat stores file information for validation
type FileStat struct {
	ModTime time.Time
	Size    int64
	Exists  bool
}

// NewFileCache creates a new file cache using Olric
func NewFileCache(olricClient *olric.EmbeddedClient) (*FileCache, error) {
	if olricClient == nil {
		return nil, fmt.Errorf("olric client is nil")
	}

	dmap, err := olricClient.NewDMap(AssetsCacheNamespace)
	if err != nil {
		return nil, fmt.Errorf("failed to create Olric DMap for assets cache: %v", err)
	}

	fc := &FileCache{
		cache: dmap,
	}

	return fc, nil
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
		file.ExpiresAt = calculateExpiry(file.MimeType, file.Path)
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
		log.Printf("Error setting key %s in Olric cache: %v", key, err)
	}
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

// Global file cache - will be initialized in CreateDbAssetHandler
var fileCache *FileCache

// ShutdownFileCache properly shuts down the global file cache
// This should be called during application shutdown
func ShutdownFileCache() {
	if fileCache != nil {
		fileCache.Close()
	}
}

// CreateDbAssetHandler optimized for static file serving with aggressive caching
func CreateDbAssetHandler(cruds map[string]*resource.DbResource, olricClient *olric.EmbeddedClient) func(*gin.Context) {
	// Initialize the global file cache with Olric
	var err error
	fileCache, err = NewFileCache(olricClient)
	if err != nil {
		log.Printf("Failed to initialize Olric file cache: %v. Using nil cache.", err)
		// Continue without cache
	}
	return AssetRouteHandler(cruds)
}
