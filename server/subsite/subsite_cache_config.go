package subsite

import (
	"encoding/json"
	"time"
)

// CacheConfig provides flexible control over caching behavior for subsite endpoints
type CacheConfig struct {
	// Enable determines if caching is enabled for this endpoint
	Enable bool `json:"enable"`

	// MaxAge specifies the maximum time in seconds that a response can be cached
	// Corresponds to Cache-Control: max-age directive
	MaxAge int `json:"max_age"`

	// Revalidate determines if the client should revalidate cached responses
	// When true, adds Cache-Control: must-revalidate
	Revalidate bool `json:"revalidate"`

	// NoCache indicates content must be revalidated before serving from cache
	// Adds Cache-Control: no-cache when true
	NoCache bool `json:"no_cache"`

	// NoStore prevents caching of response entirely
	// Adds Cache-Control: no-store when true
	NoStore bool `json:"no_store"`

	// Private indicates response is intended for a single user
	// Adds Cache-Control: private when true, otherwise public is used
	Private bool `json:"private"`

	// VaryByHeaders specifies which request headers should be considered when caching
	// Populates the Vary header
	VaryByHeaders []string `json:"vary_by_headers"`

	// VaryByQueryParams specifies which query parameters should be considered when caching
	VaryByQueryParams []string `json:"vary_by_query_params"`

	// VaryByPath determines if the full request path should be considered when caching
	VaryByPath bool `json:"vary_by_path"`

	// StaleWhileRevalidate allows serving stale content while fetching a fresh version
	// Corresponds to Cache-Control: stale-while-revalidate directive
	StaleWhileRevalidate int `json:"stale_while_revalidate"`

	// CustomHeaders allows setting additional cache-related headers
	CustomHeaders map[string]string `json:"custom_headers"`

	// ExpiresAt sets an absolute expiration time
	// Populates the Expires header
	ExpiresAt *time.Time `json:"expires_at,omitempty"`

	// ETag strategy: "weak", "strong", or "none"
	ETagStrategy string `json:"etag_strategy"`

	// CacheKeyPrefix allows for custom cache key prefixing
	CacheKeyPrefix string `json:"cache_key_prefix"`

	// --- In-Memory Cache Controls ---

	// EnableInMemoryCache determines if server-side in-memory caching is enabled
	EnableInMemoryCache bool `json:"enable_in_memory_cache"`

	// InMemoryCacheTTL specifies the time-to-live in seconds for in-memory cache entries
	// After this time, entries will be considered stale and regenerated
	InMemoryCacheTTL int `json:"in_memory_cache_ttl"`

	// InMemoryCacheMaxSize specifies the maximum number of entries to keep in the in-memory cache
	// When exceeded, least recently used entries will be evicted
	InMemoryCacheMaxSize int `json:"in_memory_cache_max_size"`

	// InMemoryCacheStrategy controls how the cache behaves: "lru" (least recently used) or "lfu" (least frequently used)
	InMemoryCacheStrategy string `json:"in_memory_cache_strategy"`

	// InMemoryCacheCompression enables compression of cached content to reduce memory usage
	InMemoryCacheCompression bool `json:"in_memory_cache_compression"`
}

// GetCacheConfig parses the cache configuration from the provided interface
func GetCacheConfig(cacheConfigInterface interface{}) (*CacheConfig, error) {
	// Default cache configuration with sensible defaults
	cacheConfig := CacheConfig{
		// HTTP cache control defaults
		Enable:       false,
		MaxAge:       0,
		Revalidate:   true,
		NoCache:      false,
		NoStore:      false,
		Private:      false,
		VaryByPath:   true,
		ETagStrategy: "weak",

		// In-memory cache defaults
		EnableInMemoryCache:      false,
		InMemoryCacheTTL:         300,   // 5 minutes default TTL
		InMemoryCacheMaxSize:     100,   // Default to 100 entries
		InMemoryCacheStrategy:    "lru", // Default to LRU strategy
		InMemoryCacheCompression: false,
	}

	if cacheConfigInterface == nil {
		cacheConfigInterface = "{}"
	}

	actionReqStr := cacheConfigInterface.(string)
	if len(actionReqStr) == 0 {
		actionReqStr = "{}"
	}

	err := json.Unmarshal([]byte(actionReqStr), &cacheConfig)
	if err != nil {
		return nil, err
	}

	return &cacheConfig, nil
}

