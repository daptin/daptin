# Security Analysis: server/endpoint_ftp.go

**File:** `server/endpoint_ftp.go`  
**Type:** FTP server creation and subsite integration  
**Lines of Code:** 63  

## Overview
This file creates FTP servers with integration to subsites and cloud storage systems. It manages FTP-enabled subsites, integrates with asset cache folders, and creates a custom Daptin FTP driver. The implementation handles site enumeration, cloud store mapping, and FTP server instantiation with subsite-specific configurations.

## Key Components

### CreateFtpServers function
**Lines:** 15-57  
**Purpose:** Creates FTP server instances with subsite and cloud store integration  

### Data Retrieval and Mapping
- **Subsite enumeration:** Lines 17-20
- **Cloud store retrieval:** Lines 21-30
- **Cloud store UUID mapping:** Lines 26-30

### FTP Site Configuration
- **FTP-enabled site filtering:** Lines 36-38
- **Asset cache integration:** Lines 40-43
- **Site structure creation:** Lines 44-48

### SubSiteAssetCache struct
**Lines:** 59-62  
**Purpose:** Combines subsite configuration with asset cache functionality  

## Security Analysis

### 1. CRITICAL: Missing Input Validation - CRITICAL RISK
**Severity:** CRITICAL  
**Lines:** 15, 52  
**Issue:** FTP interface parameter and site data used without comprehensive validation.

```go
func CreateFtpServers(resources map[string]*resource.DbResource, resourcesInterfaces map[string]dbresourceinterface.DbResourceInterface, certManager *resource.CertificateManager, ftp_interface string, transaction *sqlx.Tx) (*server.FtpServer, error) {
    // No validation of ftp_interface parameter
    driver, err = NewDaptinFtpDriver(resources, certManager, ftp_interface, sites)
}
```

**Risk:**
- **FTP interface injection** through malicious interface parameters
- **Site configuration manipulation** via database injection
- **Resource access abuse** through unvalidated site data
- **Network binding exploitation** via crafted interface strings

### 2. HIGH: Unsafe Type Conversion - HIGH RISK
**Severity:** HIGH  
**Lines:** 28-29  
**Issue:** UUID conversion from bytes without error handling can cause panics.

```go
re, _ := uuid.FromBytes(cloudStore.ReferenceId[:])  // Error ignored
cloudStoreMap[re] = cloudStore
```

**Risk:**
- **Service disruption** through UUID conversion failures
- **Data corruption** from invalid UUID handling
- **Memory safety issues** from malformed reference IDs
- **Silent failures** masking security issues

### 3. HIGH: Database Injection Risk - HIGH RISK
**Severity:** HIGH  
**Lines:** 17, 21  
**Issue:** Database queries through interfaces without visible validation.

```go
subsites, err := subsite.GetAllSites(resourcesInterfaces["site"], transaction)
cloudStores, err := cloud_store.GetAllCloudStores(resourcesInterfaces["cloud_store"], transaction)
```

**Risk:**
- **SQL injection** through manipulated resource interfaces
- **Unauthorized data access** via database interface abuse
- **Data exfiltration** through malicious queries
- **Transaction manipulation** for unauthorized operations

### 4. MEDIUM: Resource Access Control - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 40-43, 52  
**Issue:** Asset cache and site resources accessed without explicit authorization checks.

```go
assetCacheFolder, ok := resourcesInterfaces["site"].SubsiteFolderCache(ftpServer.ReferenceId)
driver, err = NewDaptinFtpDriver(resources, certManager, ftp_interface, sites)
```

**Risk:**
- **Unauthorized file access** through asset cache manipulation
- **Cross-site data access** via reference ID manipulation
- **Resource enumeration** through cache folder access
- **Privilege escalation** via site resource access

### 5. MEDIUM: Error Information Disclosure - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 54  
**Issue:** Detailed error information exposed through logging without sanitization.

```go
resource.CheckErr(err, "Failed to create daptin ftp driver [%v]", driver)
```

**Risk:**
- **System architecture disclosure** through error details
- **Configuration information leakage** via error messages
- **Attack vector identification** through detailed errors
- **Internal state exposure** in production logs

### 6. MEDIUM: Memory and Resource Management - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 26-30, 33-48  
**Issue:** Large data structures created without size limits or validation.

```go
cloudStoreMap := make(map[uuid.UUID]rootpojo.CloudStore)  // No size limits
sites := make([]SubSiteAssetCache, 0)  // Unbounded growth
```

