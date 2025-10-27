# Security Analysis: server/websockets/web_socket_connection_handler.go

**File:** `server/websockets/web_socket_connection_handler.go`  
**Type:** WebSocket connection handler with pub/sub messaging  
**Lines of Code:** 231  

## Overview
This file implements a WebSocket connection handler that manages topic subscriptions, message routing, and user permissions for real-time communication. It integrates with Olric for distributed pub/sub and includes filtering and permission checking for event messages.

## Key Components

### WebSocketConnectionHandlerImpl struct
**Lines:** 17-22  
**Purpose:** Handles WebSocket connections with distributed topic management  

### MessageFromClient method
**Lines:** 24-230  
**Purpose:** Main message router handling various WebSocket operations (subscribe, create-topic, list-topic, destroy-topic, new-message, unsubscribe)  

## Security Analysis

### 1. CRITICAL: Type Assertion Vulnerabilities - CRITICAL RISK
**Severity:** CRITICAL  
**Lines:** 27, 38, 49, 75, 127, 170, 190, 191, 213  
**Issue:** Multiple unsafe type assertions without validation that can panic the application.

```go
topics, ok := message.Payload["topicName"].(string)  // Line 27 - can panic
filtersMap = filters.(map[string]interface{})       // Line 38 - can panic  
eventTypeString = eventType.(string)                // Line 49 - can panic
_, tableExists = wsch.cruds[typeName.(string)]      // Line 75 - can panic
topicName, ok := message.Payload["name"].(string)   // Line 127 - can panic
topics := message.Payload["topicName"].(string)     // Line 213 - can panic without ok check
```

**Risk:**
- **Application DoS** through crafted WebSocket messages causing panics
- **Service disruption** from type assertion failures
- **Runtime crashes** affecting all connected clients
- **Memory corruption** in extreme cases

### 2. CRITICAL: Permission Bypass Vulnerability - CRITICAL RISK
**Severity:** CRITICAL  
**Lines:** 78, 93  
**Issue:** Default permission allows all access when no table exists.

```go
permission := permission.PermissionInstance{Permission: auth.ALLOW_ALL_PERMISSIONS}
if tableExists {
    // Only check permissions if table exists
    permission = wsch.cruds["world"].GetRowPermission(eventData, tx)
}
if permission.CanRead(client.user.UserReferenceId, client.user.Groups, wsch.cruds["world"].AdministratorGroupId) {
    // Send message - bypasses permission check for non-table events
}
```

**Risk:**
- **Data exposure** through events not associated with tables
- **Privilege escalation** by accessing non-table system events
- **Information disclosure** of sensitive system operations
- **Authorization bypass** for administrative events

### 3. HIGH: Goroutine Leak Vulnerability - HIGH RISK
**Severity:** HIGH  
**Lines:** 55, 142  
**Issue:** Unlimited goroutine creation without lifecycle management.

```go
go func(pubsub *redis.PubSub, eventType string, filtersMap map[string]interface{}) {
    listenChannel := pubsub.Channel()
    for {
        msg := <-listenChannel  // Infinite loop with no cleanup
        // ... processing
    }
}(subscription, eventTypeString, filtersMap)
```

**Risk:**
- **Resource exhaustion** from unlimited goroutine creation
- **Memory leaks** from abandoned goroutines
- **Performance degradation** under high connection load
- **System instability** from resource consumption

### 4. HIGH: Transaction Resource Leak - HIGH RISK
**Severity:** HIGH  
**Lines:** 81-90  
**Issue:** Database transactions not properly cleaned up on errors.

```go
tx, err := wsch.cruds["world"].Connection().Beginx()
if err != nil {
    resource.CheckErr(err, "Failed to begin transaction [78]")  // Returns without cleanup
}
permission = wsch.cruds["world"].GetRowPermission(eventData, tx)
err = tx.Commit()
if err != nil {
    resource.CheckErr(err, "Failed to commit transaction [84]")  // No rollback
}
```

