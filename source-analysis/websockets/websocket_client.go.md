# Security Analysis: server/websockets/websocket_client.go

**File:** `server/websockets/websocket_client.go`  
**Type:** WebSocket client connection management  
**Lines of Code:** 140  

## Overview
This file implements WebSocket client management including connection lifecycle, message reading/writing, and integration with the connection handler. It manages individual client connections with channel-based communication and user authentication.

## Key Components

### Client struct
**Lines:** 18-26  
**Purpose:** Represents individual WebSocket client connection with user context  

### NewClient function
**Lines:** 29-64  
**Purpose:** Creates new client with authentication and connection setup  

### Message handling methods
**Lines:** 85-139  
**Purpose:** Listen for read/write operations through channels  

## Security Analysis

### 1. CRITICAL: Type Assertion Vulnerability - CRITICAL RISK
**Severity:** CRITICAL  
**Lines:** 54  
**Issue:** Unsafe type assertion without validation that can panic the application.

```go
u := ws.Request().Context().Value("user")
if u == nil {
    return nil, errors.New("{\"message\": \"unauthorized\"}")
}
user := u.(*auth.SessionUser)  // Unsafe type assertion - can panic
```

**Risk:**
- **Application panic** if context contains wrong type for "user" key
- **Service disruption** from type assertion failures
- **DoS attack vector** through crafted authentication context
- **Runtime crashes** affecting WebSocket server

### 2. HIGH: Global State Race Condition - HIGH RISK
**Severity:** HIGH  
**Lines:** 16, 46  
**Issue:** Global maxId variable accessed without synchronization causing race conditions.

```go
var maxId = 0  // Global variable

func NewClient(ws *websocket.Conn, server *Server) (*Client, error) {
    maxId++    // Race condition - not thread-safe
    // ...
}
```

**Risk:**
- **ID collision** from concurrent client creation
- **Data races** in multi-threaded environment
- **Client identification issues** from duplicate IDs
- **Memory corruption** potential from race conditions

### 3. HIGH: Channel Resource Leak - HIGH RISK
**Severity:** HIGH  
**Lines:** 47, 48, 104-106, 119-121  
**Issue:** Channels not properly closed on client disconnection causing resource leaks.

```go
ch := make(chan resource.EventMessage, channelBufSize)
doneCh := make(chan bool)

// In listenWrite
case <-c.doneCh:
    c.server.Del(c)
    c.doneCh <- true  // Sends but doesn't close channel
    return

// In listenRead  
case <-c.doneCh:
    c.server.Del(c)
    c.doneCh <- true  // Sends but doesn't close channel
    return
```

**Risk:**
- **Memory leaks** from unclosed channels
- **Goroutine leaks** from blocked channel operations
- **Resource exhaustion** under high connection turnover
- **Performance degradation** from accumulated resources

### 4. HIGH: Error Information Disclosure - HIGH RISK
**Severity:** HIGH  
**Lines:** 52, 131  
**Issue:** Detailed error messages exposed to clients and logs.

```go
return nil, errors.New("{\"message\": \"unauthorized\"}")  // JSON in error message
c.server.Err(err)  // Exposes internal errors
```

**Risk:**
- **Information disclosure** through detailed error messages
- **System information leakage** in error responses
- **Attack surface expansion** through error message analysis
- **Debug information exposure** in production

### 5. MEDIUM: Panic in Client Creation - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 32, 36  
**Issue:** Function panics instead of returning errors for invalid inputs.

```go
if ws == nil {
    panic("ws cannot be nil")  // Panic instead of error
}
if server == nil {
    panic("server cannot be nil")  // Panic instead of error
}
```

**Risk:**
- **Application crashes** from invalid function calls
- **Service disruption** from panics in client creation
- **Poor error handling** making debugging difficult
- **Unpredictable behavior** under error conditions

### 6. MEDIUM: Channel Blocking Issues - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 71-77, 96-101  
**Issue:** Channel operations can block or drop messages without proper handling.

