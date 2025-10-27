# Security Analysis: server/task_scheduler/task_scheduler.go

**File:** `server/task_scheduler/task_scheduler.go`  
**Type:** Task scheduler interface definition  
**Lines of Code:** 9  

## Overview
This file defines the TaskScheduler interface for managing scheduled tasks in the Daptin system. It provides a minimal contract for task scheduling operations including starting, adding, and stopping tasks.

## Key Components

### TaskScheduler interface
**Lines:** 5-9  
**Purpose:** Interface defining task scheduler operations  

## Security Analysis

### 1. LOW: Missing Method Documentation - LOW RISK
**Severity:** LOW  
**Lines:** 5-9  
**Issue:** Interface methods lack parameter validation specifications.

```go
type TaskScheduler interface {
    StartTasks()
    AddTask(task task.Task) error  // No validation requirements specified
    StopTasks()
}
```

**Risk:**
- **Implementation inconsistency** across different TaskScheduler implementations
- **No validation contracts** for AddTask method
- **Unclear error handling** behavior expectations
- **Missing concurrency safety** requirements

### 2. LOW: Interface Design Limitations - LOW RISK
**Severity:** LOW  
**Lines:** 7  
**Issue:** AddTask method accepts task by value, not validation requirements.

```go
AddTask(task task.Task) error  // Task passed by value
```

**Risk:**
- **No validation enforcement** at interface level
- **Implementation freedom** may lead to inconsistent security
- **Task modification** after addition not prevented
- **No task lifetime management** specified

### 3. LOW: Missing Context Support - LOW RISK
**Severity:** LOW  
**Lines:** 5-9  
**Issue:** Interface methods don't support context for cancellation or timeouts.

```go
StartTasks()              // No context support
AddTask(task task.Task)   // No context support  
StopTasks()              // No context support
```

**Risk:**
- **No timeout control** for long-running operations
- **No cancellation support** for graceful shutdowns
- **Resource leak potential** in blocking operations
- **Difficult testing** without context control

### 4. LOW: No Task Identification - LOW RISK
**Severity:** LOW  
**Lines:** 7  
**Issue:** No method to identify, retrieve, or remove specific tasks.

```go
AddTask(task task.Task) error  // No task ID returned
```

**Risk:**
- **No task management** capabilities beyond adding
- **Resource exhaustion** from accumulated tasks
- **No task status monitoring** possible
- **Difficult debugging** without task introspection

## Potential Attack Vectors

### Interface Implementation Attacks
1. **Malicious Implementation:** Create TaskScheduler implementation with security vulnerabilities
2. **Resource Exhaustion:** Add excessive tasks without removal mechanism
3. **Task Injection:** Add malicious tasks through AddTask method

### Operational Security Issues
1. **Missing Validation:** Implementations may not validate task parameters
2. **Concurrent Access:** No thread safety requirements specified
3. **Resource Management:** No cleanup or resource limit specifications

## Recommendations

### Immediate Actions
1. **Add Method Documentation:** Specify validation and security requirements
2. **Context Support:** Add context.Context parameters to methods
3. **Task Management:** Add methods for task identification and removal
4. **Error Specifications:** Define specific error types and handling

### Enhanced Security Implementation