/*
# CacheConfig Documentation

## Overview
The CacheConfig structure provides comprehensive control over how responses are cached,
both on the client-side (browser) and server-side (in-memory). It allows fine-grained
control over caching behavior, enabling performance optimization while ensuring content
freshness.

## Structure Fields

### HTTP Cache Control

- `Enable` (bool): Master switch that enables/disables all caching functionality.
  When false, no cache headers will be sent and in-memory caching is disabled.

- `MaxAge` (int): Specifies how long (in seconds) the response should be considered fresh.
  This sets the Cache-Control: max-age directive. A value of 0 means the response should
  not be cached without revalidation.

- `Revalidate` (bool): When true, adds Cache-Control: must-revalidate directive,
  forcing clients to check with the server before using a cached response.

- `NoCache` (bool): When true, adds Cache-Control: no-cache directive, indicating
  that the response can be stored but must be validated before use.

- `NoStore` (bool): When true, adds Cache-Control: no-store directive, preventing
  any storage of the response. This is useful for sensitive data.

- `Private` (bool): When true, adds Cache-Control: private directive, indicating
  the response is intended for a single user and should not be stored in shared caches.
  When false, Cache-Control: public is used.

- `VaryByHeaders` ([]string): List of HTTP headers that should be considered when
  caching. This populates the Vary header. Common values include "Accept-Encoding",
  "User-Agent", or "Authorization".

- `VaryByQueryParams` ([]string): List of query parameters that affect caching.
  This is implemented via a custom X-Vary-By-Query-Params header.

- `VaryByPath` (bool): When true, the full request path is considered for caching.
  This is generally recommended to be true.

- `StaleWhileRevalidate` (int): Time in seconds during which a stale response can be
  served while a fresh one is fetched in the background. This implements the
  stale-while-revalidate directive.

- `CustomHeaders` (map[string]string): Additional cache-related headers to include
  in the response.

- `ExpiresAt` (*time.Time): Sets an absolute expiration time for the response.
  This populates the Expires header. If nil, the Expires header is derived from MaxAge.

- `ETagStrategy` (string): Controls ETag generation. Options are:
  - "weak": Generates a weak ETag (prefixed with W/)
  - "strong": Generates a strong ETag
  - "none": Disables ETag generation

- `CacheKeyPrefix` (string): Prefix added to cache keys. Useful for namespacing
  or versioning cache entries.

### In-Memory Cache Control

- `EnableInMemoryCache` (bool): Enables server-side in-memory caching of responses.
  This can significantly improve performance for frequently accessed content.

- `InMemoryCacheTTL` (int): Time-to-live in seconds for in-memory cache entries.
  After this time, entries are considered stale and will be regenerated.

- `InMemoryCacheMaxSize` (int): Maximum number of entries to keep in the in-memory cache.
  When exceeded, entries will be evicted based on the configured strategy.

- `InMemoryCacheStrategy` (string): Controls how entries are evicted when the cache is full:
  - "lru": Least Recently Used - removes entries that haven't been accessed for the longest time
  - "lfu": Least Frequently Used - removes entries that have been accessed the least number of times

- `InMemoryCacheCompression` (bool): When true, compresses cached content to reduce
  memory usage. This adds some CPU overhead but can be beneficial for large responses.

## Usage Examples

Here are five common configurations for different scenarios:

### 1. Static Content Caching
Optimal for static assets like images, CSS, and JavaScript files that rarely change.

```json
{
  "enable": true,
  "max_age": 86400,
  "revalidate": false,
  "private": false,
  "etag_strategy": "strong",
  "enable_in_memory_cache": true,
  "in_memory_cache_ttl": 3600,
  "in_memory_cache_max_size": 500
}
```

### 2. Dynamic Content with Short TTL
Suitable for frequently updated content that can be cached for short periods.

```json
{
  "enable": true,
  "max_age": 60,
  "revalidate": true,
  "stale_while_revalidate": 30,
  "etag_strategy": "weak",
  "enable_in_memory_cache": true,
  "in_memory_cache_ttl": 120,
  "vary_by_query_params": ["sort", "filter"]
}
```

### 3. Personalized Content
For content that varies by user but can still benefit from caching.

```json
{
  "enable": true,
  "max_age": 300,
  "private": true,
  "vary_by_headers": ["Authorization"],
  "etag_strategy": "weak",
  "enable_in_memory_cache": true,
  "in_memory_cache_strategy": "lfu"
}
```

### 4. API Responses
Optimized for API endpoints with validation but minimal caching.

```json
{
  "enable": true,
  "max_age": 0,
  "no_cache": true,
  "revalidate": true,
  "etag_strategy": "strong",
  "vary_by_headers": ["Accept", "Accept-Encoding"],
  "vary_by_query_params": ["page", "limit"],
  "enable_in_memory_cache": false
}
```

### 5. Sensitive Content (No Caching)
For sensitive data that should never be cached.

```json
{
  "enable": true,
  "no_store": true,
  "private": true,
  "etag_strategy": "none",
  "enable_in_memory_cache": false
}
```

## Best Practices

1. **Start Conservative**: Begin with shorter cache times and increase as you gain confidence.

2. **Use ETags**: ETags provide validation without requiring full content regeneration.

3. **Consider Vary Headers**: Properly configure VaryByHeaders to prevent serving incorrect
   cached content to different clients.

4. **Monitor Cache Hit Rates**: Use the X-Cache header to track cache performance.

5. **Balance Memory Usage**: For in-memory caching, set appropriate TTL and MaxSize values
   based on your server's memory constraints.

6. **Version Your Cache**: When deploying updates, consider using CacheKeyPrefix to version
   your cache and avoid serving stale content.

7. **Security Considerations**: Use no-store for sensitive data and private for
   user-specific content.
*/