```go
func (c *Client) Write(msg resource.EventMessage) {
    select {
    case c.ch <- msg:
    default:
        c.server.Del(c)  // Deletes client on channel full
        err := fmt.Errorf("client %d is disconnected.", c.id)
        c.server.Err(err)
    }
}
```

**Risk:**
- **Message loss** when channels are full
- **Client disconnection** due to temporary blocking
- **Poor user experience** from dropped messages
- **Debugging difficulty** from silent message drops

### 7. MEDIUM: JSON Deserialization Security - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 98, 127  
**Issue:** JSON serialization/deserialization without validation or size limits.

```go
err := websocket.JSON.Send(c.ws, msg)        // No validation of msg content
err := websocket.JSON.Receive(c.ws, &msg)    // No size limits or validation
```

**Risk:**
- **DoS attacks** through large JSON payloads
- **Memory exhaustion** from oversized messages
- **Parser vulnerabilities** exploitation
- **Injection attacks** through malformed JSON

### 8. LOW: Missing Context Cancellation - LOW RISK
**Severity:** LOW  
**Lines:** 85-139  
**Issue:** Long-running goroutines without context cancellation support.

```go
func (c *Client) Listen() {
    go c.listenWrite()  // No context for cancellation
    c.listenRead()      // No timeout or cancellation
}
```

**Risk:**
- **Resource leaks** from long-running goroutines
- **Graceful shutdown issues** without cancellation
- **Difficult testing** without timeout control
- **Resource management complexity** in high-load scenarios

## Potential Attack Vectors

### Authentication Bypass Attacks
1. **Type Confusion:** Send invalid type in authentication context to cause panic
2. **Context Manipulation:** Modify request context to bypass authentication
3. **User Impersonation:** Exploit ID collision from race conditions

### Denial of Service Attacks
1. **Panic DoS:** Trigger panics through invalid client creation parameters
2. **Channel DoS:** Fill client channels to cause disconnections
3. **JSON DoS:** Send oversized JSON messages to exhaust memory
4. **Connection Flooding:** Create many connections to exhaust resources

### Resource Exhaustion Attacks
1. **Memory Exhaustion:** Create clients without proper cleanup
2. **Goroutine Exhaustion:** Trigger channel leaks to accumulate goroutines
3. **ID Exhaustion:** Exploit race conditions to exhaust ID space

## Recommendations

### Immediate Actions
1. **Fix Type Assertion:** Add proper type validation for user context
2. **Add Synchronization:** Protect global maxId with mutex or atomic operations
3. **Implement Channel Cleanup:** Properly close channels on client disconnection
4. **Sanitize Error Messages:** Remove sensitive information from error responses

### Enhanced Security Implementation

