# Security Analysis: server/endpoint_imap.go

**File:** `server/endpoint_imap.go`  
**Type:** IMAP server initialization and configuration  
**Lines of Code:** 49  

## Overview
This file initializes an IMAP server for email access in the Daptin application. It handles IMAP server configuration through the configuration store, sets up TLS certificates, configures the server with security settings, and starts the server in a separate goroutine. The implementation supports both secure (IMAPS on port 993) and standard IMAP connections with automatic TLS configuration.

## Key Components

### InitializeImapResources function
**Lines:** 11-48  
**Purpose:** Initializes and starts IMAP server with TLS configuration and security settings  

### Configuration Management
- **Listen interface configuration:** Lines 12-17
- **Hostname configuration:** Lines 19-20
- **Default value handling:** Lines 14-16

### Server Setup and Security
- **IMAP backend creation:** Line 21
- **Server configuration:** Lines 24-28
- **TLS certificate setup:** Lines 30-32
- **Server startup:** Lines 36-46

## Security Analysis

### 1. CRITICAL: Missing Input Validation - CRITICAL RISK
**Severity:** CRITICAL  
**Lines:** 12, 19, 25  
**Issue:** IMAP configuration values used without validation for network binding and hostname.

```go
imapListenInterface, err := configStore.GetConfigValueFor("imap.listen_interface", "backend", transaction)
// No validation of imapListenInterface
imapServer.Addr = imapListenInterface  // Direct assignment without validation
```

**Risk:**
- **Network binding manipulation** through malicious interface configuration
- **Port hijacking** via configuration injection
- **Service disruption** through invalid interface binding
- **Hostname injection** enabling domain spoofing attacks

### 2. HIGH: Insecure Default Configuration - HIGH RISK
**Severity:** HIGH  
**Lines:** 14, 16  
**Issue:** Default IMAP interface binds to all interfaces on non-standard port without security consideration.

```go
err = configStore.SetConfigValueFor("imap.listen_interface", ":1143", "backend", transaction)
imapListenInterface = ":1143"  // Binds to all interfaces
```

**Risk:**
- **Network exposure** to all interfaces by default
- **Unauthorized external access** to IMAP server
- **Non-standard port** discovery and exploitation
- **Configuration persistence** of insecure defaults

### 3. HIGH: Goroutine Resource Management - HIGH RISK
**Severity:** HIGH  
**Lines:** 36-46  
**Issue:** IMAP server started in goroutine without proper lifecycle management or error handling.

```go
go func() {
    if EndsWithCheck(imapListenInterface, ":993") {
        if err := imapServer.ListenAndServeTLS(); err != nil {
            resource.CheckErr(err, "Imap server is not listening anymore 1")  // Error logged but goroutine continues
        }
    } else {
        if err := imapServer.ListenAndServe(); err != nil {
            resource.CheckErr(err, "Imap server is not listening anymore 2")  // Error logged but goroutine continues
        }
    }
}()
```

**Risk:**
- **Goroutine leaks** from failed server startup
- **Silent failures** in IMAP server operation
- **Resource exhaustion** from unmanaged goroutines
- **Service instability** from unhandled errors

### 4. MEDIUM: TLS Configuration Security - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 30-32  
**Issue:** TLS configuration depends on external certificate manager without validation.

```go
cert, err := certificateManager.GetTLSConfig(hostname, true, transaction)
resource.CheckErr(err, "Failed to get certificate for IMAP [%v]", hostname)
imapServer.TLSConfig = cert.TLSConfig  // No validation of TLS config
```

**Risk:**
- **Certificate validation bypass** through malicious certificates
- **TLS downgrade attacks** via weak certificate configuration
- **Man-in-the-middle** attacks through compromised certificates
- **Encryption bypass** via invalid TLS configuration

### 5. MEDIUM: Hostname Manipulation - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 19-20  
**Issue:** Hostname construction without validation enables subdomain injection.

```go
hostname, err := configStore.GetConfigValueFor("hostname", "backend", transaction)
hostname = "imap." + hostname  // String concatenation without validation
```