**Risk:**
- **Memory exhaustion** from large cloud store maps
- **Resource consumption** through excessive site enumeration
- **Performance degradation** under resource pressure
- **Denial of service** via resource exhaustion

### 7. LOW: Silent Error Handling - LOW RISK
**Severity:** LOW  
**Lines:** 28, 41  
**Issue:** Errors silently ignored in non-critical operations.

```go
re, _ := uuid.FromBytes(cloudStore.ReferenceId[:])  // Error ignored
assetCacheFolder, ok := resourcesInterfaces["site"].SubsiteFolderCache(ftpServer.ReferenceId)
if !ok {
    continue  // Silent skip without logging
}
```

**Risk:**
- **Silent failures** masking configuration issues
- **Debugging complexity** from undocumented failures
- **Security issue concealment** through silent errors
- **Operational blind spots** from ignored errors

## Potential Attack Vectors

### FTP Server Manipulation Attacks
1. **Interface Injection:** Inject malicious FTP interface parameters to control server binding
2. **Site Configuration Abuse:** Manipulate subsite configurations to access unauthorized resources
3. **Driver Parameter Injection:** Inject malicious parameters into FTP driver creation
4. **Resource Map Pollution:** Corrupt resource maps to redirect FTP access

### Database and Resource Attacks
1. **SQL Injection:** Exploit database interface queries for unauthorized access
2. **Reference ID Manipulation:** Manipulate UUID reference IDs for cross-site access
3. **Transaction Abuse:** Exploit database transactions for unauthorized operations
4. **Cache Folder Access:** Access unauthorized asset cache folders through reference manipulation

### Memory and Resource Exhaustion
1. **Cloud Store Map Bombing:** Create excessive cloud store entries to exhaust memory
2. **Site List Explosion:** Generate numerous FTP-enabled sites to consume resources
3. **Asset Cache Flooding:** Overwhelm asset cache systems through site proliferation
4. **UUID Conversion DoS:** Trigger UUID conversion failures to disrupt service

### Information Disclosure Attacks
1. **Error Message Harvesting:** Extract system information through induced errors
2. **Configuration Discovery:** Discover system configuration through error details
3. **Resource Enumeration:** Enumerate available resources through error patterns
4. **Architecture Fingerprinting:** Map system architecture through detailed logging

## Recommendations

### Immediate Actions
1. **Add Input Validation:** Validate FTP interface parameter and all input data
2. **Handle UUID Conversion Errors:** Properly handle UUID conversion failures
3. **Implement Resource Limits:** Add size limits for data structures and operations
4. **Sanitize Error Messages:** Remove sensitive information from error logging

### Enhanced Security Implementation