```go
package websockets

import (
    "context"
    "errors"
    "fmt"
    "sync"
    "sync/atomic"
    "time"
    
    "github.com/daptin/daptin/server/auth"
    "github.com/daptin/daptin/server/resource"
    "github.com/go-redis/redis/v8"
    log "github.com/sirupsen/logrus"
    "golang.org/x/net/websocket"
    "io"
)

const (
    channelBufSize = 100
    maxJSONSize = 64 * 1024 // 64KB
    clientTimeout = 30 * time.Minute
    maxClients = 10000
)

var (
    clientIdCounter int64
    activeClients   int64
    clientMutex     sync.RWMutex
)

// Client represents a secure WebSocket client connection
type Client struct {
    id                         int64
    ws                         *websocket.Conn
    server                     *Server
    ch                         chan resource.EventMessage
    doneCh                     chan bool
    user                       *auth.SessionUser
    webSocketConnectionHandler WebSocketConnectionHandlerImpl
    
    // Security and resource management
    ctx                        context.Context
    cancel                     context.CancelFunc
    lastActivity               time.Time
    isActive                   bool
    mutex                      sync.RWMutex
}

// ClientStats provides client statistics for monitoring
type ClientStats struct {
    ID              int64     `json:"id"`
    UserID          string    `json:"user_id"`
    ConnectedAt     time.Time `json:"connected_at"`
    LastActivity    time.Time `json:"last_activity"`
    MessagesSent    int64     `json:"messages_sent"`
    MessagesReceived int64    `json:"messages_received"`
    IsActive        bool      `json:"is_active"`
}

// validateUser safely validates user from context
func validateUser(ws *websocket.Conn) (*auth.SessionUser, error) {
    if ws == nil {
        return nil, fmt.Errorf("websocket connection is nil")
    }
    
    if ws.Request() == nil {
        return nil, fmt.Errorf("websocket request is nil")
    }
    
    ctx := ws.Request().Context()
    if ctx == nil {
        return nil, fmt.Errorf("request context is nil")
    }
    
    userValue := ctx.Value("user")
    if userValue == nil {
        return nil, fmt.Errorf("user not found in context")
    }
    
    user, ok := userValue.(*auth.SessionUser)
    if !ok {
        return nil, fmt.Errorf("invalid user type in context: %T", userValue)
    }
    
    if user == nil {
        return nil, fmt.Errorf("user is nil")
    }
    
    // Validate user fields
    if len(user.UserReferenceId) == 0 {
        return nil, fmt.Errorf("user reference ID is empty")
    }
    
    return user, nil
}

// NewClient creates a new secure WebSocket client
func NewClient(ws *websocket.Conn, server *Server) (*Client, error) {
    // Input validation
    if ws == nil {
        return nil, fmt.Errorf("websocket connection cannot be nil")
    }
    
    if server == nil {
        return nil, fmt.Errorf("server cannot be nil")
    }
    
    // Check client limit
    currentClients := atomic.LoadInt64(&activeClients)
    if currentClients >= maxClients {
        return nil, fmt.Errorf("maximum client limit reached: %d", maxClients)
    }
    
    // Validate user authentication
    user, err := validateUser(ws)
    if err != nil {
        log.Printf("Client authentication failed: %v", err)
        return nil, fmt.Errorf("authentication failed")
    }
    
    // Generate unique client ID
    clientId := atomic.AddInt64(&clientIdCounter, 1)
    
    // Create context with timeout
    ctx, cancel := context.WithTimeout(context.Background(), clientTimeout)
    
    // Create channels
    ch := make(chan resource.EventMessage, channelBufSize)
    doneCh := make(chan bool, 1) // Buffered to prevent blocking
    
    // Create connection handler
    webSocketConnectionHandler := WebSocketConnectionHandlerImpl{
        DtopicMap:        server.dtopicMap,
        subscribedTopics: make(map[string]*redis.PubSub),
        olricDb:          server.olricDb,
        cruds:            server.cruds,
    }
    
    client := &Client{
        id:                         clientId,
        ws:                         ws,
        server:                     server,
        ch:                         ch,
        doneCh:                     doneCh,
        user:                       user,
        webSocketConnectionHandler: webSocketConnectionHandler,
        ctx:                        ctx,
        cancel:                     cancel,
        lastActivity:               time.Now(),
        isActive:                   true,
    }
    
    // Increment active client count
    atomic.AddInt64(&activeClients, 1)
    
    log.Printf("Client %d created for user %s", clientId, user.UserReferenceId)
    
    return client, nil
}

// Conn returns the WebSocket connection
func (c *Client) Conn() *websocket.Conn {
    return c.ws
}

// Write sends a message to the client with proper error handling
func (c *Client) Write(msg resource.EventMessage) error {
    c.mutex.RLock()
    if !c.isActive {
        c.mutex.RUnlock()
        return fmt.Errorf("client %d is not active", c.id)
    }
    c.mutex.RUnlock()
    
    // Validate message size
    if len(msg.EventData) > maxJSONSize {
        return fmt.Errorf("message too large: %d bytes", len(msg.EventData))
    }
    
    select {
    case c.ch <- msg:
        return nil
    case <-time.After(time.Second):
        // Channel full or blocked
        log.Printf("Client %d channel blocked, disconnecting", c.id)
        c.disconnect("channel blocked")
        return fmt.Errorf("client channel blocked")
    case <-c.ctx.Done():
        return fmt.Errorf("client context cancelled")
    }
}

// Done signals the client to disconnect
func (c *Client) Done() {
    c.disconnect("done signal received")
}

// disconnect safely disconnects the client
func (c *Client) disconnect(reason string) {
    c.mutex.Lock()
    defer c.mutex.Unlock()
    
    if !c.isActive {
        return // Already disconnected
    }
    
    c.isActive = false
    log.Printf("Client %d disconnecting: %s", c.id, reason)
    
    // Cancel context
    c.cancel()
    
    // Send done signal
    select {
    case c.doneCh <- true:
    default:
        // Channel already has signal or is closed
    }
    
    // Remove from server
    c.server.Del(c)
    
    // Decrement active client count
    atomic.AddInt64(&activeClients, -1)
}

// cleanup performs final resource cleanup
func (c *Client) cleanup() {
    c.mutex.Lock()
    defer c.mutex.Unlock()
    
    // Close channels
    close(c.ch)
    close(c.doneCh)
    
    // Close WebSocket connection
    if c.ws != nil {
        c.ws.Close()
    }
    
    // Cleanup subscriptions
    for topic, pubsub := range c.webSocketConnectionHandler.subscribedTopics {
        if pubsub != nil {
            pubsub.Close()
        }
        delete(c.webSocketConnectionHandler.subscribedTopics, topic)
    }
    
    log.Printf("Client %d cleanup completed", c.id)
}

// updateActivity updates the last activity timestamp
func (c *Client) updateActivity() {
    c.mutex.Lock()
    c.lastActivity = time.Now()
    c.mutex.Unlock()
}

// isExpired checks if the client has expired
func (c *Client) isExpired() bool {
    c.mutex.RLock()
    defer c.mutex.RUnlock()
    return time.Since(c.lastActivity) > clientTimeout
}

// Listen starts listening for read and write operations
func (c *Client) Listen() {
    defer c.cleanup()
    
    // Start write listener
    go c.listenWrite()
    
    // Start read listener (blocking)
    c.listenRead()
}

// listenWrite handles outgoing messages to the client
func (c *Client) listenWrite() {
    defer func() {
        if r := recover(); r != nil {
            log.Printf("Recovered from panic in listenWrite for client %d: %v", c.id, r)
        }
    }()
    
    for {
        select {
        case msg := <-c.ch:
            c.updateActivity()
            
            // Send message with timeout
            done := make(chan error, 1)
            go func() {
                done <- websocket.JSON.Send(c.ws, msg)
            }()
            
            select {
            case err := <-done:
                if err != nil {
                    log.Printf("Failed to send message to client %d: %v", c.id, err)
                    c.disconnect("send error")
                    return
                }
            case <-time.After(10 * time.Second):
                log.Printf("Send timeout for client %d", c.id)
                c.disconnect("send timeout")
                return
            case <-c.ctx.Done():
                log.Printf("Context cancelled for client %d", c.id)
                return
            }
            
        case <-c.doneCh:
            log.Printf("Done signal received for client %d", c.id)
            return
            
        case <-c.ctx.Done():
            log.Printf("Context timeout for client %d", c.id)
            c.disconnect("context timeout")
            return
        }
    }
}

// listenRead handles incoming messages from the client
func (c *Client) listenRead() {
    defer func() {
        if r := recover(); r != nil {
            log.Printf("Recovered from panic in listenRead for client %d: %v", c.id, r)
        }
    }()
    
    log.Printf("Client %d starting read listener", c.id)
    
    for {
        select {
        case <-c.doneCh:
            log.Printf("Done signal received in read listener for client %d", c.id)
            return
            
        case <-c.ctx.Done():
            log.Printf("Context cancelled in read listener for client %d", c.id)
            return
            
        default:
            // Set read deadline
            deadline := time.Now().Add(30 * time.Second)
            c.ws.SetReadDeadline(deadline)
            
            var msg WebSocketPayload
            err := websocket.JSON.Receive(c.ws, &msg)
            
            if err == io.EOF {
                log.Printf("Client %d disconnected (EOF)", c.id)
                c.disconnect("client disconnected")
                return
            } else if err != nil {
                if !c.isExpired() {
                    log.Printf("Read error for client %d: %v", c.id, err)
                }
                c.disconnect("read error")
                return
            }
            
            c.updateActivity()
            
            // Validate message size
            if len(msg.Method) > 100 {
                log.Printf("Message method too long from client %d", c.id)
                c.disconnect("invalid message")
                return
            }
            
            // Process message with error handling
            func() {
                defer func() {
                    if r := recover(); r != nil {
                        log.Printf("Recovered from panic processing message for client %d: %v", c.id, r)
                        c.disconnect("message processing panic")
                    }
                }()
                
                c.webSocketConnectionHandler.MessageFromClient(msg, c)
            }()
        }
    }
}

// GetStats returns client statistics
func (c *Client) GetStats() ClientStats {
    c.mutex.RLock()
    defer c.mutex.RUnlock()
    
    return ClientStats{
        ID:           c.id,
        UserID:       string(c.user.UserReferenceId),
        LastActivity: c.lastActivity,
        IsActive:     c.isActive,
    }
}

// GetActiveClientCount returns the number of active clients
func GetActiveClientCount() int64 {
    return atomic.LoadInt64(&activeClients)
}
```

