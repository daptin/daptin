# Security Analysis: server/websockets/websocket_server.go

**File:** `server/websockets/websocket_server.go`  
**Type:** WebSocket server implementation and client management  
**Lines of Code:** 143  

## Overview
This file implements the WebSocket server that manages client connections, handles connection lifecycle events, and integrates with the Gin web framework. It serves as the central coordination point for WebSocket operations in the Daptin system.

## Key Components

### WebSocketPayload struct
**Lines:** 11-14  
**Purpose:** Message structure for WebSocket communication  

### Server struct
**Lines:** 19-29  
**Purpose:** Main WebSocket server managing client connections and topics  

### Connection handler
**Lines:** 86-142  
**Purpose:** Main server loop handling client lifecycle events  

## Security Analysis

### 1. HIGH: Client ID Integer Overflow - HIGH RISK
**Severity:** HIGH  
**Lines:** 120, 127  
**Issue:** Client ID used as map key without bounds checking, vulnerable to integer overflow.

```go
s.clients[c.id] = c        // Line 120 - no bounds checking
delete(s.clients, c.id)    // Line 127 - relies on potentially overflowed ID
```

**Risk:**
- **Client collision** when ID counter overflows
- **Map corruption** from negative or invalid IDs
- **Client isolation failure** due to ID conflicts
- **Data integrity issues** in client management

### 2. HIGH: Unbounded Client Storage - HIGH RISK
**Severity:** HIGH  
**Lines:** 33, 120  
**Issue:** No limit on number of concurrent clients causing memory exhaustion.

```go
clients := make(map[int]*Client)  // No size limit
s.clients[c.id] = c              // Unlimited client addition
```

**Risk:**
- **Memory exhaustion** from unlimited client connections
- **Performance degradation** with large client maps
- **DoS attacks** through connection flooding
- **System instability** under high load

### 3. HIGH: Error Information Disclosure - HIGH RISK
**Severity:** HIGH  
**Lines:** 101, 136  
**Issue:** Detailed error messages exposed to clients and logs.

```go
_, _ = ws.Write([]byte(err.Error()))  // Exposes internal errors to client
log.Infof("[136] error: %s", err.Error())  // Logs detailed errors
```

**Risk:**
- **Information disclosure** through error messages
- **System information leakage** to unauthorized clients
- **Attack surface expansion** through error analysis
- **Debug information exposure** in production

### 4. MEDIUM: Insecure WebSocket Response - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 101-103  
**Issue:** Raw error message written to WebSocket without validation.

```go
_, _ = ws.Write([]byte(err.Error()))  // Raw error bytes
_ = ws.WriteClose(400)               // HTTP status code on WebSocket
```

**Risk:**
- **Protocol confusion** mixing HTTP and WebSocket responses
- **Client parsing errors** from unexpected response format
- **Information leakage** through raw error content
- **Poor error handling** affecting client experience

### 5. MEDIUM: Missing Input Validation - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 11-16  
**Issue:** WebSocketPayload and Message types lack validation constraints.

```go
type WebSocketPayload struct {
    Method  string  `json:"method"`      // No length or content validation
    Payload Message `json:"attributes"`  // No size limits
}
type Message map[string]interface{}      // Generic interface map
```

**Risk:**
- **DoS attacks** through oversized payloads
- **Memory exhaustion** from large message maps
- **Type confusion** in message processing
- **Injection attacks** through unvalidated content

### 6. MEDIUM: Race Condition in Client Management - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 120, 127  
**Issue:** Concurrent access to clients map without synchronization.

```go
// Multiple goroutines can access this simultaneously
s.clients[c.id] = c        // Add operation
delete(s.clients, c.id)    // Delete operation
```

**Risk:**
- **Data races** in client map operations
- **Map corruption** under concurrent access
- **Client state inconsistency** in high-concurrency scenarios
- **Potential crashes** from map corruption

### 7. MEDIUM: Broadcast Without Permission Check - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 74-78  
**Issue:** sendAll method broadcasts to all clients without permission validation.

```go
func (s *Server) sendAll(msg resource.EventMessage) {
    for _, c := range s.clients {
        c.Write(msg)  // No permission check for recipient
    }
}
```

