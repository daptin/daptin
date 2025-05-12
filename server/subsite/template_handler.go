package subsite

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/artpar/api2go"
	_ "github.com/artpar/rclone/backend/all" // import all fs
	"github.com/buraksezer/olric"
	"github.com/daptin/daptin/server"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/dbresourceinterface"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/permission"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"net/http"
	"path/filepath"
	"strings"
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

var fileCache *server.FileCache

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

func CreateTemplateHooks(transaction *sqlx.Tx, cruds map[string]dbresourceinterface.DbResourceInterface, hostSwitch HostRouterProvider, olricDb *olric.EmbeddedClient) error {
	allRouters := hostSwitch.GetAllRouter()
	templateList, err := cruds["template"].GetAllObjects("template", transaction)
	log.Infof("Got [%d] Templates from database", len(templateList))
	if err != nil {
		return err
	}
	if fileCache == nil {
		fileCache, err = server.NewFileCache(olricDb, "template-cache")
		CheckErr(err, "Failed to create olric template cache")
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
		// Get the Olric cache instance
		log.Infof("Cache config for [%v] [%v]", templateName, cacheConfig)

		return func(c *gin.Context) {

			// Apply caching configuration if available
			log.Tracef("Serve subsite[%s] request[%s]", templateName, c.Request.URL.Path)
			if cacheConfig != nil && cacheConfig.Enable {
				// Apply cache control headers based on configuration
				applyCacheHeaders(c, cacheConfig)

				// Check if we can serve from cache using ETag or other cache validators
				if checkCacheValidators(c, cacheConfig) {
					// Return 304 Not Modified if client has valid cached version
					c.Writer.WriteHeader(http.StatusNotModified)
					return
				}

				// Check if we can serve the response from in-memory cache
				if cacheConfig.EnableInMemoryCache {
					cacheKey := generateCacheKey(c, cacheConfig)
					if cacheKey != "" {
						if cachedFile, found := fileCache.Get(cacheKey); found {
							if clientEtag := c.GetHeader("If-None-Match"); clientEtag != "" && clientEtag == cachedFile.ETag {
								c.Header("Cache-Control", "public, max-age=31536000") // 1 year for 304 responses
								c.Header("ETag", cachedFile.ETag)
								c.AbortWithStatus(http.StatusNotModified)
								return
							}

							// Set basic headers from cache
							c.Header("Content-Type", cachedFile.MimeType)
							c.Header("ETag", cachedFile.ETag)

							// Set cache control based on expiry time
							maxAge := int(time.Until(cachedFile.ExpiresAt).Seconds())
							if maxAge <= 0 {
								maxAge = 60 // Minimum 1 minute for almost expired resources
							}
							c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d", maxAge))

							// Add content disposition if needed
							if cachedFile.IsDownload {
								c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%v\"", filepath.Base(cachedFile.Path)))
							} else {
								c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%v\"", filepath.Base(cachedFile.Path)))
							}

							// Check if client accepts gzip and we have compressed data
							if cachedFile.GzipData != nil && len(cachedFile.GzipData) > 0 && strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
								c.Header("Content-Encoding", "gzip")
								c.Header("Vary", "Accept-Encoding")
								c.Data(http.StatusOK, cachedFile.MimeType, cachedFile.GzipData)
								return
							}

							// Serve uncompressed data
							c.Data(http.StatusOK, cachedFile.MimeType, cachedFile.Data)
							return
						}
					}
				}
			}

			inFields := make(map[string]interface{})

			inFields["template"] = templateName
			queryMap := make(map[string]interface{})
			inFields["query"] = queryMap

			for _, param := range c.Params {
				inFields[param.Key] = param.Value
			}

			queryParams := c.Request.URL.Query()

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
				_ = c.AbortWithError(500, errTxn)
				return
			}
			defer func() {
				_ = transaction1.Commit()
			}()

			var api2goRequestData = api2go.Request{
				PlainRequest: c.Request,
				QueryParams:  c.Request.URL.Query(),
				Pagination:   nil,
				Header:       c.Request.Header,
				Context:      nil,
			}

			if len(actionRequest.Action) > 0 && len(actionRequest.Type) > 0 {
				actionRequest.Attributes = inFields
				actionResponses, errAction := cruds["action"].HandleActionRequest(actionRequest, api2goRequestData, transaction1)
				if errAction != nil {
					_ = c.AbortWithError(500, errAction)
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
				_ = c.AbortWithError(500, err[0])
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
					c.Writer.Header().Set("ETag", etag)
				}

				// Set Last-Modified header for cache validation
				c.Writer.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
			}

			// Apply VaryByQueryParams if configured
			if cacheConfig != nil && cacheConfig.Enable && len(cacheConfig.VaryByQueryParams) > 0 {
				// Add Vary header for query parameters
				// This is a custom implementation as HTTP doesn't directly support varying by query params
				// We'll add a custom header to indicate which query params affect caching
				c.Writer.Header().Set("X-Vary-By-Query-Params", strings.Join(cacheConfig.VaryByQueryParams, ", "))
			}

			// Prepare response headers
			c.Writer.Header().Set("Content-Type", mimeType)
			for hKey, hValue := range headers {
				c.Writer.Header().Set(hKey, hValue)
			}

			// Set cache miss header for debugging
			c.Writer.Header().Set("X-Cache", "MISS")

			// Write status and flush headers
			c.Writer.WriteHeader(http.StatusOK)
			c.Writer.Flush()

			// Render the content
			fmt.Fprint(c.Writer, decodedContent)
			c.Writer.Flush()
			c.Abort()
			expiryTime := server.CalculateExpiry(mimeType, c.Request.URL.Path)

			// Store in cache if in-memory caching is enabled
			if cacheConfig != nil && cacheConfig.Enable && cacheConfig.EnableInMemoryCache {
				cacheKey := generateCacheKey(c, cacheConfig)
				// Check if client has fresh copy before we do anything else
				if clientEtag := c.GetHeader("If-None-Match"); clientEtag != "" && clientEtag == etag {
					c.Header("ETag", etag)
					c.AbortWithStatus(http.StatusNotModified)
					return
				}

				// Create cache entry
				data := []byte(decodedContent)
				newCachedFile := &server.CachedFile{
					Data:       data,
					ETag:       etag,
					Modtime:    time.Now().UTC(),
					MimeType:   mimeType,
					Size:       len(data),
					Path:       c.Request.URL.Path,
					IsDownload: false,
					ExpiresAt:  expiryTime,
				}

				// Pre-compress text files for better performance
				needsCompression := server.ShouldCompress(mimeType) && len(data) > server.CompressionThreshold
				if needsCompression {
					if compressedData, err := server.CompressData(data); err == nil {
						newCachedFile.GzipData = compressedData
					}
				}

				// Add to cache for future requests
				fileCache.Set(cacheKey, newCachedFile)

				// Set ETag header
				c.Header("ETag", etag)

				// Use compression if client accepts it and we have compressed data
				if newCachedFile.GzipData != nil && strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
					c.Header("Content-Encoding", "gzip")
					c.Header("Vary", "Accept-Encoding")
					c.Data(http.StatusOK, mimeType, newCachedFile.GzipData)
					return
				}

				// Serve uncompressed data
				c.Data(http.StatusOK, mimeType, data)
				return
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
