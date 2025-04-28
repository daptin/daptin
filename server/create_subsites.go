package server

import (
	"bytes"
	"compress/gzip"
	"github.com/artpar/stats"
	limit "github.com/aviddiviner/gin-limit"
	"github.com/daptin/daptin/server/assetcachepojo"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/cloud_store"
	"github.com/daptin/daptin/server/hostswitch"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/rootpojo"
	"github.com/daptin/daptin/server/subsite"
	"github.com/daptin/daptin/server/task"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	limit2 "github.com/yangxikun/gin-limit-by-key"
	"golang.org/x/time/rate"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
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
var subsiteFileCache sync.Map
var cacheSizeCount int64
var cacheSizeMutex sync.Mutex

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
		cacheSizeMutex.Lock()
		defer cacheSizeMutex.Unlock()

		// If adding this would exceed the cache size, try to remove some entries
		if cacheSizeCount+entrySize > CacheConfig.MaxCacheSize {
			evictCacheEntries(entrySize)
		}

		// Update cache size
		subsiteFileCache.Store(cacheKey, entry)
		cacheSizeCount += entrySize
	} else {
		// No max size, just add it
		subsiteFileCache.Store(cacheKey, entry)
	}
}

// evictCacheEntries removes entries to make room for new ones
func evictCacheEntries(requiredSpace int64) {
	// Simple LRU-like eviction - remove oldest entries first
	// In a production system, you'd want a proper LRU implementation
	var entriesToRemove []string
	var removedSize int64

	// Find entries to remove
	subsiteFileCache.Range(func(key, value interface{}) bool {
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
		if val, ok := subsiteFileCache.Load(key); ok {
			entry := val.(*SubsiteCacheEntry)
			entrySize := int64(len(entry.Content))
			if entry.CompressedContent != nil {
				entrySize += int64(len(entry.CompressedContent))
			}

			subsiteFileCache.Delete(key)
			cacheSizeCount -= entrySize
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
	subsiteFileCache.Range(func(key, value interface{}) bool {
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
		cacheSizeMutex.Lock()
		defer cacheSizeMutex.Unlock()

		for _, key := range entriesToRemove {
			if val, ok := subsiteFileCache.Load(key); ok {
				entry := val.(*SubsiteCacheEntry)
				entrySize := int64(len(entry.Content))
				if entry.CompressedContent != nil {
					entrySize += int64(len(entry.CompressedContent))
				}

				subsiteFileCache.Delete(key)
				cacheSizeCount -= entrySize
			}
		}

		log.Infof("Removed %d expired entries from cache, freed %d bytes", len(entriesToRemove), removedSize)
	}
}

// invalidateSiteCache removes all cache entries for a given site
func invalidateSiteCache(hostname string) {
	var removedEntries int
	var removedSize int64

	subsiteFileCache.Range(func(key, value interface{}) bool {
		cacheKey := key.(string)
		if strings.HasPrefix(cacheKey, hostname+"::") {
			entry := value.(*SubsiteCacheEntry)

			// Calculate size for bookkeeping
			entrySize := int64(len(entry.Content))
			if entry.CompressedContent != nil {
				entrySize += int64(len(entry.CompressedContent))
			}

			subsiteFileCache.Delete(key)
			removedEntries++
			removedSize += entrySize
		}
		return true // continue iteration
	})

	if removedEntries > 0 {
		cacheSizeMutex.Lock()
		cacheSizeCount -= removedSize
		cacheSizeMutex.Unlock()
		log.Infof("Invalidated cache for site %s: removed %d entries, freed %d bytes",
			hostname, removedEntries, removedSize)
	}
}

func CreateSubSites(cmsConfig *resource.CmsConfig, transaction *sqlx.Tx,
	cruds map[string]*resource.DbResource, authMiddleware *auth.AuthMiddleware,
	rateConfig RateConfig, max_connections int) (hostswitch.HostSwitch, map[daptinid.DaptinReferenceId]*assetcachepojo.AssetFolderCache) {

	router := httprouter.New()
	router.ServeFiles("/*filepath", http.Dir("./scripts"))

	hs := hostswitch.HostSwitch{
		AdministratorGroupId: cruds["usergroup"].AdministratorGroupId,
	}
	subsiteCacheFolders := make(map[daptinid.DaptinReferenceId]*assetcachepojo.AssetFolderCache)
	hs.HandlerMap = make(map[string]*gin.Engine)
	hs.SiteMap = make(map[string]subsite.SubSite)
	hs.AuthMiddleware = authMiddleware

	// Start the cache cleanup routine
	startCacheCleanupRoutine()

	//log.Printf("Cruds before making sub sits: %v", cruds)
	sites, err := subsite.GetAllSites(cruds["site"], transaction)
	if err != nil {
		log.Printf("Failed to get all sites 117: %v", err)
	}
	stores, err := cloud_store.GetAllCloudStores(cruds["cloud_store"], transaction)
	if err != nil {
		log.Printf("Failed to get all cloudstores 121: %v", err)
	}
	cloudStoreMap := make(map[int64]rootpojo.CloudStore)

	adminEmailId := cruds[resource.USER_ACCOUNT_TABLE_NAME].GetAdminEmailId(transaction)
	log.Printf("Admin email id: %s", adminEmailId)

	for _, store := range stores {
		cloudStoreMap[store.Id] = store
	}

	siteMap := make(map[string]resource.SubSiteInformation)

	if err != nil {
		log.Errorf("Failed to load sites from database: %v", err)
		return hs, subsiteCacheFolders
	}

	//max_connections, err := configStore.GetConfigIntValueFor("limit.max_connections", "backend")
	//rate_limit, err := configStore.GetConfigIntValueFor("limit.rate", "backend")

	for _, site := range sites {

		if !site.Enable {
			continue
		}

		subSiteInformation := resource.SubSiteInformation{}
		//hs.SiteMap[site.Path] = site

		for _, hostname := range strings.Split(site.Hostname, ",") {
			hs.SiteMap[hostname] = site
		}

		subSiteInformation.SubSite = site

		if site.CloudStoreId == nil {
			log.Printf("Site [%v] does not have a associated storage", site.Name)
			continue
		}

		u, _ := uuid.NewV7()
		sourceDirectoryName := u.String()
		tempDirectoryPath, err := ioutil.TempDir(os.Getenv("DAPTIN_CACHE_FOLDER"), sourceDirectoryName)
		if resource.CheckErr(err, "Failed to create temp directory") {
			continue
		}
		subSiteInformation.SourceRoot = tempDirectoryPath
		cloudStore, ok := cloudStoreMap[*site.CloudStoreId]
		subSiteInformation.CloudStore = cloudStore
		if !ok {
			log.Printf("Site [%v] does not have a associated storage", site.Name)
			continue
		}

		err = cruds["task"].SyncStorageToPath(cloudStore, site.Path, tempDirectoryPath, transaction)
		if resource.CheckErr(err, "Failed to setup sync to path for subsite [%v]", site.Name) {
			continue
		}

		// Clear cache for this site when syncing
		invalidateSiteCache(site.Hostname)

		syncTask := task.Task{
			EntityName: "site",
			ActionName: "sync_site_storage",
			Attributes: map[string]interface{}{
				"site_id": site.ReferenceId.String(),
				"path":    tempDirectoryPath,
			},
			AsUserEmail: adminEmailId,
			Schedule:    "@every 1h",
		}

		activeTask := cruds["site"].NewActiveTaskInstance(syncTask)

		func(task *resource.ActiveTaskInstance) {
			go func() {
				log.Info("Sleep 5 sec for running new sync task")
				time.Sleep(5 * time.Second)
				activeTask.Run()
				// Invalidate cache after sync
				invalidateSiteCache(site.Hostname)
			}()
		}(activeTask)

		err = TaskScheduler.AddTask(syncTask)

		subsiteCacheFolders[site.ReferenceId] = &assetcachepojo.AssetFolderCache{
			LocalSyncPath: tempDirectoryPath,
			Keyname:       site.Path,
			CloudStore:    cloudStore,
		}

		resource.CheckErr(err, "Failed to register task to sync storage")

		subsiteStats := stats.New()
		hostRouter := gin.New()

		// We're using our own compression implementation instead of middleware

		hostRouter.Use(func() gin.HandlerFunc {
			return func(c *gin.Context) {
				beginning, recorder := subsiteStats.Begin(c.Writer)
				defer Stats.End(beginning, stats.WithRecorder(recorder))
				c.Next()
			}
		}())

		hostRouter.Use(gin.Logger())
		hostRouter.Use(limit.MaxAllowed(max_connections))
		hostRouter.Use(limit2.NewRateLimiter(func(c *gin.Context) string {
			requestPath := c.Request.Host + "/" + strings.Split(c.Request.RequestURI, "?")[0]
			return c.ClientIP() + requestPath // limit rate by client ip
		}, func(c *gin.Context) (*rate.Limiter, time.Duration) {
			requestPath := c.Request.Host + "/" + strings.Split(c.Request.RequestURI, "?")[0]
			limitValue, ok := rateConfig.limits[requestPath]
			if !ok {
				limitValue = 100
			}

			return rate.NewLimiter(rate.Every(100*time.Millisecond), limitValue), time.Hour // limit 10 qps/clientIp and permit bursts of at most 10 tokens, and the limiter liveness time duration is 1 hour
		}, func(c *gin.Context) {
			c.AbortWithStatus(429) // handle exceed rate limit request
		}))

		hostRouter.GET("/stats", func(c *gin.Context) {
			c.JSON(200, subsiteStats.Data())
		})

		//hostRouter.ServeFiles("/*filepath", http.Dir(tempDirectoryPath))
		hostRouter.Use(authMiddleware.AuthCheckMiddleware)

		log.Tracef("Serve subsite[%s] from source [%s]", site.Name, tempDirectoryPath)

		// Create a custom middleware for serving static files with aggressive caching
		hostRouter.Use(func(c *gin.Context) {
			path := c.Request.URL.Path
			var filePath string

			if site.SiteType == "hugo" {
				filePath = filepath.Join(tempDirectoryPath, "public", path)
			} else {
				filePath = filepath.Join(tempDirectoryPath, path)
			}

			// Handle directory paths by appending index.html
			fileInfo, err := os.Stat(filePath)
			if err == nil && fileInfo.IsDir() {
				filePath = filepath.Join(filePath, "index.html")
			}

			// Generate a cache key for this request
			cacheKey := getSubsiteCacheKey(c.Request.Host, path)

			// Check if we have this file in cache and if it's still valid
			if cachedEntry, found := subsiteFileCache.Load(cacheKey); found {
				entry := cachedEntry.(*SubsiteCacheEntry)

				// Check if entry is expired or file has been modified
				if isCacheExpired(entry) {
					// Cache entry expired or file modified, remove it from cache
					subsiteFileCache.Delete(cacheKey)

					// Update cache size tracking
					cacheSizeMutex.Lock()
					entrySize := int64(len(entry.Content))
					if entry.CompressedContent != nil {
						entrySize += int64(len(entry.CompressedContent))
					}
					cacheSizeCount -= entrySize
					cacheSizeMutex.Unlock()

					// Continue to read the file from disk
				} else {
					// Valid cache entry, check if client has a valid cached version
					clientETag := c.Request.Header.Get("If-None-Match")
					if clientETag == entry.ETag {
						c.Writer.WriteHeader(http.StatusNotModified)
						return
					}

					// Set cache headers
					c.Writer.Header().Set("ETag", entry.ETag)
					c.Writer.Header().Set("Cache-Control", "public, max-age=3600") // 1 hour
					c.Writer.Header().Set("Last-Modified", entry.LastModified.Format(http.TimeFormat))
					c.Writer.Header().Set("Content-Type", entry.ContentType)

					// Check if client accepts gzip encoding and we have compressed content
					if entry.CompressedContent != nil && len(entry.CompressedContent) > 0 &&
						strings.Contains(c.Request.Header.Get("Accept-Encoding"), "gzip") {
						c.Writer.Header().Set("Content-Encoding", "gzip")
						c.Writer.Header().Set("Vary", "Accept-Encoding")
						c.Writer.WriteHeader(http.StatusOK)
						c.Writer.Write(entry.CompressedContent)
					} else {
						c.Writer.WriteHeader(http.StatusOK)
						c.Writer.Write(entry.Content)
					}
					c.Abort()
					return
				}
			}

			// If not in cache or expired, try to read the file
			content, err := os.ReadFile(filePath)
			fileInfo1, _ := os.Stat(filePath)
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
				etag := generateETag(content, fileInfo1.ModTime())
				lastModified := time.Now()

				// Compress content if it's a compressible type
				var compressedContent []byte
				if shouldCompress(contentType) {
					compressed, err := compressContent(content)
					if err == nil {
						compressedContent = compressed
					}
				}

				// Cache the file with expiration
				cacheEntry := &SubsiteCacheEntry{
					ETag:              etag,
					Content:           content,
					CompressedContent: compressedContent,
					ContentType:       contentType,
					LastModified:      lastModified,
					FilePath:          filePath,
					ExpiresAt:         time.Now().Add(CacheConfig.DefaultTTL),
				}

				// Add to cache with size management
				addToCache(cacheKey, cacheEntry)

				// Set cache headers
				c.Writer.Header().Set("ETag", etag)
				c.Writer.Header().Set("Cache-Control", "public, max-age=3600") // 1 hour
				c.Writer.Header().Set("Last-Modified", lastModified.Format(http.TimeFormat))
				c.Writer.Header().Set("Content-Type", contentType)
				c.Writer.Header().Set("Vary", "Accept-Encoding")

				// Check if client accepts gzip encoding and we have compressed content
				if compressedContent != nil && len(compressedContent) > 0 &&
					strings.Contains(c.Request.Header.Get("Accept-Encoding"), "gzip") {
					c.Writer.Header().Set("Content-Encoding", "gzip")
					c.Writer.WriteHeader(http.StatusOK)
					c.Writer.Write(compressedContent)
				} else {
					c.Writer.WriteHeader(http.StatusOK)
					c.Writer.Write(content)
				}
				return
			}

			// Fallback to standard file serving if reading fails
			// Try to read and cache index.html with compression
			indexPath := filepath.Join(tempDirectoryPath, "index.html")
			indexContent, err := os.ReadFile(indexPath)
			fileinfo, err := os.Stat(indexPath)
			if err == nil {
				// Generate ETag
				indexEtag := generateETag(indexContent, fileinfo.ModTime())
				indexLastModified := time.Now()

				// Compress the index.html content
				var compressedIndexContent []byte
				compressedIndex, err := compressContent(indexContent)
				if err == nil {
					compressedIndexContent = compressedIndex
				}

				// Cache the index.html with expiration
				indexCacheKey := getSubsiteCacheKey(c.Request.Host, "/index.html")
				indexCacheEntry := &SubsiteCacheEntry{
					ETag:              indexEtag,
					Content:           indexContent,
					CompressedContent: compressedIndexContent,
					ContentType:       "text/html; charset=utf-8",
					LastModified:      indexLastModified,
					FilePath:          indexPath,
					ExpiresAt:         time.Now().Add(CacheConfig.DefaultTTL),
				}

				// Add to cache with size management
				addToCache(indexCacheKey, indexCacheEntry)

				// Set cache headers
				c.Writer.Header().Set("ETag", indexEtag)
				c.Writer.Header().Set("Cache-Control", "public, max-age=3600") // 1 hour
				c.Writer.Header().Set("Last-Modified", indexLastModified.Format(http.TimeFormat))
				c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
				c.Writer.Header().Set("Vary", "Accept-Encoding")

				// Serve compressed content if client accepts it
				if compressedIndexContent != nil && strings.Contains(c.Request.Header.Get("Accept-Encoding"), "gzip") {
					c.Writer.Header().Set("Content-Encoding", "gzip")
					c.Writer.WriteHeader(http.StatusOK)
					c.Writer.Write(compressedIndexContent)
				} else {
					c.Writer.WriteHeader(http.StatusOK)
					c.Writer.Write(indexContent)
				}
				return
			}

			// If reading fails, fallback to standard file serving
			c.File(indexPath)
		})

		// Add a cache invalidation endpoint for manual cache clearing
		//hostRouter.Handle("GET", "/_clear_cache", func(c *gin.Context) {
		//	if c.Query("secret") == os.Getenv("DAPTIN_ADMIN_SECRET") {
		//		invalidateSiteCache(c.Request.Host)
		//		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Cache cleared"})
		//	} else {
		//		c.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "Unauthorized"})
		//	}
		//})

		hostRouter.Handle("GET", "/statistics", func(c *gin.Context) {
			c.JSON(http.StatusOK, Stats.Data())
		})

		hs.HandlerMap[site.Hostname] = hostRouter
		siteMap[subSiteInformation.SubSite.Hostname] = subSiteInformation
		//SiteMap[subSiteInformation.SubSite.Path] = subSiteInformation
	}

	cmsConfig.SubSites = siteMap

	return hs, subsiteCacheFolders
}