**Risk:**
- **Unauthorized message delivery** to restricted clients
- **Information disclosure** through broadcast messages
- **Privacy violations** from cross-client message leakage
- **Compliance issues** with data protection regulations

### 8. LOW: Commented Authentication Code - LOW RISK
**Severity:** LOW  
**Lines:** 53-58  
**Issue:** Commented-out authentication code suggests incomplete security implementation.

```go
//sessionUser := auth.SessionUser{}
//token, _, ok := c.ws.Request().BasicAuth()
//token  := c.ws.Request().FormValue("token")
//if ok {
//    log.Printf("New web socket connection token: %v", token)
//}
```

**Risk:**
- **Development artifacts** indicating incomplete features
- **Security gaps** from disabled authentication
- **Code maintenance issues** from commented code
- **Potential security bypass** if intended features are missing

### 9. LOW: Insufficient Error Handling - LOW RISK
**Severity:** LOW  
**Lines:** 95, 101-102  
**Issue:** Error handling ignores some errors and uses non-standard patterns.

```go
s.errCh <- err              // Error sent to channel but may not be processed
_, _ = ws.Write(...)         // Ignores write errors
_ = ws.WriteClose(400)      // Ignores close errors
```

**Risk:**
- **Silent failures** from ignored errors
- **Resource leaks** from unhandled connection errors
- **Difficult debugging** from suppressed error information
- **Connection state inconsistency** from partial operations

## Potential Attack Vectors

### Denial of Service Attacks
1. **Connection Flooding:** Open unlimited WebSocket connections to exhaust memory
2. **ID Overflow:** Trigger integer overflow in client ID generation
3. **Large Payload DoS:** Send oversized messages to exhaust memory
4. **Channel Flooding:** Fill server channels to block operations

### Information Disclosure Attacks
1. **Error Mining:** Trigger various errors to extract system information
2. **Broadcast Interception:** Receive unauthorized messages through sendAll
3. **Client Enumeration:** Discover connected clients through ID patterns
4. **System State Discovery:** Analyze error messages for internal state

### Resource Exhaustion Attacks
1. **Memory Exhaustion:** Create unlimited client connections
2. **Channel Exhaustion:** Fill communication channels to block processing
3. **Map Corruption:** Exploit race conditions to corrupt client storage
4. **Connection Leaks:** Trigger connection leaks through error conditions

## Recommendations

### Immediate Actions
1. **Add Client Limits:** Implement maximum concurrent client limits
2. **Add Synchronization:** Protect client map with mutex for thread safety
3. **Sanitize Error Messages:** Remove sensitive information from error responses
4. **Add Input Validation:** Validate WebSocket message size and content

### Enhanced Security Implementation