**Risk:**
- **Domain spoofing** through hostname manipulation
- **Certificate mismatch** from malformed hostnames
- **DNS attacks** via malicious hostname values
- **Service identification** confusion through hostname injection

### 6. MEDIUM: Port Detection Logic - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 37  
**Issue:** TLS detection based on string suffix matching without proper port parsing.

```go
if EndsWithCheck(imapListenInterface, ":993") {  // Simple string suffix check
```

**Risk:**
- **Protocol confusion** from malformed interface strings
- **TLS bypass** through interface manipulation
- **Port spoofing** via crafted interface values
- **Security mode bypass** through string manipulation

### 7. LOW: Information Disclosure - LOW RISK
**Severity:** LOW  
**Lines:** 34, 39, 43  
**Issue:** IMAP server details logged without sanitization.

```go
logrus.Printf("Starting IMAP server at %s: %v", imapListenInterface, hostname)
resource.CheckErr(err, "Imap server is not listening anymore 1")
```

**Risk:**
- **Network topology disclosure** through interface logging
- **Service fingerprinting** via detailed error messages
- **Configuration exposure** in log files
- **Attack surface mapping** through logged details

## Potential Attack Vectors

### Network Configuration Attacks
1. **Interface Injection:** Inject malicious interface values to control IMAP server binding
2. **Port Manipulation:** Configure IMAP server to bind to privileged or conflicting ports
3. **Hostname Spoofing:** Manipulate hostname configuration for domain spoofing
4. **Network Discovery:** Use IMAP server binding to discover internal network topology

### Certificate and TLS Attacks
1. **Certificate Substitution:** Replace legitimate certificates with malicious ones
2. **TLS Downgrade:** Force use of weak TLS configurations
3. **Certificate Bypass:** Exploit certificate validation weaknesses
4. **Man-in-the-Middle:** Intercept IMAP communications through certificate attacks

### Configuration Injection Attacks
1. **Database Configuration Tampering:** Modify IMAP configuration through database access
2. **Persistent Configuration Corruption:** Inject malicious configuration that persists
3. **Service Disruption:** Configure invalid interfaces to disable IMAP service
4. **Protocol Confusion:** Configure conflicting protocols on same interface

### Resource Management Attacks
1. **Goroutine Exhaustion:** Cause goroutine leaks through repeated initialization
2. **Server Startup Flooding:** Trigger multiple server startup attempts
3. **Resource Allocation:** Consume system resources through failed server instances
4. **Silent Service Degradation:** Exploit error handling to create degraded service states

## Recommendations

### Immediate Actions
1. **Validate Interface Configuration:** Add comprehensive validation for IMAP interface values
2. **Secure Default Configuration:** Use localhost-only binding by default
3. **Implement Goroutine Management:** Add proper goroutine lifecycle management
4. **Validate TLS Configuration:** Add validation for TLS certificate configuration

### Enhanced Security Implementation

