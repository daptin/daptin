package subsite

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/artpar/api2go"
	_ "github.com/artpar/rclone/backend/all" // import all fs
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/dbresourceinterface"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/permission"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type JsonApiError struct {
	Message string
}

type SubSite struct {
	Id           int64
	Name         string
	Hostname     string
	Path         string
	CloudStoreId *int64 `db:"cloud_store_id"`
	Permission   permission.PermissionInstance
	SiteType     string                     `db:"site_type"`
	FtpEnabled   bool                       `db:"ftp_enabled"`
	UserId       *int64                     `db:"user_account_id"`
	ReferenceId  daptinid.DaptinReferenceId `db:"reference_id"`
	Enable       bool                       `db:"enable"`
}

type HostRouterProvider interface {
	GetHostRouter(name string) *gin.Engine
	GetAllRouter() []*gin.Engine
}

// CacheEntry represents a single entry in the in-memory cache
type CacheEntry struct {
	Content     string
	MimeType    string
	Headers     map[string]string
	ETag        string
	CreatedAt   time.Time
	LastAccess  time.Time
	AccessCount int
}

// InMemoryCache provides a thread-safe in-memory cache implementation
type InMemoryCache struct {
	Entries    map[string]*CacheEntry
	MaxSize    int
	Strategy   string
	Mutex      sync.RWMutex
	Compressed bool
}

// Global cache instance
var globalCache *InMemoryCache
var cacheOnce sync.Once

// GetInMemoryCache returns the singleton cache instance
func GetInMemoryCache() *InMemoryCache {
	cacheOnce.Do(func() {
		globalCache = &InMemoryCache{
			Entries:    make(map[string]*CacheEntry),
			MaxSize:    100, // Default max size
			Strategy:   "lru",
			Compressed: false,
		}
	})
	return globalCache
}

// ConfigureCache updates the cache configuration
func (c *InMemoryCache) ConfigureCache(config *CacheConfig) {
	if config == nil {
		return
	}

	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	if config.InMemoryCacheMaxSize > 0 {
		c.MaxSize = config.InMemoryCacheMaxSize
	}

	if config.InMemoryCacheStrategy != "" {
		c.Strategy = config.InMemoryCacheStrategy
	}

	c.Compressed = config.InMemoryCacheCompression
}

// generateCacheKey creates a unique key for the cache based on the request and configuration
func generateCacheKey(c *gin.Context, config *CacheConfig) string {
	if config == nil {
		return ""
	}

	// Start with the path
	key := c.Request.URL.Path

	// Add prefix if configured
	if config.CacheKeyPrefix != "" {
		key = config.CacheKeyPrefix + ":" + key
	}

	// Add query parameters if configured to vary by them
	if len(config.VaryByQueryParams) > 0 {
		queryValues := c.Request.URL.Query()
		for _, param := range config.VaryByQueryParams {
			if values, exists := queryValues[param]; exists {
				for _, value := range values {
					key += ":" + param + "=" + value
				}
			}
		}
	}

	// Add headers if configured to vary by them
	if len(config.VaryByHeaders) > 0 {
		for _, header := range config.VaryByHeaders {
			value := c.GetHeader(header)
			if value != "" {
				key += ":" + header + "=" + value
			}
		}
	}

	return key
}

// Get retrieves an entry from the cache if it exists and is not expired
func (c *InMemoryCache) Get(key string, ttl int) (*CacheEntry, bool) {
	c.Mutex.RLock()
	defer c.Mutex.RUnlock()

	entry, exists := c.Entries[key]
	if !exists {
		return nil, false
	}

	// Check if entry is expired
	if ttl > 0 && time.Since(entry.CreatedAt).Seconds() > float64(ttl) {
		return nil, false
	}

	// Update access statistics
	entry.LastAccess = time.Now()
	entry.AccessCount++

	return entry, true
}

// Set adds or updates an entry in the cache
func (c *InMemoryCache) Set(key string, content string, mimeType string, headers map[string]string, etag string) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()

	// Check if we need to evict entries
	if len(c.Entries) >= c.MaxSize {
		c.evictEntries()
	}

	// Create new entry
	entry := &CacheEntry{
		Content:     content,
		MimeType:    mimeType,
		Headers:     headers,
		ETag:        etag,
		CreatedAt:   time.Now(),
		LastAccess:  time.Now(),
		AccessCount: 1,
	}

	c.Entries[key] = entry
}