```go
package task_scheduler

import (
    "context"
    "fmt"
    "time"
    
    "github.com/daptin/daptin/server/task"
)

// TaskStatus represents the current status of a task
type TaskStatus string

const (
    TaskStatusPending   TaskStatus = "pending"
    TaskStatusRunning   TaskStatus = "running" 
    TaskStatusCompleted TaskStatus = "completed"
    TaskStatusFailed    TaskStatus = "failed"
    TaskStatusCanceled  TaskStatus = "canceled"
)

// TaskInfo provides information about a scheduled task
type TaskInfo struct {
    ID          string      `json:"id"`
    Task        task.Task   `json:"task"`
    Status      TaskStatus  `json:"status"`
    CreatedAt   time.Time   `json:"created_at"`
    StartedAt   *time.Time  `json:"started_at,omitempty"`
    CompletedAt *time.Time  `json:"completed_at,omitempty"`
    Error       string      `json:"error,omitempty"`
    RetryCount  int         `json:"retry_count"`
}

// TaskSchedulerConfig provides configuration for task scheduler
type TaskSchedulerConfig struct {
    MaxConcurrentTasks int           `json:"max_concurrent_tasks"`
    MaxTasksInQueue    int           `json:"max_tasks_in_queue"`
    TaskTimeout        time.Duration `json:"task_timeout"`
    RetryLimit         int           `json:"retry_limit"`
    RetryDelay         time.Duration `json:"retry_delay"`
}

// TaskSchedulerStats provides runtime statistics
type TaskSchedulerStats struct {
    TotalTasks      int `json:"total_tasks"`
    PendingTasks    int `json:"pending_tasks"`
    RunningTasks    int `json:"running_tasks"`
    CompletedTasks  int `json:"completed_tasks"`
    FailedTasks     int `json:"failed_tasks"`
    CanceledTasks   int `json:"canceled_tasks"`
}

// TaskScheduler provides secure task scheduling operations
type TaskScheduler interface {
    // StartTasks begins processing scheduled tasks
    // Returns error if scheduler is already running or fails to start
    StartTasks(ctx context.Context) error
    
    // AddTask adds a new task to the scheduler with validation
    // Returns task ID and error if validation fails or queue is full
    AddTask(ctx context.Context, task task.Task) (string, error)
    
    // AddTaskWithPriority adds a task with specified priority
    // Higher priority tasks are processed first
    AddTaskWithPriority(ctx context.Context, task task.Task, priority int) (string, error)
    
    // GetTask retrieves task information by ID
    // Returns task info and error if task not found
    GetTask(ctx context.Context, taskID string) (*TaskInfo, error)
    
    // GetTasks retrieves all tasks with optional status filter
    // Returns slice of task info and error if operation fails
    GetTasks(ctx context.Context, status ...TaskStatus) ([]*TaskInfo, error)
    
    // CancelTask cancels a specific task by ID
    // Returns error if task not found or cannot be canceled
    CancelTask(ctx context.Context, taskID string) error
    
    // RemoveTask removes a completed/failed/canceled task from history
    // Returns error if task not found or still running
    RemoveTask(ctx context.Context, taskID string) error
    
    // GetStats returns current scheduler statistics
    GetStats(ctx context.Context) (*TaskSchedulerStats, error)
    
    // GetConfig returns current scheduler configuration
    GetConfig() *TaskSchedulerConfig
    
    // UpdateConfig updates scheduler configuration
    // Returns error if configuration is invalid
    UpdateConfig(ctx context.Context, config *TaskSchedulerConfig) error
    
    // StopTasks gracefully stops the scheduler
    // Waits for running tasks to complete or timeout
    StopTasks(ctx context.Context, timeout time.Duration) error
    
    // ForceStop immediately stops the scheduler
    // Cancels all running tasks
    ForceStop(ctx context.Context) error
    
    // IsRunning returns true if scheduler is currently running
    IsRunning() bool
    
    // Health checks the health of the scheduler
    // Returns error if scheduler is in unhealthy state
    Health(ctx context.Context) error
}

// TaskExecutor defines how individual tasks are executed
type TaskExecutor interface {
    // ExecuteTask executes a single task
    // Returns error if task execution fails
    ExecuteTask(ctx context.Context, task task.Task) error
    
    // ValidateTask validates a task before execution
    // Returns error if task is invalid
    ValidateTask(task task.Task) error
    
    // GetSupportedActions returns list of supported action names
    GetSupportedActions() []string
    
    // CanExecute returns true if executor can handle the given task
    CanExecute(task task.Task) bool
}

// TaskValidator provides task validation functionality
type TaskValidator interface {
    // ValidateTask validates task structure and content
    ValidateTask(task task.Task) error
    
    // ValidateSchedule validates cron schedule format
    ValidateSchedule(schedule string) error
    
    // ValidateUser validates user permissions for task execution
    ValidateUser(userEmail string, actionName string) error
    
    // ValidateEntity validates entity existence and permissions
    ValidateEntity(entityName string, userEmail string) error
}

// TaskAuditor provides audit logging for task operations
type TaskAuditor interface {
    // AuditTaskAdded logs task addition events
    AuditTaskAdded(ctx context.Context, taskID string, task task.Task, userID string) error
    
    // AuditTaskStarted logs task execution start events
    AuditTaskStarted(ctx context.Context, taskID string, task task.Task) error
    
    // AuditTaskCompleted logs task completion events
    AuditTaskCompleted(ctx context.Context, taskID string, task task.Task, duration time.Duration) error
    
    // AuditTaskFailed logs task failure events
    AuditTaskFailed(ctx context.Context, taskID string, task task.Task, err error) error
    
    // AuditTaskCanceled logs task cancellation events
    AuditTaskCanceled(ctx context.Context, taskID string, task task.Task, reason string) error
}

// SecureTaskScheduler combines all security interfaces
type SecureTaskScheduler interface {
    TaskScheduler
    TaskValidator
    TaskAuditor
}

// Error types for better error handling
var (
    ErrTaskNotFound       = fmt.Errorf("task not found")
    ErrTaskAlreadyRunning = fmt.Errorf("task already running")
    ErrTaskQueueFull      = fmt.Errorf("task queue full")
    ErrSchedulerNotRunning = fmt.Errorf("scheduler not running")
    ErrSchedulerAlreadyRunning = fmt.Errorf("scheduler already running")
    ErrInvalidTask        = fmt.Errorf("invalid task")
    ErrInvalidConfig      = fmt.Errorf("invalid configuration")
    ErrOperationTimeout   = fmt.Errorf("operation timeout")
    ErrUnauthorized       = fmt.Errorf("unauthorized operation")
)

// TaskSchedulerOption provides configuration options
type TaskSchedulerOption func(*TaskSchedulerConfig)

// WithMaxConcurrentTasks sets maximum concurrent task limit
func WithMaxConcurrentTasks(max int) TaskSchedulerOption {
    return func(config *TaskSchedulerConfig) {
        if max > 0 && max <= 1000 {
            config.MaxConcurrentTasks = max
        }
    }
}

// WithMaxTasksInQueue sets maximum queue size
func WithMaxTasksInQueue(max int) TaskSchedulerOption {
    return func(config *TaskSchedulerConfig) {
        if max > 0 && max <= 10000 {
            config.MaxTasksInQueue = max
        }
    }
}

// WithTaskTimeout sets task execution timeout
func WithTaskTimeout(timeout time.Duration) TaskSchedulerOption {
    return func(config *TaskSchedulerConfig) {
        if timeout > 0 && timeout <= 24*time.Hour {
            config.TaskTimeout = timeout
        }
    }
}

// WithRetryLimit sets maximum retry attempts
func WithRetryLimit(limit int) TaskSchedulerOption {
    return func(config *TaskSchedulerConfig) {
        if limit >= 0 && limit <= 10 {
            config.RetryLimit = limit
        }
    }
}

// WithRetryDelay sets delay between retry attempts
func WithRetryDelay(delay time.Duration) TaskSchedulerOption {
    return func(config *TaskSchedulerConfig) {
        if delay > 0 && delay <= time.Hour {
            config.RetryDelay = delay
        }
    }
}

// NewTaskSchedulerConfig creates a new configuration with defaults
func NewTaskSchedulerConfig(options ...TaskSchedulerOption) *TaskSchedulerConfig {
    config := &TaskSchedulerConfig{
        MaxConcurrentTasks: 10,
        MaxTasksInQueue:    1000,
        TaskTimeout:        30 * time.Minute,
        RetryLimit:         3,
        RetryDelay:         5 * time.Minute,
    }
    
    for _, option := range options {
        option(config)
    }
    
    return config
}

// ValidateConfig validates scheduler configuration
func ValidateConfig(config *TaskSchedulerConfig) error {
    if config == nil {
        return fmt.Errorf("configuration cannot be nil")
    }
    
    if config.MaxConcurrentTasks <= 0 || config.MaxConcurrentTasks > 1000 {
        return fmt.Errorf("invalid MaxConcurrentTasks: %d", config.MaxConcurrentTasks)
    }
    
    if config.MaxTasksInQueue <= 0 || config.MaxTasksInQueue > 10000 {
        return fmt.Errorf("invalid MaxTasksInQueue: %d", config.MaxTasksInQueue)
    }
    
    if config.TaskTimeout <= 0 || config.TaskTimeout > 24*time.Hour {
        return fmt.Errorf("invalid TaskTimeout: %v", config.TaskTimeout)
    }
    
    if config.RetryLimit < 0 || config.RetryLimit > 10 {
        return fmt.Errorf("invalid RetryLimit: %d", config.RetryLimit)
    }
    
    if config.RetryDelay <= 0 || config.RetryDelay > time.Hour {
        return fmt.Errorf("invalid RetryDelay: %v", config.RetryDelay)
    }
    
    return nil
}
```