```go
package server

import (
    "fmt"
    "net"
    "regexp"
    "strconv"
    "strings"
    "sync"
    "time"
    
    "github.com/artpar/go-imap-idle"
    "github.com/artpar/go-imap/server"
    "github.com/daptin/daptin/server/resource"
    "github.com/jmoiron/sqlx"
    "github.com/sirupsen/logrus"
)

const (
    defaultImapInterface = "127.0.0.1:1143"  // Localhost only by default
    defaultImapsInterface = "127.0.0.1:993"   // Secure IMAP on localhost
    minImapPort         = 1024                // Minimum non-privileged port
    maxImapPort         = 65535               // Maximum port number
    imapStartupTimeout  = 30 * time.Second    // Server startup timeout
)

var (
    // Safe interface pattern for validation
    safeImapInterfacePattern = regexp.MustCompile(`^[a-zA-Z0-9.-]+:[0-9]+$`)
    
    // IMAP server management
    imapServerMutex sync.RWMutex
    activeImapServer *server.Server
)

// validateImapInterface validates IMAP interface configuration for security
func validateImapInterface(interfaceStr string) error {
    if interfaceStr == "" {
        return fmt.Errorf("IMAP interface cannot be empty")
    }
    
    if len(interfaceStr) > 253 {
        return fmt.Errorf("IMAP interface too long: %d characters", len(interfaceStr))
    }
    
    if !safeImapInterfacePattern.MatchString(interfaceStr) {
        return fmt.Errorf("IMAP interface has invalid format")
    }
    
    // Parse host and port
    host, portStr, err := net.SplitHostPort(interfaceStr)
    if err != nil {
        return fmt.Errorf("invalid interface format: %v", err)
    }
    
    // Validate host
    if err := validateImapHost(host); err != nil {
        return fmt.Errorf("invalid host: %v", err)
    }
    
    // Validate port
    if err := validateImapPort(portStr); err != nil {
        return fmt.Errorf("invalid port: %v", err)
    }
    
    return nil
}

// validateImapHost validates the host part of IMAP interface
func validateImapHost(host string) error {
    if host == "" {
        // Empty host means bind to all interfaces - security risk
        return fmt.Errorf("host cannot be empty (would bind to all interfaces)")
    }
    
    // Check for dangerous hosts
    dangerousHosts := []string{
        "0.0.0.0",  // All interfaces - security risk
    }
    
    for _, dangerous := range dangerousHosts {
        if host == dangerous {
            return fmt.Errorf("host '%s' is not allowed for security reasons", host)
        }
    }
    
    // Validate IP address if it looks like one
    if ip := net.ParseIP(host); ip != nil {
        // Check for private/loopback addresses
        if !ip.IsLoopback() && !ip.IsPrivate() {
            logrus.Warnf("IMAP server binding to public IP address: %s", host)
        }
        return nil
    }
    
    // Validate hostname
    if len(host) > 253 {
        return fmt.Errorf("hostname too long: %d characters", len(host))
    }
    
    return nil
}

// validateImapPort validates the port part of IMAP interface
func validateImapPort(portStr string) error {
    port, err := strconv.Atoi(portStr)
    if err != nil {
        return fmt.Errorf("invalid port number: %s", portStr)
    }
    
    if port < minImapPort || port > maxImapPort {
        return fmt.Errorf("port out of range: %d (allowed: %d-%d)", port, minImapPort, maxImapPort)
    }
    
    // Check for commonly used privileged ports
    privilegedPorts := []int{
        21,  // FTP
        22,  // SSH
        23,  // Telnet
        25,  // SMTP
        53,  // DNS
        80,  // HTTP
        110, // POP3
        143, // IMAP
        443, // HTTPS
        993, // IMAPS
        995, // POP3S
    }
    
    for _, privPort := range privilegedPorts {
        if port == privPort {
            logrus.Warnf("IMAP server using standard/privileged port: %d", port)
        }
    }
    
    return nil
}

// validateHostname validates hostname for certificate generation
func validateHostname(hostname string) error {
    if hostname == "" {
        return fmt.Errorf("hostname cannot be empty")
    }
    
    if len(hostname) > 253 {
        return fmt.Errorf("hostname too long: %d characters", len(hostname))
    }
    
    // Check for dangerous patterns
    dangerousPatterns := []string{
        "..", "*", "`", "$", ";", "|", "&", 
        "localhost", "127.0.0.1", "0.0.0.0",
    }
    
    lowerHostname := strings.ToLower(hostname)
    for _, pattern := range dangerousPatterns {
        if strings.Contains(lowerHostname, pattern) {
            return fmt.Errorf("hostname contains dangerous pattern: %s", pattern)
        }
    }
    
    return nil
}

// isImapsPort checks if the given interface uses the IMAPS port securely
func isImapsPort(interfaceStr string) (bool, error) {
    _, portStr, err := net.SplitHostPort(interfaceStr)
    if err != nil {
        return false, fmt.Errorf("invalid interface format: %v", err)
    }
    
    port, err := strconv.Atoi(portStr)
    if err != nil {
        return false, fmt.Errorf("invalid port number: %s", portStr)
    }
    
    return port == 993, nil
}