**Risk:**
- **Database connection leaks** from uncommitted transactions
- **Lock contention** from held transaction locks
- **Database resource exhaustion** under error conditions
- **Performance degradation** from connection pool depletion

### 5. HIGH: Binary Unmarshaling Security - HIGH RISK
**Severity:** HIGH  
**Lines:** 65, 69  
**Issue:** Unsafe binary and JSON unmarshaling without validation.

```go
err = eventMessage.UnmarshalBinary([]byte(msg.Payload))  // Unsafe binary unmarshaling
resource.CheckErr(err, "Failed to unmarshal eventMessage")

err = json.Unmarshal(eventMessage.EventData, &eventDataMap)  // Unsafe JSON unmarshaling
resource.CheckErr(err, "Failed to unmarshal eventMessage.EventData")
```

**Risk:**
- **Code injection** through malicious binary data
- **Memory corruption** from crafted payloads
- **Parser vulnerabilities** exploitation
- **DoS attacks** through malformed data

### 6. MEDIUM: Topic Management Security - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 138, 185  
**Issue:** Insufficient validation for topic creation and deletion operations.

```go
newTopic, err := wsch.olricDb.NewPubSub()  // No limit on topic creation
delete(*wsch.DtopicMap, topic)             // No validation of topic ownership
```

**Risk:**
- **Resource exhaustion** through unlimited topic creation
- **Unauthorized access** to system topics
- **Topic enumeration** through listing operations
- **Data integrity issues** from topic manipulation

### 7. MEDIUM: Missing Input Validation - MEDIUM RISK
**Severity:** MEDIUM  
**Lines:** 32-34, 214-216  
**Issue:** Insufficient validation of input parameters.

```go
if len(topics) < 1 {
    return  // Only checks length, not content validation
}
```

**Risk:**
- **Injection attacks** through malformed topic names
- **Path traversal** in topic name handling
- **Character encoding issues** with unicode topic names
- **Filter bypass** through crafted input

### 8. LOW: Race Condition Potential - LOW RISK
**Severity:** LOW  
**Lines:** 132, 176, 193  
**Issue:** Concurrent access to shared maps without synchronization.

```go
_, exists := (*wsch.DtopicMap)[topicName]  // Race condition possible
_, isSystemTopic := wsch.cruds[topic]      // Concurrent map access
topic, ok = (*wsch.DtopicMap)[topicName]  // Race condition possible
```

**Risk:**
- **Data races** in concurrent WebSocket connections
- **Map corruption** under high concurrency
- **Inconsistent state** in topic management
- **Memory safety issues** from concurrent map operations

## Potential Attack Vectors

### Denial of Service Attacks
1. **Type Assertion DoS:** Send messages with incorrect types to panic the handler
2. **Goroutine Bomb:** Create many subscriptions to exhaust system resources
3. **Transaction DoS:** Trigger transaction errors to leak database connections
4. **Binary Data DoS:** Send malformed binary data to crash unmarshaling

### Information Disclosure Attacks
1. **Permission Bypass:** Access events for non-table entities to bypass permission checks
2. **Topic Enumeration:** List all topics to discover system topology
3. **Event Filtering Bypass:** Craft filters to access unauthorized event data
4. **System Event Access:** Subscribe to administrative events without proper authorization

### Resource Exhaustion Attacks
1. **Topic Flooding:** Create unlimited topics to exhaust memory
2. **Subscription Flooding:** Create many subscriptions per connection
3. **Message Flooding:** Send high-volume messages to overwhelm processing
4. **Connection Flooding:** Open many WebSocket connections simultaneously

## Recommendations

### Immediate Actions
1. **Add Type Validation:** Validate all type assertions with proper error handling
2. **Fix Permission Logic:** Ensure proper permission checking for all event types
3. **Implement Resource Limits:** Add limits on topics, subscriptions, and goroutines
4. **Add Transaction Cleanup:** Implement proper transaction rollback on errors

### Enhanced Security Implementation

