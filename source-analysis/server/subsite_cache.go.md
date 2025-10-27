# Security Analysis: server/subsite_cache.go

**File:** `server/subsite_cache.go`  
**Lines of Code:** 465  
**Primary Function:** Distributed caching system for subsite files using Olric with compression and TTL management

## Summary

This file implements a comprehensive caching system for subsite files using Olric distributed cache. It provides binary serialization/deserialization, content compression, TTL management, cache metrics, and periodic cache cleanup. The system includes race condition protection and configurable cache parameters for performance optimization.

## Security Issues Found

### 🟡 HIGH Issues

#### 1. **Missing Input Validation in Binary Deserialization** (Lines 84-157)
```go
func (sce *SubsiteCacheEntry) UnmarshalBinary(data []byte) error {
    // Read ETag length and ETag
    var etagLen int32
    if err := binary.Read(buf, binary.LittleEndian, &etagLen); err != nil {
        return fmt.Errorf("failed to read ETag length: %v", err)
    }
    etagBytes := make([]byte, etagLen)
```
**Risk:** Malformed binary data could cause excessive memory allocation
- No bounds checking on length fields before allocation
- Malicious data could specify extremely large lengths
- Could lead to memory exhaustion attacks
- Integer overflow potential with large length values
**Impact:** High - Memory exhaustion and potential DoS
**Remediation:** Add reasonable bounds checking for all length fields

#### 2. **Unsafe File Path Storage in Cache** (Lines 24, 250-268)
```go
FilePath     string    // Store the actual file path for checking modifications
func isFileModified(filePath string, cacheEntry *SubsiteCacheEntry) bool {
    fileInfo, err := os.Stat(filePath)
```
**Risk:** File paths stored in distributed cache could be manipulated
- File paths exposed in cache entries across distributed nodes
- No validation of file paths before os.Stat() calls
- Potential for path traversal if cache entries are manipulated
- Cache entries could reference files outside intended directories
**Impact:** High - Potential file system access and information disclosure
**Remediation:** Validate and sanitize file paths, avoid storing full paths in cache

#### 3. **Information Disclosure Through Error Messages** (Lines 90, 94, 101, 105, etc.)
```go
return fmt.Errorf("failed to read ETag length: %v", err)
return fmt.Errorf("failed to read ETag: %v", err)
return fmt.Errorf("failed to read Content length: %v", err)
```
**Risk:** Detailed error messages could expose internal system information
- Serialization errors reveal internal data structures
- Cache operation errors expose distributed cache topology
- File system errors expose directory structures
**Impact:** High - System information disclosure for reconnaissance
**Remediation:** Use generic error messages for external interfaces

### 🟠 MEDIUM Issues

#### 4. **Race Condition in Cache Initialization** (Lines 286-311)
```go
func InitSubsiteCache(client *olric.EmbeddedClient) error {
    subsiteCacheMutex.Lock()
    defer subsiteCacheMutex.Unlock()
    
    if subsiteCacheInitialized {
        return nil
    }
```
**Risk:** Race condition between check and initialization
- Multiple goroutines could bypass initialization check
- Global state modification without atomic operations
- Potential for partial initialization states
**Impact:** Medium - Cache initialization inconsistencies
**Remediation:** Use atomic operations or once.Do for initialization

#### 5. **Unbounded Memory Growth in Metrics** (Lines 185-192)
```go
var (
    cacheHits         int64
    cacheMisses       int64
    cacheBypassed     int64
    cacheRejected     int64
    cacheAdded        int64
)
```
**Risk:** Metrics counters can grow indefinitely
- No rollover or reset mechanism for metrics
- Long-running services could experience integer overflow
- Memory usage grows over time with metric collection
**Impact:** Medium - Memory exhaustion in long-running services
**Remediation:** Implement metric rollover or use atomic counters with bounds

#### 6. **Weak Cache Key Generation** (Lines 244-247)
```go
func getSubsiteCacheKey(host, path string) string {
    return host + "::" + path
}
```
**Risk:** Simple string concatenation for cache keys
- No validation or sanitization of host and path parameters
- Potential for cache key collisions with malicious inputs
- Host header manipulation could affect cache behavior
**Impact:** Medium - Cache pollution and potential key collisions
**Remediation:** Use cryptographic hash for cache key generation