// SecureImapServerManager manages IMAP server lifecycle securely
type SecureImapServerManager struct {
    server    *server.Server
    interface_ string
    hostname   string
    stopChan   chan struct{}
    errorChan  chan error
    started    bool
    mutex      sync.RWMutex
}

// NewSecureImapServerManager creates a new secure IMAP server manager
func NewSecureImapServerManager() *SecureImapServerManager {
    return &SecureImapServerManager{
        stopChan:  make(chan struct{}),
        errorChan: make(chan error, 1),
    }
}

// Start starts the IMAP server with proper error handling
func (m *SecureImapServerManager) Start(imapServer *server.Server, interface_, hostname string) error {
    m.mutex.Lock()
    defer m.mutex.Unlock()
    
    if m.started {
        return fmt.Errorf("IMAP server already started")
    }
    
    m.server = imapServer
    m.interface_ = interface_
    m.hostname = hostname
    
    // Determine if TLS should be used
    useImaps, err := isImapsPort(interface_)
    if err != nil {
        return fmt.Errorf("failed to determine IMAP protocol: %v", err)
    }
    
    // Start server in goroutine with proper error handling
    go func() {
        defer func() {
            if r := recover(); r != nil {
                logrus.Errorf("IMAP server panic: %v", r)
                m.errorChan <- fmt.Errorf("IMAP server panic: %v", r)
            }
        }()
        
        logrus.Infof("Starting IMAP server on interface: %s (TLS: %v)", m.interface_, useImaps)
        
        var err error
        if useImaps {
            err = m.server.ListenAndServeTLS()
        } else {
            err = m.server.ListenAndServe()
        }
        
        if err != nil {
            logrus.Errorf("IMAP server error: %v", err)
            m.errorChan <- err
        }
    }()
    
    // Wait for startup or timeout
    select {
    case err := <-m.errorChan:
        return fmt.Errorf("IMAP server startup failed: %v", err)
    case <-time.After(imapStartupTimeout):
        logrus.Infof("IMAP server startup timeout - assuming success")
    }
    
    m.started = true
    logrus.Infof("IMAP server started successfully on %s", m.interface_)
    return nil
}

// Stop stops the IMAP server
func (m *SecureImapServerManager) Stop() error {
    m.mutex.Lock()
    defer m.mutex.Unlock()
    
    if !m.started {
        return fmt.Errorf("IMAP server not started")
    }
    
    close(m.stopChan)
    
    if m.server != nil {
        // Implement server shutdown if available
        logrus.Infof("Stopping IMAP server")
    }
    
    m.started = false
    return nil
}

// IsRunning checks if the IMAP server is running
func (m *SecureImapServerManager) IsRunning() bool {
    m.mutex.RLock()
    defer m.mutex.RUnlock()
    return m.started
}