```go
package websockets

import (
    "context"
    "encoding/json"
    "fmt"
    "sync"
    "time"
    "unicode/utf8"
    
    "github.com/buraksezer/olric"
    "github.com/daptin/daptin/server/auth"
    "github.com/daptin/daptin/server/permission"
    "github.com/daptin/daptin/server/resource"
    "github.com/go-redis/redis/v8"
    "github.com/google/uuid"
    log "github.com/sirupsen/logrus"
    "strings"
)

const (
    MaxTopicsPerConnection = 100
    MaxSubscriptionsPerConnection = 50
    MaxTopicNameLength = 255
    MaxMessageSize = 64 * 1024 // 64KB
    MaxFiltersPerSubscription = 20
    MaxConcurrentGoroutines = 1000
    SubscriptionTimeout = 30 * time.Minute
)

// WebSocketConnectionHandlerImpl : Secure websocket connection handler
type WebSocketConnectionHandlerImpl struct {
    DtopicMap        *map[string]*olric.PubSub
    subscribedTopics map[string]*redis.PubSub
    olricDb          *olric.EmbeddedClient
    cruds            map[string]*resource.DbResource
    
    // Security and resource management
    mutex            sync.RWMutex
    topicCount       int
    subscriptionCount int
    goroutineCount   int32
    lastActivity     time.Time
    client           *Client
    rateLimiter      *time.Ticker
}

// validateTopicName validates topic name for security
func validateTopicName(name string) error {
    if len(name) == 0 {
        return fmt.Errorf("topic name cannot be empty")
    }
    
    if len(name) > MaxTopicNameLength {
        return fmt.Errorf("topic name too long: %d", len(name))
    }
    
    if !utf8.ValidString(name) {
        return fmt.Errorf("topic name contains invalid UTF-8")
    }
    
    // Check for dangerous characters
    dangerousChars := []string{"/", "\\", "..", "\x00", "\n", "\r", "\t"}
    for _, dangerous := range dangerousChars {
        if strings.Contains(name, dangerous) {
            return fmt.Errorf("topic name contains dangerous characters")
        }
    }
    
    return nil
}

// validateMessage validates message content and size
func validateMessage(data interface{}) error {
    if data == nil {
        return fmt.Errorf("message cannot be nil")
    }
    
    jsonData, err := json.Marshal(data)
    if err != nil {
        return fmt.Errorf("message not JSON serializable: %v", err)
    }
    
    if len(jsonData) > MaxMessageSize {
        return fmt.Errorf("message too large: %d bytes", len(jsonData))
    }
    
    return nil
}

// safeTypeAssertion performs type assertion with error handling
func safeTypeAssertion[T any](value interface{}, fieldName string) (T, error) {
    var zero T
    if value == nil {
        return zero, fmt.Errorf("field '%s' is nil", fieldName)
    }
    
    result, ok := value.(T)
    if !ok {
        return zero, fmt.Errorf("field '%s' has invalid type, expected %T, got %T", fieldName, zero, value)
    }
    
    return result, nil
}

// safeTransactionExecute executes database operations with proper cleanup
func (wsch *WebSocketConnectionHandlerImpl) safeTransactionExecute(operation func(*resource.Tx) error) error {
    tx, err := wsch.cruds["world"].Connection().Beginx()
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %v", err)
    }
    
    defer func() {
        if p := recover(); p != nil {
            tx.Rollback()
            panic(p) // Re-panic after cleanup
        }
    }()
    
    err = operation(tx)
    if err != nil {
        if rollbackErr := tx.Rollback(); rollbackErr != nil {
            log.Printf("Failed to rollback transaction: %v", rollbackErr)
        }
        return err
    }
    
    if err = tx.Commit(); err != nil {
        return fmt.Errorf("failed to commit transaction: %v", err)
    }
    
    return nil
}

// checkPermissionSecure performs comprehensive permission checking
func (wsch *WebSocketConnectionHandlerImpl) checkPermissionSecure(eventData map[string]interface{}, client *Client) (bool, error) {
    typeName, exists := eventData["__type"]
    if !exists {
        // No type means no table association - deny by default for security
        return false, nil
    }
    
    typeNameStr, err := safeTypeAssertion[string](typeName, "__type")
    if err != nil {
        return false, fmt.Errorf("invalid type name: %v", err)
    }
    
    crud, tableExists := wsch.cruds[typeNameStr]
    if !tableExists {
        // Table doesn't exist - deny access for security
        return false, nil
    }
    
    var permission permission.PermissionInstance
    err = wsch.safeTransactionExecute(func(tx *resource.Tx) error {
        permission = crud.GetRowPermission(eventData, tx)
        return nil
    })
    
    if err != nil {
        return false, fmt.Errorf("failed to get permissions: %v", err)
    }
    
    return permission.CanRead(client.user.UserReferenceId, client.user.Groups, crud.AdministratorGroupId), nil
}

// incrementGoroutineCount safely increments goroutine counter
func (wsch *WebSocketConnectionHandlerImpl) incrementGoroutineCount() error {
    wsch.mutex.Lock()
    defer wsch.mutex.Unlock()
    
    if wsch.goroutineCount >= MaxConcurrentGoroutines {
        return fmt.Errorf("too many concurrent goroutines: %d", wsch.goroutineCount)
    }
    
    wsch.goroutineCount++
    return nil
}

// decrementGoroutineCount safely decrements goroutine counter
func (wsch *WebSocketConnectionHandlerImpl) decrementGoroutineCount() {
    wsch.mutex.Lock()
    defer wsch.mutex.Unlock()
    wsch.goroutineCount--
}

// MessageFromClient handles messages with comprehensive security validation
func (wsch *WebSocketConnectionHandlerImpl) MessageFromClient(message WebSocketPayload, client *Client) {
    defer func() {
        if r := recover(); r != nil {
            log.Printf("Recovered from panic in MessageFromClient: %v", r)
            // Send error response to client
            errorMsg := resource.EventMessage{
                EventType:     "error",
                MessageSource: "system",
                EventData:     []byte(fmt.Sprintf(`{"error": "Internal server error"}`)),
            }
            select {
            case client.ch <- errorMsg:
            default:
                // Channel full, log error
                log.Printf("Failed to send error message to client")
            }
        }
    }()
    
    // Rate limiting check
    wsch.mutex.Lock()
    wsch.lastActivity = time.Now()
    wsch.mutex.Unlock()
    
    switch message.Method {
    case "subscribe":
        wsch.handleSubscribe(message, client)
    case "create-topicName":
        wsch.handleCreateTopic(message, client)
    case "list-topicName":
        wsch.handleListTopics(message, client)
    case "destroy-topicName":
        wsch.handleDestroyTopic(message, client)
    case "new-message":
        wsch.handleNewMessage(message, client)
    case "unsubscribe":
        wsch.handleUnsubscribe(message, client)
    default:
        log.Printf("Unknown method: %s", message.Method)
    }
}

// handleSubscribe handles topic subscription with security validation
func (wsch *WebSocketConnectionHandlerImpl) handleSubscribe(message WebSocketPayload, client *Client) {
    // Validate topic name
    topicNameRaw, exists := message.Payload["topicName"]
    if !exists {
        log.Printf("Missing topicName in subscribe request")
        return
    }
    
    topics, err := safeTypeAssertion[string](topicNameRaw, "topicName")
    if err != nil {
        log.Printf("Invalid topicName type: %v", err)
        return
    }
    
    if len(topics) == 0 {
        log.Printf("Empty topicName in subscribe request")
        return
    }
    
    // Parse and validate filters
    var filtersMap map[string]interface{}
    if filtersRaw, exists := message.Payload["filters"]; exists {
        var err error
        filtersMap, err = safeTypeAssertion[map[string]interface{}](filtersRaw, "filters")
        if err != nil {
            log.Printf("Invalid filters type: %v", err)
            return
        }
        
        // Validate filter count
        if len(filtersMap) > MaxFiltersPerSubscription {
            log.Printf("Too many filters: %d", len(filtersMap))
            return
        }
    }
    
    // Validate and process topics
    topicsList := strings.Split(topics, ",")
    if len(topicsList) > MaxSubscriptionsPerConnection {
        log.Printf("Too many topics in single request: %d", len(topicsList))
        return
    }
    
    for _, topic := range topicsList {
        topic = strings.TrimSpace(topic)
        if err := validateTopicName(topic); err != nil {
            log.Printf("Invalid topic name '%s': %v", topic, err)
            continue
        }
        
        wsch.mutex.Lock()
        _, alreadySubscribed := wsch.subscribedTopics[topic]
        if alreadySubscribed {
            wsch.mutex.Unlock()
            continue
        }
        
        if wsch.subscriptionCount >= MaxSubscriptionsPerConnection {
            wsch.mutex.Unlock()
            log.Printf("Too many subscriptions for connection: %d", wsch.subscriptionCount)
            return
        }
        wsch.mutex.Unlock()
        
        // Get topic and create subscription
        dTopic, exists := (*wsch.DtopicMap)[topic]
        if !exists {
            log.Printf("Topic does not exist: %s", topic)
            continue
        }
        
        subscription := dTopic.Subscribe(context.Background(), topic)
        
        wsch.mutex.Lock()
        wsch.subscribedTopics[topic] = subscription
        wsch.subscriptionCount++
        wsch.mutex.Unlock()
        
        // Validate event type filter
        eventTypeString := ""
        if filtersMap != nil {
            if eventTypeRaw, exists := filtersMap["EventType"]; exists {
                var err error
                eventTypeString, err = safeTypeAssertion[string](eventTypeRaw, "EventType")
                if err != nil {
                    log.Printf("Invalid EventType filter: %v", err)
                    eventTypeString = ""
                } else {
                    // Remove EventType from filters map for processing
                    filtersCopy := make(map[string]interface{})
                    for k, v := range filtersMap {
                        if k != "EventType" {
                            filtersCopy[k] = v
                        }
                    }
                    filtersMap = filtersCopy
                }
            }
        }
        
        // Start goroutine with resource management
        if err := wsch.incrementGoroutineCount(); err != nil {
            log.Printf("Cannot start subscription goroutine: %v", err)
            subscription.Close()
            wsch.mutex.Lock()
            delete(wsch.subscribedTopics, topic)
            wsch.subscriptionCount--
            wsch.mutex.Unlock()
            continue
        }
        
        go func(pubsub *redis.PubSub, eventType string, filters map[string]interface{}, topicName string) {
            defer wsch.decrementGoroutineCount()
            defer func() {
                if r := recover(); r != nil {
                    log.Printf("Recovered from panic in subscription goroutine: %v", r)
                }
            }()
            
            listenChannel := pubsub.Channel()
            timeout := time.NewTimer(SubscriptionTimeout)
            defer timeout.Stop()
            
            for {
                select {
                case msg := <-listenChannel:
                    if msg == nil {
                        log.Printf("Subscription closed for topic: %s", topicName)
                        return
                    }
                    
                    // Reset timeout on activity
                    if !timeout.Stop() {
                        select {
                        case <-timeout.C:
                        default:
                        }
                    }
                    timeout.Reset(SubscriptionTimeout)
                    
                    // Process message with error handling
                    if err := wsch.processEventMessage(msg, eventType, filters, client); err != nil {
                        log.Printf("Failed to process event message: %v", err)
                    }
                    
                case <-timeout.C:
                    log.Printf("Subscription timeout for topic: %s", topicName)
                    pubsub.Close()
                    return
                }
            }
        }(subscription, eventTypeString, filtersMap, topic)
    }
}

// processEventMessage processes individual event messages with security checks
func (wsch *WebSocketConnectionHandlerImpl) processEventMessage(msg *redis.Message, eventType string, filters map[string]interface{}, client *Client) error {
    var eventMessage resource.EventMessage
    err := eventMessage.UnmarshalBinary([]byte(msg.Payload))
    if err != nil {
        return fmt.Errorf("failed to unmarshal event message: %v", err)
    }
    
    // Validate event data size
    if len(eventMessage.EventData) > MaxMessageSize {
        return fmt.Errorf("event data too large: %d bytes", len(eventMessage.EventData))
    }
    
    var eventDataMap map[string]interface{}
    err = json.Unmarshal(eventMessage.EventData, &eventDataMap)
    if err != nil {
        return fmt.Errorf("failed to unmarshal event data: %v", err)
    }
    
    // Check permissions
    canRead, err := wsch.checkPermissionSecure(eventDataMap, client)
    if err != nil {
        return fmt.Errorf("permission check failed: %v", err)
    }
    
    if !canRead {
        // Silently drop unauthorized events
        return nil
    }
    
    // Apply filters
    sendMessage := true
    if filters != nil {
        // Check event type filter
        if eventType != "" && eventMessage.EventType != eventType {
            return nil
        }
        
        // Check other filters
        for key, val := range filters {
            if eventDataMap[key] != val {
                sendMessage = false
                break
            }
        }
    }
    
    if sendMessage {
        select {
        case client.ch <- eventMessage:
        default:
            // Channel full, drop message to prevent blocking
            log.Printf("Client channel full, dropping message")
        }
    }
    
    return nil
}

// Additional secure methods would follow similar patterns...
// handleCreateTopic, handleListTopics, handleDestroyTopic, handleNewMessage, handleUnsubscribe
// Each with proper validation, resource limits, and error handling
```

