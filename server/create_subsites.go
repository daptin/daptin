package server

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"fmt"
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

// SubsiteCacheEntry represents a cached file with its etag and content
type SubsiteCacheEntry struct {
	ETag              string
	Content           []byte
	CompressedContent []byte // Gzip compressed content
	ContentType       string
	LastModified      time.Time
}

// SubsiteFileCache is a global in-memory cache for subsite files
var subsiteFileCache sync.Map

// generateSubsiteETag creates an ETag for a file based on its content
func generateSubsiteETag(content []byte) string {
	hash := md5.Sum(content)
	return fmt.Sprintf("\"%s\"", hex.EncodeToString(hash[:]))
}

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
		subsiteFileCache.Range(func(key, value interface{}) bool {
			cacheKey := key.(string)
			if strings.HasPrefix(cacheKey, site.Hostname) {
				subsiteFileCache.Delete(key)
			}
			return true
		})

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

			// Check if we have this file in cache
			if cachedEntry, found := subsiteFileCache.Load(cacheKey); found {
				entry := cachedEntry.(*SubsiteCacheEntry)

				// Check if client has a valid cached version using If-None-Match header
				clientETag := c.Request.Header.Get("If-None-Match")
				if clientETag == entry.ETag {
					c.Writer.WriteHeader(http.StatusNotModified)
					return
				}

				// Set cache headers
				c.Writer.Header().Set("ETag", entry.ETag)
				c.Writer.Header().Set("Cache-Control", "public, max-age=604800, immutable") // 7 days with immutable directive
				c.Writer.Header().Set("Last-Modified", entry.LastModified.Format(http.TimeFormat))
				c.Writer.Header().Set("Content-Type", entry.ContentType)

				// Check if client accepts gzip encoding and we have compressed content
				if entry.CompressedContent != nil && len(entry.CompressedContent) > 0 && strings.Contains(c.Request.Header.Get("Accept-Encoding"), "gzip") {
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

			// If not in cache, try to read the file
			content, err := os.ReadFile(filePath)
			if err != nil {
				// Let the next middleware handle 404s
				c.Next()
				return
			}

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
			etag := generateSubsiteETag(content)
			lastModified := time.Now()

			// Compress content if it's a compressible type
			var compressedContent []byte
			if shouldCompress(contentType) {
				compressed, err := compressContent(content)
				if err == nil {
					compressedContent = compressed
				}
			}

			// Cache the file
			subsiteFileCache.Store(cacheKey, &SubsiteCacheEntry{
				ETag:              etag,
				Content:           content,
				CompressedContent: compressedContent,
				ContentType:       contentType,
				LastModified:      lastModified,
			})

			// Set cache headers
			c.Writer.Header().Set("ETag", etag)
			c.Writer.Header().Set("Cache-Control", "public, max-age=604800, immutable") // 7 days with immutable directive
			c.Writer.Header().Set("Last-Modified", lastModified.Format(http.TimeFormat))
			c.Writer.Header().Set("Content-Type", contentType)
			c.Writer.Header().Set("Vary", "Accept-Encoding")

			// Check if client accepts gzip encoding and we have compressed content
			if compressedContent != nil && len(compressedContent) > 0 && strings.Contains(c.Request.Header.Get("Accept-Encoding"), "gzip") {
				c.Writer.Header().Set("Content-Encoding", "gzip")
				c.Writer.WriteHeader(http.StatusOK)
				c.Writer.Write(compressedContent)
			} else {
				c.Writer.WriteHeader(http.StatusOK)
				c.Writer.Write(content)
			}
			c.Abort()
			return

			// Fallback to standard file serving if reading fails
			// Try to read and cache index.html with compression
			indexPath := filepath.Join(tempDirectoryPath, "index.html")
			indexContent, err := os.ReadFile(indexPath)
			if err == nil {
				// Generate ETag
				indexEtag := generateSubsiteETag(indexContent)
				indexLastModified := time.Now()

				// Compress the index.html content
				var compressedIndexContent []byte
				compressedIndex, err := compressContent(indexContent)
				if err == nil {
					compressedIndexContent = compressedIndex
				}

				// Cache the index.html
				indexCacheKey := getSubsiteCacheKey(c.Request.Host, "/index.html")
				subsiteFileCache.Store(indexCacheKey, &SubsiteCacheEntry{
					ETag:              indexEtag,
					Content:           indexContent,
					CompressedContent: compressedIndexContent,
					ContentType:       "text/html; charset=utf-8",
					LastModified:      indexLastModified,
				})

				// Set cache headers
				c.Writer.Header().Set("ETag", indexEtag)
				c.Writer.Header().Set("Cache-Control", "public, max-age=604800, immutable") // 7 days with immutable directive
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