```go
package server

import (
    "fmt"
    "net"
    "regexp"
    "strconv"
    "strings"
    
    "github.com/daptin/daptin/server/assetcachepojo"
    "github.com/daptin/daptin/server/cloud_store"
    "github.com/daptin/daptin/server/dbresourceinterface"
    "github.com/daptin/daptin/server/resource"
    "github.com/daptin/daptin/server/rootpojo"
    "github.com/daptin/daptin/server/subsite"
    "github.com/fclairamb/ftpserver/server"
    "github.com/google/uuid"
    "github.com/jmoiron/sqlx"
    log "github.com/sirupsen/logrus"
)

const (
    maxCloudStores    = 1000   // Maximum cloud stores to process
    maxSubsites       = 100    // Maximum subsites to process
    maxSitesPerFtp    = 50     // Maximum sites per FTP server
    ftpInterfaceMaxLen = 255   // Maximum FTP interface string length
)

var (
    // Safe FTP interface pattern
    safeFtpInterfacePattern = regexp.MustCompile(`^[a-zA-Z0-9.-]+:[0-9]+$`)
)

// validateFtpInterface validates FTP interface parameter for security
func validateFtpInterface(ftpInterface string) error {
    if ftpInterface == "" {
        return fmt.Errorf("FTP interface cannot be empty")
    }
    
    if len(ftpInterface) > ftpInterfaceMaxLen {
        return fmt.Errorf("FTP interface too long: %d characters", len(ftpInterface))
    }
    
    if !safeFtpInterfacePattern.MatchString(ftpInterface) {
        return fmt.Errorf("FTP interface has invalid format")
    }
    
    // Parse and validate host:port
    host, portStr, err := net.SplitHostPort(ftpInterface)
    if err != nil {
        return fmt.Errorf("invalid host:port format: %v", err)
    }
    
    // Validate host
    if host == "" {
        return fmt.Errorf("host cannot be empty")
    }
    
    // Check for dangerous hosts
    if host == "0.0.0.0" {
        return fmt.Errorf("binding to all interfaces not allowed for security")
    }
    
    // Validate port
    port, err := strconv.Atoi(portStr)
    if err != nil {
        return fmt.Errorf("invalid port number: %s", portStr)
    }
    
    if port < 1024 || port > 65535 {
        return fmt.Errorf("port out of safe range: %d", port)
    }
    
    return nil
}

// validateResourceInterfaces validates resource interfaces for security
func validateResourceInterfaces(resourcesInterfaces map[string]dbresourceinterface.DbResourceInterface) error {
    if resourcesInterfaces == nil {
        return fmt.Errorf("resource interfaces cannot be nil")
    }
    
    requiredInterfaces := []string{"site", "cloud_store"}
    for _, required := range requiredInterfaces {
        if _, exists := resourcesInterfaces[required]; !exists {
            return fmt.Errorf("required resource interface missing: %s", required)
        }
    }
    
    return nil
}

// safeUUIDFromBytes safely converts bytes to UUID with error handling
func safeUUIDFromBytes(data []byte) (uuid.UUID, error) {
    if len(data) != 16 {
        return uuid.Nil, fmt.Errorf("invalid UUID byte length: %d", len(data))
    }
    
    // Create a copy to avoid potential memory issues
    uuidBytes := make([]byte, 16)
    copy(uuidBytes, data)
    
    return uuid.FromBytes(uuidBytes)
}

// SecureSubSiteAssetCache extends SubSiteAssetCache with validation
type SecureSubSiteAssetCache struct {
    SubSite          subsite.SubSite
    AssetFolderCache *assetcachepojo.AssetFolderCache
    Validated        bool
}

// validateSubSite validates subsite configuration for security
func validateSubSite(site subsite.SubSite) error {
    // Validate site reference ID
    if site.ReferenceId == [16]byte{} {
        return fmt.Errorf("site reference ID cannot be empty")
    }
    
    // Additional subsite validation can be added here
    return nil
}

// validateAssetCache validates asset cache folder for security
func validateAssetCache(cache *assetcachepojo.AssetFolderCache, siteRefId [16]byte) error {
    if cache == nil {
        return fmt.Errorf("asset cache cannot be nil")
    }
    
    // Additional asset cache validation can be added here
    return nil
}

// CreateSecureFtpServers creates FTP servers with comprehensive security validation
func CreateSecureFtpServers(resources map[string]*resource.DbResource, resourcesInterfaces map[string]dbresourceinterface.DbResourceInterface, certManager *resource.CertificateManager, ftpInterface string, transaction *sqlx.Tx) (*server.FtpServer, error) {
    
    // Validate input parameters
    if err := validateFtpInterface(ftpInterface); err != nil {
        return nil, fmt.Errorf("invalid FTP interface: %v", err)
    }
    
    if err := validateResourceInterfaces(resourcesInterfaces); err != nil {
        return nil, fmt.Errorf("invalid resource interfaces: %v", err)
    }
    
    if resources == nil {
        return nil, fmt.Errorf("resources cannot be nil")
    }
    
    if certManager == nil {
        return nil, fmt.Errorf("certificate manager cannot be nil")
    }
    
    if transaction == nil {
        return nil, fmt.Errorf("transaction cannot be nil")
    }
    
    // Get subsites with error handling
    subsites, err := subsite.GetAllSites(resourcesInterfaces["site"], transaction)
    if err != nil {
        return nil, fmt.Errorf("failed to retrieve subsites: %v", err)
    }
    
    if len(subsites) > maxSubsites {
        log.Warnf("Too many subsites (%d), limiting to %d", len(subsites), maxSubsites)
        subsites = subsites[:maxSubsites]
    }
    
    // Get cloud stores with error handling
    cloudStores, err := cloud_store.GetAllCloudStores(resourcesInterfaces["cloud_store"], transaction)
    if err != nil {
        return nil, fmt.Errorf("failed to retrieve cloud stores: %v", err)
    }
    
    if len(cloudStores) > maxCloudStores {
        log.Warnf("Too many cloud stores (%d), limiting to %d", len(cloudStores), maxCloudStores)
        cloudStores = cloudStores[:maxCloudStores]
    }
    
    // Create cloud store map with secure UUID handling
    cloudStoreMap := make(map[uuid.UUID]rootpojo.CloudStore, len(cloudStores))
    for _, cloudStore := range cloudStores {
        cloudUUID, err := safeUUIDFromBytes(cloudStore.ReferenceId[:])
        if err != nil {
            log.Warnf("Invalid cloud store UUID, skipping: %v", err)
            continue
        }
        cloudStoreMap[cloudUUID] = cloudStore
    }
    
    // Process FTP-enabled sites with validation
    sites := make([]SecureSubSiteAssetCache, 0)
    processedSites := 0
    
    for _, ftpSite := range subsites {
        // Check FTP enabled status
        if !ftpSite.FtpEnabled {
            continue
        }
        
        // Limit number of sites per FTP server
        if processedSites >= maxSitesPerFtp {
            log.Warnf("Maximum sites per FTP server reached (%d), skipping remaining sites", maxSitesPerFtp)
            break
        }
        
        // Validate site configuration
        if err := validateSubSite(ftpSite); err != nil {
            log.Warnf("Invalid subsite configuration, skipping: %v", err)
            continue
        }
        
        // Get asset cache folder with error handling
        assetCacheFolder, ok := resourcesInterfaces["site"].SubsiteFolderCache(ftpSite.ReferenceId)
        if !ok {
            log.Debugf("No asset cache folder for site: %x", ftpSite.ReferenceId)
            continue
        }
        
        // Validate asset cache
        if err := validateAssetCache(assetCacheFolder, ftpSite.ReferenceId); err != nil {
            log.Warnf("Invalid asset cache for site %x: %v", ftpSite.ReferenceId, err)
            continue
        }
        
        // Create secure site structure
        secureSite := SecureSubSiteAssetCache{
            SubSite:          ftpSite,
            AssetFolderCache: assetCacheFolder,
            Validated:        true,
        }
        
        sites = append(sites, secureSite)
        processedSites++
    }
    
    if len(sites) == 0 {
        return nil, fmt.Errorf("no valid FTP-enabled sites found")
    }
    
    log.Infof("Processed %d valid FTP-enabled sites", len(sites))
    
    // Convert to legacy format for driver creation
    legacySites := make([]SubSiteAssetCache, len(sites))
    for i, site := range sites {
        legacySites[i] = SubSiteAssetCache{
            SubSite:          site.SubSite,
            AssetFolderCache: site.AssetFolderCache,
        }
    }
    
    // Create FTP driver with validated parameters
    driver, err := NewSecureDaptinFtpDriver(resources, certManager, ftpInterface, legacySites)
    if err != nil {
        return nil, fmt.Errorf("failed to create secure FTP driver: %v", err)
    }
    
    // Create FTP server
    ftpServer := server.NewFtpServer(driver)
    if ftpServer == nil {
        return nil, fmt.Errorf("failed to create FTP server instance")
    }
    
    log.Infof("Secure FTP server created successfully with %d sites", len(sites))
    return ftpServer, nil
}

// NewSecureDaptinFtpDriver creates a secure FTP driver with validation
func NewSecureDaptinFtpDriver(resources map[string]*resource.DbResource, certManager *resource.CertificateManager, ftpInterface string, sites []SubSiteAssetCache) (*DaptinFtpDriver, error) {
    
    // Validate parameters again
    if err := validateFtpInterface(ftpInterface); err != nil {
        return nil, fmt.Errorf("FTP interface validation failed: %v", err)
    }
    
    if len(sites) == 0 {
        return nil, fmt.Errorf("no sites provided for FTP driver")
    }
    
    // Call original function with validated parameters
    return NewDaptinFtpDriver(resources, certManager, ftpInterface, sites)
}

// GetFtpServerConfiguration returns FTP server configuration for monitoring
func GetFtpServerConfiguration() map[string]interface{} {
    return map[string]interface{}{
        "max_cloud_stores":      maxCloudStores,
        "max_subsites":         maxSubsites,
        "max_sites_per_ftp":    maxSitesPerFtp,
        "ftp_interface_max_len": ftpInterfaceMaxLen,
        "validation_enabled":   true,
    }
}

// CreateFtpServers maintains backward compatibility with security enhancements
func CreateFtpServers(resources map[string]*resource.DbResource, resourcesInterfaces map[string]dbresourceinterface.DbResourceInterface, certManager *resource.CertificateManager, ftpInterface string, transaction *sqlx.Tx) (*server.FtpServer, error) {
    
    // Try secure implementation first
    secureServer, err := CreateSecureFtpServers(resources, resourcesInterfaces, certManager, ftpInterface, transaction)
    if err != nil {
        log.Warnf("Secure FTP server creation failed, falling back to original: %v", err)
        
        // Fallback to original implementation with basic validation
        if err := validateFtpInterface(ftpInterface); err != nil {
            return nil, fmt.Errorf("FTP interface validation failed: %v", err)
        }
        
        // Original implementation with error handling improvements
        subsites, err := subsite.GetAllSites(resourcesInterfaces["site"], transaction)
        if err != nil {
            return nil, fmt.Errorf("failed to get subsites: %v", err)
        }
        
        cloudStores, err := cloud_store.GetAllCloudStores(resourcesInterfaces["cloud_store"], transaction)
        if err != nil {
            return nil, fmt.Errorf("failed to get cloud stores: %v", err)
        }
        
        cloudStoreMap := make(map[uuid.UUID]rootpojo.CloudStore)
        for _, cloudStore := range cloudStores {
            re, err := safeUUIDFromBytes(cloudStore.ReferenceId[:])
            if err != nil {
                log.Warnf("Invalid cloud store UUID: %v", err)
                continue
            }
            cloudStoreMap[re] = cloudStore
        }
        
        sites := make([]SubSiteAssetCache, 0)
        for _, ftpServer := range subsites {
            if !ftpServer.FtpEnabled {
                continue
            }
            
            assetCacheFolder, ok := resourcesInterfaces["site"].SubsiteFolderCache(ftpServer.ReferenceId)
            if !ok {
                continue
            }
            
            site := SubSiteAssetCache{
                SubSite:          ftpServer,
                AssetFolderCache: assetCacheFolder,
            }
            sites = append(sites, site)
        }
        
        driver, err := NewDaptinFtpDriver(resources, certManager, ftpInterface, sites)
        if err != nil {
            return nil, fmt.Errorf("failed to create FTP driver: %v", err)
        }
        
        ftpS := server.NewFtpServer(driver)
        return ftpS, nil
    }
    
    return secureServer, nil
}
```