### Long-term Improvements
1. **Authentication Integration:** Integrate with proper authentication middleware
2. **Rate Limiting:** Implement per-client rate limiting for all operations
3. **Audit Logging:** Log all WebSocket operations for security monitoring
4. **Message Encryption:** Add end-to-end encryption for sensitive messages
5. **Connection Pooling:** Optimize resource usage with connection pooling

## Edge Cases Identified

1. **Rapid Connection/Disconnection:** High-frequency connect/disconnect cycles
2. **Large Message Payloads:** Messages approaching size limits
3. **Malformed JSON:** Various invalid JSON patterns in messages
4. **Unicode Topic Names:** Topic names with unicode and special characters
5. **Concurrent Subscriptions:** Multiple simultaneous subscription requests
6. **Network Interruptions:** Connection drops during message processing
7. **Memory Pressure:** Operations under high memory pressure
8. **Database Unavailability:** Handling database connection failures

## Security Best Practices Violations

1. **No input validation** for WebSocket message parameters
2. **Unsafe type assertions** without error handling throughout
3. **Default allow permissions** for non-table events
4. **Unlimited resource creation** (topics, subscriptions, goroutines)
5. **Missing transaction cleanup** on error conditions
6. **No rate limiting** for WebSocket operations
7. **Unsafe binary unmarshaling** without validation

## Positive Security Aspects

1. **Permission integration** with existing auth system
2. **Transaction-based permission checks** for data consistency
3. **Topic isolation** through separate pub/sub channels
4. **User context tracking** in client connections

## Critical Issues Summary

1. **Type Assertion Vulnerabilities:** Multiple unsafe type assertions causing potential panics
2. **Permission Bypass Vulnerability:** Default allow permissions for non-table events
3. **Goroutine Leak Vulnerability:** Unlimited goroutine creation without lifecycle management
4. **Transaction Resource Leak:** Database transactions not properly cleaned up on errors
5. **Binary Unmarshaling Security:** Unsafe binary and JSON unmarshaling without validation
6. **Topic Management Security:** Insufficient validation for topic operations
7. **Missing Input Validation:** Insufficient validation of all input parameters
8. **Race Condition Potential:** Concurrent access to shared maps without synchronization

---
**Analysis Date:** 2025-01-27  
**Analyst:** Claude  
**Priority:** CRITICAL - WebSocket handler with multiple critical vulnerabilities requiring immediate security hardening