### Long-term Improvements
1. **Authentication Integration:** Integrate with comprehensive authentication middleware
2. **Rate Limiting:** Implement per-client rate limiting for message processing
3. **Connection Pooling:** Optimize resource usage with connection pooling
4. **Message Encryption:** Add message-level encryption for sensitive data
5. **Monitoring Integration:** Add comprehensive metrics and health checking

## Edge Cases Identified

1. **Rapid Connect/Disconnect:** High-frequency connection establishment and teardown
2. **Context Cancellation:** Handling of cancelled contexts during message processing
3. **Channel Overflow:** Handling of full channels under high message volume
4. **Network Interruptions:** Connection drops during message send/receive
5. **Memory Pressure:** Client operations under high memory pressure
6. **User Session Expiry:** Handling of expired user sessions during connection
7. **Server Shutdown:** Graceful client disconnection during server shutdown
8. **Message Size Limits:** Handling of oversized messages

## Security Best Practices Violations

1. **Unsafe type assertion** without validation in authentication
2. **Global state race conditions** in client ID generation
3. **Missing resource cleanup** for channels and connections
4. **Information disclosure** through detailed error messages
5. **No input validation** for JSON message size or content
6. **Missing timeout handling** for long-running operations
7. **No rate limiting** for message processing

## Positive Security Aspects

1. **User authentication** integration with session system
2. **Channel-based communication** for decoupled message handling
3. **Error handling** for connection failures
4. **Separation of concerns** between client and connection handler

## Critical Issues Summary

1. **Type Assertion Vulnerability:** Unsafe type assertion in user authentication can panic application
2. **Global State Race Condition:** Global maxId variable accessed without synchronization
3. **Channel Resource Leak:** Channels not properly closed causing memory leaks
4. **Error Information Disclosure:** Detailed error messages exposed to clients
5. **Panic in Client Creation:** Function panics instead of returning errors
6. **Channel Blocking Issues:** Poor handling of full channels causing disconnections
7. **JSON Deserialization Security:** No validation or size limits for JSON messages
8. **Missing Context Cancellation:** Long-running operations without proper cancellation

---
**Analysis Date:** 2025-01-27  
**Analyst:** Claude  
**Priority:** CRITICAL - WebSocket client management with critical type assertion and resource management vulnerabilities