// evictEntries removes entries based on the configured strategy
func (c *InMemoryCache) evictEntries() {
	// Determine how many entries to evict (20% of max size or at least 1)
	evictCount := c.MaxSize / 5
	if evictCount < 1 {
		evictCount = 1
	}

	// Create a slice of entries for sorting
	entries := make([]*CacheEntry, 0, len(c.Entries))
	keys := make([]string, 0, len(c.Entries))

	for key, entry := range c.Entries {
		entries = append(entries, entry)
		keys = append(keys, key)
	}

	// Sort based on strategy
	if c.Strategy == "lru" {
		// Sort by last access time (oldest first)
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].LastAccess.Before(entries[j].LastAccess)
		})
	} else if c.Strategy == "lfu" {
		// Sort by access count (least frequently used first)
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].AccessCount < entries[j].AccessCount
		})
	}

	// Create a map of entries to their keys
	entryToKey := make(map[*CacheEntry]string)
	for i, entry := range entries {
		entryToKey[entry] = keys[i]
	}

	// Remove the entries
	for i := 0; i < evictCount && i < len(entries); i++ {
		delete(c.Entries, entryToKey[entries[i]])
	}
}

func CreateTemplateHooks(transaction *sqlx.Tx, cruds map[string]dbresourceinterface.DbResourceInterface, hostSwitch HostRouterProvider) error {
	allRouters := hostSwitch.GetAllRouter()
	templateList, err := cruds["template"].GetAllObjects("template", transaction)
	log.Infof("Got [%d] Templates from database", len(templateList))
	if err != nil {
		return err
	}
	handlerCreator := CreateTemplateRouteHandler(cruds, transaction)
	for _, templateRow := range templateList {
		log.Infof("ProcessTemplateRoute [%s] %v", templateRow["name"], templateRow["url_pattern"])
		urlPattern := templateRow["url_pattern"].(string)
		strArray := make([]string, 0)
		err = json.Unmarshal([]byte(urlPattern), &strArray)
		if err != nil {
			log.Errorf("Failed to parse url pattern [%v] as string array [%v]", urlPattern, err)
			continue
		}
		templateRenderHelper := handlerCreator(templateRow)
		for _, urlMatch := range strArray {
			log.Infof("TemplateRoute [%s] => %s", urlMatch, templateRow["name"])
			for _, router := range allRouters {
				router.Any(urlMatch, templateRenderHelper)
			}
		}
	}
	return nil
}