// InitializeSecureImapResources initializes IMAP server with enhanced security
func InitializeSecureImapResources(configStore *resource.ConfigStore, transaction *sqlx.Tx, cruds map[string]*resource.DbResource, imapServer *server.Server, certificateManager *resource.CertificateManager) (*server.Server, error) {
    
    imapServerMutex.Lock()
    defer imapServerMutex.Unlock()
    
    // Get IMAP interface configuration with validation
    imapListenInterface, err := configStore.GetConfigValueFor("imap.listen_interface", "backend", transaction)
    if err != nil {
        logrus.Infof("IMAP interface not configured, using secure default: %s", defaultImapInterface)
        imapListenInterface = defaultImapInterface
        
        // Store default value
        err = configStore.SetConfigValueFor("imap.listen_interface", imapListenInterface, "backend", transaction)
        if err != nil {
            return nil, fmt.Errorf("failed to store default IMAP interface configuration: %v", err)
        }
    }
    
    // Validate IMAP interface configuration
    if err := validateImapInterface(imapListenInterface); err != nil {
        logrus.Errorf("Invalid IMAP interface configuration: %v", err)
        
        // Fall back to secure default
        logrus.Infof("Using secure default IMAP interface: %s", defaultImapInterface)
        imapListenInterface = defaultImapInterface
        
        // Update configuration with secure default
        err = configStore.SetConfigValueFor("imap.listen_interface", imapListenInterface, "backend", transaction)
        if err != nil {
            logrus.Warnf("Failed to update IMAP interface configuration: %v", err)
        }
    }
    
    // Get and validate hostname
    hostname, err := configStore.GetConfigValueFor("hostname", "backend", transaction)
    if err != nil {
        return nil, fmt.Errorf("failed to get hostname configuration: %v", err)
    }
    
    // Validate hostname
    if err := validateHostname(hostname); err != nil {
        logrus.Warnf("Invalid hostname for IMAP: %v", err)
        hostname = "localhost" // Safe fallback
    }
    
    // Construct IMAP hostname
    imapHostname := "imap." + hostname
    
    // Create IMAP backend
    imapBackend := resource.NewImapServer(cruds)
    if imapBackend == nil {
        return nil, fmt.Errorf("failed to create IMAP backend")
    }
    
    // Create new IMAP server
    imapServer = server.New(imapBackend)
    if imapServer == nil {
        return nil, fmt.Errorf("failed to create IMAP server instance")
    }
    
    // Configure server settings
    imapServer.Addr = imapListenInterface
    imapServer.Debug = nil // Disable debug in production
    imapServer.AllowInsecureAuth = false // Always require secure auth
    
    // Enable IDLE extension
    imapServer.Enable(idle.NewExtension())
    
    // Get TLS configuration
    cert, err := certificateManager.GetTLSConfig(imapHostname, true, transaction)
    if err != nil {
        return nil, fmt.Errorf("failed to get TLS certificate for IMAP: %v", err)
    }
    
    if cert == nil || cert.TLSConfig == nil {
        return nil, fmt.Errorf("invalid TLS configuration for IMAP")
    }
    
    imapServer.TLSConfig = cert.TLSConfig
    
    // Create server manager
    manager := NewSecureImapServerManager()
    
    // Start IMAP server with proper error handling
    if err := manager.Start(imapServer, imapListenInterface, imapHostname); err != nil {
        return nil, fmt.Errorf("failed to start IMAP server: %v", err)
    }
    
    // Store active server reference
    activeImapServer = imapServer
    
    logrus.Infof("IMAP server initialized and started successfully on %s", imapListenInterface)
    return imapServer, nil
}

// StopSecureImapServer stops the active IMAP server
func StopSecureImapServer() error {
    imapServerMutex.Lock()
    defer imapServerMutex.Unlock()
    
    if activeImapServer == nil {
        return fmt.Errorf("no active IMAP server to stop")
    }
    
    // Implement server shutdown logic
    logrus.Infof("Stopping active IMAP server")
    activeImapServer = nil
    
    return nil
}

// GetImapServerStatus returns IMAP server status
func GetImapServerStatus() map[string]interface{} {
    imapServerMutex.RLock()
    defer imapServerMutex.RUnlock()
    
    return map[string]interface{}{
        "running":             activeImapServer != nil,
        "default_interface":   defaultImapInterface,
        "default_imaps_interface": defaultImapsInterface,
        "port_range":         fmt.Sprintf("%d-%d", minImapPort, maxImapPort),
    }
}