```go
package websockets

import (
    "context"
    "fmt"
    "sync"
    "sync/atomic"
    "time"
    
    "github.com/buraksezer/olric"
    "github.com/daptin/daptin/server/resource"
    "github.com/gin-gonic/gin"
    log "github.com/sirupsen/logrus"
    "golang.org/x/net/websocket"
)

const (
    MaxClients = 10000
    MaxMessageSize = 64 * 1024 // 64KB
    MaxMethodLength = 100
    ServerTimeout = 5 * time.Minute
)

// WebSocketPayload represents a secure WebSocket message
type WebSocketPayload struct {
    Method  string  `json:"method" validate:"required,max=100"`
    Payload Message `json:"attributes" validate:"required"`
}

// Validate validates the WebSocket payload
func (w *WebSocketPayload) Validate() error {
    if len(w.Method) == 0 {
        return fmt.Errorf("method cannot be empty")
    }
    
    if len(w.Method) > MaxMethodLength {
        return fmt.Errorf("method too long: %d", len(w.Method))
    }
    
    // Validate method contains only safe characters
    for _, r := range w.Method {
        if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || 
             (r >= '0' && r <= '9') || r == '-' || r == '_') {
            return fmt.Errorf("method contains invalid characters")
        }
    }
    
    if w.Payload == nil {
        return fmt.Errorf("payload cannot be nil")
    }
    
    return w.Payload.Validate()
}

// Message represents a secure message map
type Message map[string]interface{}

// Validate validates the message content
func (m Message) Validate() error {
    if len(m) == 0 {
        return fmt.Errorf("message cannot be empty")
    }
    
    if len(m) > 100 {
        return fmt.Errorf("too many message fields: %d", len(m))
    }
    
    // Validate total message size
    totalSize := 0
    for key, value := range m {
        if len(key) > 100 {
            return fmt.Errorf("message key too long: %d", len(key))
        }
        
        if valueStr, ok := value.(string); ok {
            totalSize += len(valueStr)
            if totalSize > MaxMessageSize {
                return fmt.Errorf("message too large: %d bytes", totalSize)
            }
        }
    }
    
    return nil
}

// ServerStats provides server statistics
type ServerStats struct {
    ConnectedClients int       `json:"connected_clients"`
    TotalConnections int64     `json:"total_connections"`
    MessagesSent     int64     `json:"messages_sent"`
    MessagesReceived int64     `json:"messages_received"`
    Errors           int64     `json:"errors"`
    StartTime        time.Time `json:"start_time"`
    Uptime           string    `json:"uptime"`
}

// Server represents a secure WebSocket server
type Server struct {
    pattern   string
    clients   map[int64]*Client
    addCh     chan *Client
    delCh     chan *Client
    doneCh    chan bool
    errCh     chan error
    dtopicMap *map[string]*olric.PubSub
    olricDb   *olric.EmbeddedClient
    cruds     map[string]*resource.DbResource
    
    // Security and monitoring
    mutex            sync.RWMutex
    clientCount      int64
    totalConnections int64
    messagesSent     int64
    messagesReceived int64
    errors           int64
    startTime        time.Time
    maxClients       int
    isShuttingDown   bool
    ctx              context.Context
    cancel           context.CancelFunc
}

// NewServer creates a new secure WebSocket server
func NewServer(pattern string, dtopicMap *map[string]*olric.PubSub, cruds map[string]*resource.DbResource) *Server {
    if pattern == "" {
        pattern = "/ws"
    }
    
    if dtopicMap == nil {
        log.Fatal("dtopicMap cannot be nil")
    }
    
    if cruds == nil || cruds["world"] == nil {
        log.Fatal("cruds map must contain 'world' entry")
    }
    
    ctx, cancel := context.WithCancel(context.Background())
    
    return &Server{
        pattern:          pattern,
        clients:          make(map[int64]*Client),
        addCh:            make(chan *Client, 100),
        delCh:            make(chan *Client, 100),
        doneCh:           make(chan bool, 1),
        errCh:            make(chan error, 100),
        dtopicMap:        dtopicMap,
        olricDb:          cruds["world"].OlricDb,
        cruds:            cruds,
        startTime:        time.Now(),
        maxClients:       MaxClients,
        ctx:              ctx,
        cancel:           cancel,
    }
}

// Add safely adds a client to the server
func (s *Server) Add(c *Client) error {
    if c == nil {
        return fmt.Errorf("client cannot be nil")
    }
    
    s.mutex.RLock()
    isShuttingDown := s.isShuttingDown
    clientCount := s.clientCount
    s.mutex.RUnlock()
    
    if isShuttingDown {
        return fmt.Errorf("server is shutting down")
    }
    
    if clientCount >= int64(s.maxClients) {
        return fmt.Errorf("maximum client limit reached: %d", s.maxClients)
    }
    
    select {
    case s.addCh <- c:
        return nil
    case <-time.After(time.Second):
        return fmt.Errorf("server add channel blocked")
    case <-s.ctx.Done():
        return fmt.Errorf("server context cancelled")
    }
}

// Del safely removes a client from the server
func (s *Server) Del(c *Client) {
    if c == nil {
        return
    }
    
    select {
    case s.delCh <- c:
    case <-time.After(time.Second):
        log.Printf("Delete channel blocked for client %d", c.id)
    case <-s.ctx.Done():
        log.Printf("Server context cancelled during client deletion")
    }
}

// Done signals the server to shutdown
func (s *Server) Done() {
    s.mutex.Lock()
    s.isShuttingDown = true
    s.mutex.Unlock()
    
    select {
    case s.doneCh <- true:
    default:
        // Channel already has signal
    }
}

// Err safely reports an error
func (s *Server) Err(err error) {
    if err == nil {
        return
    }
    
    atomic.AddInt64(&s.errors, 1)
    
    // Sanitize error for logging
    sanitizedErr := fmt.Errorf("websocket error occurred")
    log.Printf("WebSocket error: %v", sanitizedErr)
    
    select {
    case s.errCh <- sanitizedErr:
    default:
        // Error channel full, drop error
        log.Printf("Error channel full, dropping error")
    }
}

// SendAll broadcasts a message to all authorized clients
func (s *Server) SendAll(msg resource.EventMessage) {
    s.mutex.RLock()
    clients := make([]*Client, 0, len(s.clients))
    for _, client := range s.clients {
        clients = append(clients, client)
    }
    s.mutex.RUnlock()
    
    // Send to clients outside of lock to prevent blocking
    for _, client := range clients {
        // Add permission check here
        if s.canReceiveMessage(client, msg) {
            if err := client.Write(msg); err != nil {
                log.Printf("Failed to send message to client %d: %v", client.id, err)
            } else {
                atomic.AddInt64(&s.messagesSent, 1)
            }
        }
    }
}

// canReceiveMessage checks if client can receive the message
func (s *Server) canReceiveMessage(client *Client, msg resource.EventMessage) bool {
    // Implement permission checking logic here
    // This is a placeholder - actual implementation would check user permissions
    return client != nil && client.user != nil
}

// GetStats returns server statistics
func (s *Server) GetStats() ServerStats {
    s.mutex.RLock()
    defer s.mutex.RUnlock()
    
    uptime := time.Since(s.startTime)
    
    return ServerStats{
        ConnectedClients: len(s.clients),
        TotalConnections: atomic.LoadInt64(&s.totalConnections),
        MessagesSent:     atomic.LoadInt64(&s.messagesSent),
        MessagesReceived: atomic.LoadInt64(&s.messagesReceived),
        Errors:          atomic.LoadInt64(&s.errors),
        StartTime:       s.startTime,
        Uptime:          uptime.String(),
    }
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(timeout time.Duration) error {
    log.Printf("Shutting down WebSocket server...")
    
    s.mutex.Lock()
    s.isShuttingDown = true
    s.mutex.Unlock()
    
    // Signal shutdown
    s.Done()
    
    // Wait for shutdown with timeout
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()
    
    done := make(chan bool, 1)
    go func() {
        // Close all client connections
        s.mutex.RLock()
        clients := make([]*Client, 0, len(s.clients))
        for _, client := range s.clients {
            clients = append(clients, client)
        }
        s.mutex.RUnlock()
        
        for _, client := range clients {
            client.Done()
        }
        
        done <- true
    }()
    
    select {
    case <-done:
        log.Printf("WebSocket server shutdown completed")
        return nil
    case <-ctx.Done():
        return fmt.Errorf("shutdown timeout exceeded")
    }
}

// Listen starts the WebSocket server with enhanced security
func (s *Server) Listen(router *gin.Engine) {
    log.Printf("Starting secure WebSocket server at %v", s.pattern)
    
    // Create connection handler with security measures
    onConnected := func(ws *websocket.Conn) {
        defer func() {
            if r := recover(); r != nil {
                log.Printf("Recovered from panic in WebSocket handler: %v", r)
            }
            
            if err := ws.Close(); err != nil {
                log.Printf("Error closing WebSocket connection: %v", err)
            }
        }()
        
        // Set connection timeouts
        ws.SetReadDeadline(time.Now().Add(30 * time.Second))
        ws.SetWriteDeadline(time.Now().Add(30 * time.Second))
        
        client, err := NewClient(ws, s)
        if err != nil {
            log.Printf("Failed to create client: %v", err)
            
            // Send sanitized error response
            errorResponse := []byte(`{"error": "Connection failed"}`)
            ws.Write(errorResponse)
            ws.WriteClose(websocket.StatusNormalClosure)
            return
        }
        
        if err := s.Add(client); err != nil {
            log.Printf("Failed to add client: %v", err)
            
            errorResponse := []byte(`{"error": "Server full"}`)
            ws.Write(errorResponse)
            ws.WriteClose(websocket.StatusNormalClosure)
            return
        }
        
        atomic.AddInt64(&s.totalConnections, 1)
        
        // Start client listener
        client.Listen()
    }
    
    wsHandler := websocket.Handler(onConnected)
    router.GET(s.pattern, func(ginContext *gin.Context) {
        wsHandler.ServeHTTP(ginContext.Writer, ginContext.Request)
    })
    
    log.Printf("WebSocket handler created at %s", s.pattern)
    
    // Main server loop
    for {
        select {
        case c := <-s.addCh:
            s.mutex.Lock()
            s.clients[c.id] = c
            s.clientCount = int64(len(s.clients))
            s.mutex.Unlock()
            
            log.Printf("Client %d connected, total clients: %d", c.id, s.clientCount)
            
        case c := <-s.delCh:
            s.mutex.Lock()
            delete(s.clients, c.id)
            s.clientCount = int64(len(s.clients))
            s.mutex.Unlock()
            
            log.Printf("Client %d disconnected, total clients: %d", c.id, s.clientCount)
            
        case err := <-s.errCh:
            // Errors are already sanitized in Err method
            log.Printf("Server error: %v", err)
            
        case <-s.doneCh:
            log.Printf("WebSocket server shutdown signal received")
            s.cancel()
            return
            
        case <-s.ctx.Done():
            log.Printf("WebSocket server context cancelled")
            return
        }
    }
}

// WebSocketConnectionHandler interface for secure message handling
type WebSocketConnectionHandler interface {
    MessageFromClient(message WebSocketPayload, client *Client) error
}
```