func CreateTemplateRouteHandler(cruds map[string]dbresourceinterface.DbResourceInterface, transaction *sqlx.Tx) func(template map[string]interface{}) func(ginContext *gin.Context) {
	return func(templateInstance map[string]interface{}) func(ginContext *gin.Context) {

		templateName := templateInstance["name"].(string)
		actionConfigInterface := templateInstance["action_config"]
		cacheConfigInterface := templateInstance["cache_config"]
		actionRequest, err := GetActionConfig(actionConfigInterface)
		if err != nil {
			log.Errorf("Failed to get template instance for template [%v]", templateName)
		}

		var cacheConfig *CacheConfig
		cacheConfig, err = GetCacheConfig(cacheConfigInterface)
		if err != nil {
			log.Errorf("Failed to get template instance for template [%v]", templateName)
		}
		// Get the in-memory cache instance
		cache := GetInMemoryCache()

		// Configure the cache based on the current config
		if cacheConfig != nil {
			cache.ConfigureCache(cacheConfig)
		}

		return func(ginContext *gin.Context) {

			// Apply caching configuration if available
			log.Tracef("Serve subsite[%s] reqeust[%s]", templateName, ginContext.Request.URL.Path)
			if cacheConfig != nil && cacheConfig.Enable {
				// Apply cache control headers based on configuration
				applyCacheHeaders(ginContext, cacheConfig)

				// Check if we can serve from cache using ETag or other cache validators
				if checkCacheValidators(ginContext, cacheConfig) {
					// Return 304 Not Modified if client has valid cached version
					ginContext.Writer.WriteHeader(http.StatusNotModified)
					return
				}

				// Check if we can serve the response from in-memory cache
				if cacheConfig.EnableInMemoryCache {
					cacheKey := generateCacheKey(ginContext, cacheConfig)
					if cacheKey != "" {
						if entry, found := cache.Get(cacheKey, cacheConfig.InMemoryCacheTTL); found {
							// Serve from cache
							log.Infof("Serving response from in-memory cache for %s", ginContext.Request.URL.Path)

							// Set ETag if available
							if entry.ETag != "" {
								ginContext.Writer.Header().Set("ETag", entry.ETag)
							}

							// Set content type
							ginContext.Writer.Header().Set("Content-Type", entry.MimeType)

							// Set custom headers
							for key, value := range entry.Headers {
								ginContext.Writer.Header().Set(key, value)
							}

							// Set cache hit header for debugging
							ginContext.Writer.Header().Set("X-Cache", "HIT")

							// Write the response
							ginContext.Writer.WriteHeader(http.StatusOK)
							fmt.Fprint(ginContext.Writer, entry.Content)
							ginContext.Writer.Flush()
							return
						}
					}
				}
			}

			inFields := make(map[string]interface{})

			inFields["template"] = templateName
			queryMap := make(map[string]interface{})
			inFields["query"] = queryMap

			for _, param := range ginContext.Params {
				inFields[param.Key] = param.Value
			}

			queryParams := ginContext.Request.URL.Query()

			for key, valArray := range queryParams {
				if len(valArray) == 1 {
					inFields[key] = valArray[0]
					inFields[key+"[]"] = valArray
				} else {
					inFields[key] = valArray
					inFields[key+"[]"] = valArray
				}
			}

			var outcomeRequest actionresponse.Outcome
			var transaction1 *sqlx.Tx
			var errTxn error
			transaction1, errTxn = cruds["world"].Connection().Beginx()
			if errTxn != nil {
				_ = ginContext.AbortWithError(500, errTxn)
				return
			}
			defer func() {
				_ = transaction1.Commit()
			}()

			var api2goRequestData = api2go.Request{
				PlainRequest: ginContext.Request,
				QueryParams:  ginContext.Request.URL.Query(),
				Pagination:   nil,
				Header:       ginContext.Request.Header,
				Context:      nil,
			}

			if len(actionRequest.Action) > 0 && len(actionRequest.Type) > 0 {
				actionRequest.Attributes = inFields
				actionResponses, errAction := cruds["action"].HandleActionRequest(actionRequest, api2goRequestData, transaction1)
				if errAction != nil {
					_ = ginContext.AbortWithError(500, errAction)
					return
				}
				inFields["actionResponses"] = actionResponses

				for _, actionResponse := range actionResponses {
					inFields[actionResponse.ResponseType] = actionResponse.Attributes
				}
			}

			api2goResponder, _, err := cruds["world"].GetActionHandler("template.render").DoAction(
				outcomeRequest, inFields, transaction1)
			if err != nil && len(err) > 0 {
				_ = ginContext.AbortWithError(500, err[0])
				return
			}

			api2GoResult := api2goResponder.Result().(api2go.Api2GoModel)

			attrs := api2GoResult.GetAttributes()
			var content = attrs["content"].(string)
			var mimeType = attrs["mime_type"].(string)
			var headers = attrs["headers"].(map[string]string)

			// Decode content first to use for ETag generation if needed
			decodedContent := Atob(content)

			// Variable to store ETag if generated
			var etag string

			// Generate ETag if configured and add it to response headers
			if cacheConfig != nil && cacheConfig.Enable && cacheConfig.ETagStrategy != "none" {
				etag = generateETag(decodedContent, cacheConfig.ETagStrategy)
				if etag != "" {
					ginContext.Writer.Header().Set("ETag", etag)
				}

				// Set Last-Modified header for cache validation
				ginContext.Writer.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
			}

			// Apply VaryByQueryParams if configured
			if cacheConfig != nil && cacheConfig.Enable && len(cacheConfig.VaryByQueryParams) > 0 {
				// Add Vary header for query parameters
				// This is a custom implementation as HTTP doesn't directly support varying by query params
				// We'll add a custom header to indicate which query params affect caching
				ginContext.Writer.Header().Set("X-Vary-By-Query-Params", strings.Join(cacheConfig.VaryByQueryParams, ", "))
			}

			// Prepare response headers
			ginContext.Writer.Header().Set("Content-Type", mimeType)
			for hKey, hValue := range headers {
				ginContext.Writer.Header().Set(hKey, hValue)
			}

			// Set cache miss header for debugging
			ginContext.Writer.Header().Set("X-Cache", "MISS")

			// Write status and flush headers
			ginContext.Writer.WriteHeader(http.StatusOK)
			ginContext.Writer.Flush()

			// Render the content
			fmt.Fprint(ginContext.Writer, decodedContent)
			ginContext.Writer.Flush()

			// Store in cache if in-memory caching is enabled
			if cacheConfig != nil && cacheConfig.Enable && cacheConfig.EnableInMemoryCache {
				cacheKey := generateCacheKey(ginContext, cacheConfig)
				if cacheKey != "" {
					// Clone headers to avoid reference issues
					cachedHeaders := make(map[string]string)
					for k, v := range headers {
						cachedHeaders[k] = v
					}

					// Store in cache
					cache := GetInMemoryCache()
					cache.Set(cacheKey, decodedContent, mimeType, cachedHeaders, etag)
					log.Infof("Stored response in in-memory cache for %s", ginContext.Request.URL.Path)
				}
			}

		}
	}
}