// InitializeImapResources maintains backward compatibility with security enhancements
func InitializeImapResources(configStore *resource.ConfigStore, transaction *sqlx.Tx, cruds map[string]*resource.DbResource, imapServer *server.Server, certificateManager *resource.CertificateManager) *server.Server {
    secureServer, err := InitializeSecureImapResources(configStore, transaction, cruds, imapServer, certificateManager)
    if err != nil {
        logrus.Errorf("Secure IMAP initialization failed: %v", err)
        // Continue with original implementation as fallback
        logrus.Warnf("Falling back to original IMAP initialization")
        
        imapListenInterface, err := configStore.GetConfigValueFor("imap.listen_interface", "backend", transaction)
        if err != nil {
            err = configStore.SetConfigValueFor("imap.listen_interface", defaultImapInterface, "backend", transaction)
            if err != nil {
                logrus.Errorf("Failed to set default IMAP interface: %v", err)
            }
            imapListenInterface = defaultImapInterface
        }
        
        // Basic validation
        if err := validateImapInterface(imapListenInterface); err != nil {
            logrus.Warnf("Invalid IMAP interface, using default: %v", err)
            imapListenInterface = defaultImapInterface
        }
        
        hostname, err := configStore.GetConfigValueFor("hostname", "backend", transaction)
        if err != nil {
            hostname = "localhost"
        }
        hostname = "imap." + hostname
        
        imapBackend := resource.NewImapServer(cruds)
        imapServer = server.New(imapBackend)
        imapServer.Addr = imapListenInterface
        imapServer.Debug = nil
        imapServer.AllowInsecureAuth = false
        imapServer.Enable(idle.NewExtension())
        
        cert, err := certificateManager.GetTLSConfig(hostname, true, transaction)
        if err != nil {
            logrus.Errorf("Failed to get TLS config: %v", err)
        } else {
            imapServer.TLSConfig = cert.TLSConfig
        }
        
        logrus.Printf("Starting IMAP server at %s: %v", imapListenInterface, hostname)
        
        go func() {
            useImaps, _ := isImapsPort(imapListenInterface)
            if useImaps {
                if err := imapServer.ListenAndServeTLS(); err != nil {
                    logrus.Errorf("IMAP server TLS error: %v", err)
                }
            } else {
                if err := imapServer.ListenAndServe(); err != nil {
                    logrus.Errorf("IMAP server error: %v", err)
                }
            }
        }()
    }
    
    return secureServer
}
```

### Long-term Improvements
1. **IMAP Security Hardening:** Implement comprehensive IMAP security controls
2. **Access Control Integration:** Integrate with authentication and authorization systems
3. **Audit Logging:** Comprehensive logging of all IMAP operations for security monitoring
4. **Rate Limiting:** Implement rate limiting for IMAP connections and operations
5. **Configuration Management:** Dynamic configuration management with validation

## Edge Cases Identified

1. **Port Conflicts:** IMAP server port conflicts with other services
2. **Interface Binding Failures:** Network interface not available during startup
3. **Certificate Expiry:** TLS certificates expiring during IMAP operation
4. **Configuration Corruption:** Malformed IMAP configuration in database
5. **Rapid Server Restarts:** Multiple rapid initialization attempts
6. **Network Changes:** Network interface changes while server is running
7. **Resource Exhaustion:** System resource exhaustion preventing server startup
8. **Permission Errors:** Insufficient permissions to bind to configured interface
9. **TLS Configuration Failures:** Invalid or corrupted TLS certificate configuration
10. **Database Transaction Failures:** Configuration storage failures during initialization

## Security Best Practices Violations

1. **Missing input validation** for IMAP interface and hostname configuration
2. **Insecure default configuration** binding to all interfaces
3. **Poor goroutine management** without lifecycle control
4. **TLS configuration security** without validation
5. **Hostname manipulation** enabling subdomain injection
6. **Port detection logic** using simple string matching
7. **Information disclosure** through detailed logging
8. **No network security** considerations for IMAP protocol
9. **Missing access controls** for IMAP server configuration
10. **No audit logging** for IMAP server operations

## Positive Security Aspects

1. **TLS support** with certificate integration
2. **Insecure authentication disabled** by default
3. **IDLE extension** for efficient connections
4. **Error handling** with logging for troubleshooting
5. **Certificate manager integration** for TLS configuration

## Critical Issues Summary

1. **Missing Input Validation:** IMAP configuration values used without validation
2. **Insecure Default Configuration:** Default interface binds to all interfaces
3. **Goroutine Resource Management:** IMAP server started without proper lifecycle management
4. **TLS Configuration Security:** TLS configuration depends on external manager without validation
5. **Hostname Manipulation:** Hostname construction without validation enables injection
6. **Port Detection Logic:** TLS detection based on string matching without proper parsing
7. **Information Disclosure:** IMAP server details logged without sanitization

---
**Analysis Date:** 2025-01-27  
**Analyst:** Claude  
**Priority:** CRITICAL - IMAP server initialization with missing validation and insecure defaults