### Long-term Improvements
1. **Rate Limiting:** Implement per-client and global rate limiting
2. **Message Encryption:** Add end-to-end encryption for sensitive messages
3. **Authentication Integration:** Proper token-based authentication
4. **Connection Pooling:** Optimize resource usage with connection pooling
5. **Monitoring Integration:** Comprehensive metrics and alerting

## Edge Cases Identified

1. **Server Shutdown:** Graceful handling of server shutdown with active clients
2. **Memory Pressure:** Server operations under high memory pressure
3. **Network Interruptions:** Handling of network failures during operation
4. **Client ID Overflow:** Handling of integer overflow in client identification
5. **Channel Overflow:** Handling of full communication channels
6. **Concurrent Access:** Multiple goroutines accessing shared state
7. **Error Propagation:** Proper error handling across components
8. **Resource Cleanup:** Proper cleanup of resources on errors

## Security Best Practices Violations

1. **No client connection limits** allowing unlimited resource consumption
2. **Race conditions** in client map operations without synchronization
3. **Information disclosure** through detailed error messages
4. **Missing input validation** for WebSocket message content
5. **Insecure error handling** exposing internal state
6. **No rate limiting** for connection establishment or message processing
7. **Incomplete authentication** with commented-out code

## Positive Security Aspects

1. **Integration with authentication context** through request processing
2. **Channel-based architecture** providing separation of concerns
3. **Error reporting mechanism** for centralized error handling
4. **Resource cleanup** attempt in connection handler

## Critical Issues Summary

1. **Client ID Integer Overflow:** Client ID vulnerable to overflow causing collisions
2. **Unbounded Client Storage:** No limit on concurrent clients causing memory exhaustion
3. **Error Information Disclosure:** Detailed error messages exposed to clients
4. **Insecure WebSocket Response:** Raw error content written to WebSocket
5. **Missing Input Validation:** WebSocket payload lacks size and content validation
6. **Race Condition in Client Management:** Concurrent client map access without synchronization
7. **Broadcast Without Permission Check:** Messages sent to all clients without authorization
8. **Commented Authentication Code:** Incomplete security implementation
9. **Insufficient Error Handling:** Errors ignored or improperly handled

---
**Analysis Date:** 2025-01-27  
**Analyst:** Claude  
**Priority:** HIGH - WebSocket server with multiple high-severity vulnerabilities requiring security hardening