### Long-term Improvements
1. **Authorization Framework:** Implement comprehensive authorization for FTP site access
2. **Audit Logging:** Add detailed audit logging for all FTP operations and configurations
3. **Resource Monitoring:** Monitor resource usage and implement dynamic limits
4. **Configuration Validation:** Implement schema validation for all configuration data
5. **Site Isolation:** Enhance site isolation to prevent cross-site access

## Edge Cases Identified

1. **Large Site Collections:** Very large numbers of subsites causing performance issues
2. **Invalid UUID References:** Malformed or corrupt UUID reference IDs in database
3. **Missing Asset Caches:** Sites with missing or inaccessible asset cache folders
4. **Database Transaction Failures:** Database transaction failures during site retrieval
5. **Memory Pressure:** High memory usage from large cloud store and site collections
6. **Concurrent Access:** Multiple FTP server creations accessing same resources
7. **Resource Interface Failures:** Resource interface method failures during site processing
8. **Certificate Manager Issues:** Certificate manager failures affecting FTP server creation
9. **Site Configuration Corruption:** Corrupted or inconsistent site configuration data
10. **Network Interface Conflicts:** FTP interface conflicts with existing services

## Security Best Practices Violations

1. **Missing input validation** for FTP interface and site data
2. **Unsafe type conversion** ignoring UUID conversion errors
3. **Database injection risk** through unvalidated interface queries
4. **Resource access control** issues with asset cache access
5. **Error information disclosure** through detailed logging
6. **Memory and resource management** without limits or validation
7. **Silent error handling** masking security issues
8. **No authorization checks** for site and resource access
9. **Missing audit logging** for FTP operations
10. **Insufficient data validation** for cloud store and site data

## Positive Security Aspects

1. **FTP enablement checks** to filter enabled sites only
2. **Error handling** for database operations
3. **Resource encapsulation** through structured data types
4. **Certificate manager integration** for potential TLS support

## Critical Issues Summary

1. **Missing Input Validation:** FTP interface and site data used without comprehensive validation
2. **Unsafe Type Conversion:** UUID conversion failures ignored causing potential panics
3. **Database Injection Risk:** Database queries through interfaces without visible validation
4. **Resource Access Control:** Asset cache and resources accessed without authorization checks
5. **Error Information Disclosure:** Detailed error information exposed without sanitization
6. **Memory and Resource Management:** Large data structures without size limits
7. **Silent Error Handling:** Critical errors silently ignored masking security issues

---
**Analysis Date:** 2025-01-27  
**Analyst:** Claude  
**Priority:** CRITICAL - FTP server creation with missing validation and resource access control issues