### Long-term Improvements
1. **Comprehensive Interface Design:** Add full task lifecycle management
2. **Security Integration:** Integrate with authentication and authorization systems
3. **Monitoring and Metrics:** Add comprehensive monitoring and alerting
4. **Persistence Layer:** Add task persistence for reliability
5. **Distributed Scheduling:** Support for distributed task scheduling

## Edge Cases Identified

1. **Empty Interface Implementation:** Implementations with no actual functionality
2. **Resource Exhaustion:** Unlimited task addition without cleanup
3. **Concurrent Access:** Multiple goroutines accessing scheduler simultaneously
4. **Task Validation:** Tasks with invalid or malicious content
5. **Memory Pressure:** Operations under high memory pressure
6. **Implementation Panics:** Panic in scheduler implementation methods
7. **Context Cancellation:** Handling of canceled contexts mid-operation
8. **Error Propagation:** Proper error handling across interface boundaries

## Security Best Practices Adherence

✅ **Good Practices:**
1. Simple interface design reduces complexity
2. Error return for AddTask method allows validation
3. Clear separation of concerns

⚠️ **Areas for Improvement:**
1. Missing validation requirements specification
2. No context support for cancellation/timeouts
3. Limited task management capabilities
4. No security or audit requirements

## Critical Issues Summary

1. **Missing Method Documentation:** Interface methods lack validation and security requirements
2. **Interface Design Limitations:** No validation enforcement at interface level
3. **Missing Context Support:** No support for cancellation or timeouts
4. **No Task Identification:** Limited task management capabilities beyond addition

---
**Analysis Date:** 2025-01-27  
**Analyst:** Claude  
**Priority:** LOW - Simple interface requiring enhanced design for comprehensive task scheduling security