func Atob(data string) string {
	decodedData, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		log.Printf("Atob failed: %v", err)
		return ""
	}
	return string(decodedData)
}

// applyCacheHeaders applies cache control headers based on the provided CacheConfig
func applyCacheHeaders(c *gin.Context, config *CacheConfig) {
	if config == nil || !config.Enable {
		return
	}

	// Build Cache-Control header
	cacheControl := ""

	// Handle no-store directive (highest priority, prevents caching)
	if config.NoStore {
		cacheControl = "no-store"
		c.Header("Cache-Control", cacheControl)
		return
	}

	// Handle no-cache directive
	if config.NoCache {
		cacheControl = "no-cache"
	}

	// Set privacy directive
	if config.Private {
		if len(cacheControl) > 0 {
			cacheControl += ", "
		}
		cacheControl += "private"
	} else {
		if len(cacheControl) > 0 {
			cacheControl += ", "
		}
		cacheControl += "public"
	}

	// Set max-age directive
	if config.MaxAge > 0 {
		if len(cacheControl) > 0 {
			cacheControl += ", "
		}
		cacheControl += fmt.Sprintf("max-age=%d", config.MaxAge)
	}

	// Set must-revalidate directive
	if config.Revalidate {
		if len(cacheControl) > 0 {
			cacheControl += ", "
		}
		cacheControl += "must-revalidate"
	}

	// Set stale-while-revalidate directive
	if config.StaleWhileRevalidate > 0 {
		if len(cacheControl) > 0 {
			cacheControl += ", "
		}
		cacheControl += fmt.Sprintf("stale-while-revalidate=%d", config.StaleWhileRevalidate)
	}

	// Apply Cache-Control header
	if len(cacheControl) > 0 {
		c.Header("Cache-Control", cacheControl)
	}

	// Apply Expires header if configured
	if config.ExpiresAt != nil {
		c.Header("Expires", config.ExpiresAt.Format(http.TimeFormat))
	} else if config.MaxAge > 0 {
		// Set Expires based on max-age if ExpiresAt not explicitly set
		expiresTime := time.Now().Add(time.Duration(config.MaxAge) * time.Second)
		c.Header("Expires", expiresTime.Format(http.TimeFormat))
	}

	// Apply Vary header based on configuration
	if len(config.VaryByHeaders) > 0 {
		c.Header("Vary", strings.Join(config.VaryByHeaders, ", "))
	}

	// Apply custom headers
	if config.CustomHeaders != nil {
		for key, value := range config.CustomHeaders {
			c.Header(key, value)
		}
	}
}

// generateETag generates an ETag for the response based on the configured strategy
func generateETag(content string, strategy string) string {
	if strategy == "none" {
		return ""
	}

	// Create a hash of the content
	hash := sha256.Sum256([]byte(content))
	etag := hex.EncodeToString(hash[:8]) // Use first 8 bytes for brevity

	if strategy == "weak" {
		return fmt.Sprintf("W/\"%s\"", etag)
	}

	// Strong ETag
	return fmt.Sprintf("\"%s\"", etag)
}

// checkCacheValidators checks if the client's cached version is still valid
func checkCacheValidators(c *gin.Context, config *CacheConfig) bool {
	if config == nil || !config.Enable {
		return false
	}

	// If no-store or no-cache is set, we shouldn't use validators
	if config.NoStore || config.NoCache {
		return false
	}

	// Check If-None-Match header against ETag
	ifNoneMatch := c.GetHeader("If-None-Match")
	if len(ifNoneMatch) > 0 && config.ETagStrategy != "none" {
		// In a real implementation, we would compare against the actual ETag
		// For now, we'll assume if the header exists, it might match
		// In a complete implementation, we would need to store and retrieve ETags
		// This is a placeholder for the actual implementation
		return true
	}

	// Check If-Modified-Since header
	ifModifiedSince := c.GetHeader("If-Modified-Since")
	if len(ifModifiedSince) > 0 {
		// Parse the If-Modified-Since header
		modifiedSinceTime, err := time.Parse(http.TimeFormat, ifModifiedSince)
		if err == nil {
			// In a real implementation, we would compare against the actual last modified time
			// For now, we'll use a simple time comparison
			// This is a placeholder for the actual implementation
			return time.Now().Before(modifiedSinceTime)
		}
	}

	return false
}