#### 7. **File System Race Condition** (Lines 250-268)
```go
func isFileModified(filePath string, cacheEntry *SubsiteCacheEntry) bool {
    fileInfo, err := os.Stat(filePath)
    if err != nil {
        return true  // If we can't stat the file, consider it modified
    }
```
**Risk:** TOCTOU race condition between file stat and content serving
- File could be modified between stat check and content delivery
- Cached content might not match current file state
- No file locking or atomic operations
**Impact:** Medium - Stale content delivery and cache inconsistency
**Remediation:** Implement file locking or atomic content validation

### 🔵 LOW Issues

#### 8. **Global Variable Usage** (Lines 176-182)
```go
var SubsiteCache olric.DMap
var olricClient *olric.EmbeddedClient
var subsiteCacheInitialized bool
```
**Risk:** Global state management issues
- Thread safety depends on external synchronization
- Difficult to test and mock global state
- Single point of failure for cache operations
**Impact:** Low - Code maintainability and testing issues
**Remediation:** Use dependency injection instead of global variables

#### 9. **Magic Numbers in Configuration** (Lines 169-172)
```go
DefaultTTL:   time.Minute * 30, // Default to 30 minutes
MaxEntrySize: 100 * 1024,       // 100KB max entry size
```
**Risk:** Hardcoded configuration values
- Fixed limits may not suit all deployment environments
- No runtime configuration capability
- Potential for suboptimal performance
**Impact:** Low - Configuration inflexibility
**Remediation:** Make configuration values externally configurable

#### 10. **Potential Goroutine Leak** (Lines 448-464)
```go
func logCacheMetricsPeriodically() {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()
    
    for range ticker.C {
        // Metrics logging
    }
}
```
**Risk:** Long-running goroutine without cancellation mechanism
- Goroutine runs indefinitely without context cancellation
- No way to stop metrics logging during shutdown
- Resource leak in test environments
**Impact:** Low - Resource leak in specific scenarios
**Remediation:** Add context cancellation for graceful shutdown

## Code Quality Issues

1. **Error Handling**: Inconsistent error handling patterns across functions
2. **Memory Management**: Potential for unbounded memory growth in several areas
3. **Thread Safety**: Mixed use of mutexes and atomic operations
4. **Configuration**: Hardcoded values reduce deployment flexibility
5. **Resource Management**: Missing cleanup mechanisms for background goroutines

## Recommendations

### Immediate Actions Required

1. **Input Validation**: Add bounds checking for all binary deserialization operations
2. **Path Security**: Validate and sanitize file paths before file system operations
3. **Error Handling**: Use generic error messages to prevent information disclosure
4. **Memory Bounds**: Implement limits for metrics and cache entry allocations

### Security Improvements

1. **Cache Security**: Implement secure cache key generation using cryptographic hashes
2. **Access Control**: Add validation for cache operations and file access
3. **Information Security**: Limit exposure of internal paths and system information
4. **Race Prevention**: Use atomic operations for shared state management

### Code Quality Enhancements

1. **Configuration**: Make all cache parameters externally configurable
2. **Resource Management**: Add proper cleanup and cancellation mechanisms
3. **Testing**: Improve testability by reducing global state dependencies
4. **Documentation**: Add security considerations and usage guidelines

## Attack Vectors

1. **Memory Exhaustion**: Send malformed binary data with large length fields
2. **Cache Pollution**: Manipulate host headers to create cache key collisions
3. **Information Gathering**: Use error messages to gather system information
4. **Path Traversal**: Manipulate cached file paths to access unauthorized files
5. **Resource Exhaustion**: Trigger unbounded metric growth over time

## Impact Assessment

- **Confidentiality**: MEDIUM - File paths and system information exposed
- **Integrity**: MEDIUM - Race conditions could affect cache consistency
- **Availability**: HIGH - Memory exhaustion attacks could cause DoS
- **Authentication**: N/A - Cache operations don't involve authentication
- **Authorization**: MEDIUM - File path validation affects access control

This cache implementation provides good performance features but has several security vulnerabilities primarily around input validation, memory management, and information disclosure. The distributed nature of the cache adds complexity to security considerations.

## Technical Notes

The caching system includes:
1. Binary serialization/deserialization for distributed storage
2. Content compression for text-based files
3. TTL management with automatic expiration
4. Cache metrics and performance monitoring
5. Race condition protection for cache loading
6. File modification detection for cache invalidation

The main security concerns revolve around the handling of untrusted input in binary deserialization and the exposure of file system paths in a distributed environment.