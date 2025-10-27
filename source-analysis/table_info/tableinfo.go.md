# Security Analysis: server/table_info/tableinfo.go

**File:** `server/table_info/tableinfo.go`  
**Type:** Table metadata and relationship management structure  
**Lines of Code:** 87  

## Overview
This file defines the TableInfo structure which represents database table metadata including columns, relationships, permissions, and configuration settings. It provides methods for managing table relations and column lookups.

## Key Components

### TableRelation struct
**Lines:** 10-13  
**Purpose:** Extends api2go.TableRelation with deletion behavior configuration  

### TableInfo struct
**Lines:** 15-38  
**Purpose:** Comprehensive table metadata structure with permissions, columns, and relationships  

### Column and Relation Lookup Methods
**Lines:** 40-61  
**Purpose:** Methods to find columns and relations by name  

### AddRelation method
**Lines:** 63-86  
**Purpose:** Adds relations to table with duplicate detection  

## Security Analysis

### 1. LOW: Missing Input Validation - LOW RISK
**Severity:** LOW  
**Lines:** 40-49, 51-59  
**Issue:** No validation of input parameters in lookup methods.

```go
func (ti *TableInfo) GetColumnByName(name string) (*api2go.ColumnInfo, bool) {
    for _, col := range ti.Columns {
        if col.Name == name || col.ColumnName == name {  // No validation of name parameter
            return &col, true
        }
    }
    return nil, false
}
```

**Risk:**
- **Potential for empty string searches** causing inefficient iterations
- **No length limits** on search parameters
- **Case sensitivity issues** in name matching
- **No sanitization** of input names

### 2. LOW: Pointer Return Security - LOW RISK
**Severity:** LOW  
**Lines:** 44, 55  
**Issue:** Returns pointers to internal slice elements.

```go
return &col, true      // Returns pointer to slice element
return &relation, true // Returns pointer to slice element
```

**Risk:**
- **External modification** of internal data structures
- **Data integrity issues** through external pointer manipulation
- **Memory safety concerns** if slice is reallocated
- **Race conditions** in concurrent access scenarios

### 3. LOW: Hash Collision Handling - LOW RISK
**Severity:** LOW  
**Lines:** 71-79  
**Issue:** Duplicate detection relies on hash comparison without collision handling.

```go
hash := relation.Hash()
for _, existingRelation := range ti.Relations {
    if existingRelation.Hash() == hash {  // Hash collision not handled
        exists = true
        break
    }
}
```

**Risk:**
- **False positive duplicates** from hash collisions
- **Legitimate relations rejected** due to hash conflicts
- **Data integrity issues** from incorrect duplicate detection
- **No fallback validation** beyond hash comparison

### 4. LOW: Slice Management Issues - LOW RISK
**Severity:** LOW  
**Lines:** 65-67, 82  
**Issue:** Manual slice management without bounds checking.

```go
if ti.Relations == nil {
    ti.Relations = make([]api2go.TableRelation, 0)  // Creates new slice
}
ti.Relations = append(ti.Relations, relation)       // Unbounded append
```

**Risk:**
- **Memory exhaustion** through unlimited relation additions
- **Performance degradation** with very large relation lists
- **No validation** of relation count limits

## Potential Attack Vectors

### Data Integrity Attacks
1. **Pointer Manipulation:** Modify returned pointers to corrupt internal data
2. **Hash Collision:** Exploit hash collisions to bypass duplicate detection
3. **Memory Exhaustion:** Add excessive relations to exhaust memory

### Information Disclosure
1. **Structure Enumeration:** Use lookup methods to enumerate table structure
2. **Internal State Access:** Access internal data through returned pointers
3. **Relation Discovery:** Discover table relationships through method calls

## Recommendations

### Immediate Actions
1. **Add Input Validation:** Validate all input parameters for length and content
2. **Return Copies:** Return copies instead of pointers to internal data
3. **Add Bounds Checking:** Implement limits on relation counts
4. **Improve Duplicate Detection:** Add secondary validation beyond hash comparison

### Enhanced Security Implementation

```go
package table_info

import (
    "fmt"
    "strings"
    "unicode/utf8"
    
    "github.com/artpar/api2go/v2"
    "github.com/daptin/daptin/server/auth"
    "github.com/daptin/daptin/server/columns"
    "github.com/daptin/daptin/server/fsm"
)

const (
    MaxNameLength = 255
    MaxRelationCount = 1000
    MaxColumnCount = 1000
)

// TableRelation extends api2go.TableRelation with validation
type TableRelation struct {
    api2go.TableRelation
    OnDelete string
}

// Validate validates the table relation
func (tr *TableRelation) Validate() error {
    if len(tr.Subject) == 0 || len(tr.Object) == 0 {
        return fmt.Errorf("subject and object cannot be empty")
    }
    
    if len(tr.Subject) > MaxNameLength || len(tr.Object) > MaxNameLength {
        return fmt.Errorf("subject and object names too long")
    }
    
    validOnDeleteActions := []string{"cascade", "restrict", "set_null", "set_default", "no_action"}
    if tr.OnDelete != "" {
        valid := false
        for _, action := range validOnDeleteActions {
            if strings.ToLower(tr.OnDelete) == action {
                valid = true
                break
            }
        }
        if !valid {
            return fmt.Errorf("invalid OnDelete action: %s", tr.OnDelete)
        }
    }
    
    return nil
}

// TableInfo provides secure table metadata management
type TableInfo struct {
    TableName              string `db:"table_name"`
    TableId                int
    TableDescription       string
    DefaultPermission      auth.AuthPermission `db:"default_permission"`
    Columns                []api2go.ColumnInfo
    StateMachines          []fsm.LoopbookFsmDescription
    Relations              []api2go.TableRelation
    IsTopLevel             bool `db:"is_top_level"`
    Permission             auth.AuthPermission
    UserId                 uint64              `db:"user_account_id"`
    IsHidden               bool                `db:"is_hidden"`
    IsJoinTable            bool                `db:"is_join_table"`
    IsStateTrackingEnabled bool                `db:"is_state_tracking_enabled"`
    IsAuditEnabled         bool                `db:"is_audit_enabled"`
    TranslationsEnabled    bool                `db:"translation_enabled"`
    DefaultGroups          []string            `db:"default_groups"`
    DefaultRelations       map[string][]string `db:"default_relations"`
    Validations            []columns.ColumnTag
    Conformations          []columns.ColumnTag
    DefaultOrder           string
    Icon                   string
    CompositeKeys          [][]string
}

// validateName validates table/column/relation names
func validateName(name string) error {
    if len(name) == 0 {
        return fmt.Errorf("name cannot be empty")
    }
    
    if len(name) > MaxNameLength {
        return fmt.Errorf("name too long: %d characters", len(name))
    }
    
    if !utf8.ValidString(name) {
        return fmt.Errorf("name contains invalid UTF-8")
    }
    
    // Check for SQL injection characters
    dangerousChars := []string{";", "--", "/*", "*/", "'", "\"", "\\", "\x00"}
    for _, dangerous := range dangerousChars {
        if strings.Contains(name, dangerous) {
            return fmt.Errorf("name contains dangerous characters")
        }
    }
    
    return nil
}

// GetColumnByNameSecure provides secure column lookup with validation
func (ti *TableInfo) GetColumnByNameSecure(name string) (*api2go.ColumnInfo, bool, error) {
    if err := validateName(name); err != nil {
        return nil, false, fmt.Errorf("invalid column name: %v", err)
    }
    
    if ti.Columns == nil {
        return nil, false, nil
    }
    
    if len(ti.Columns) > MaxColumnCount {
        return nil, false, fmt.Errorf("too many columns to search")
    }
    
    for _, col := range ti.Columns {
        if col.Name == name || col.ColumnName == name {
            // Return a copy to prevent external modification
            colCopy := api2go.ColumnInfo{
                Name:         col.Name,
                ColumnName:   col.ColumnName,
                ColumnType:   col.ColumnType,
                DataType:     col.DataType,
                DefaultValue: col.DefaultValue,
                IsNullable:   col.IsNullable,
                IsUnique:     col.IsUnique,
                IsPrimaryKey: col.IsPrimaryKey,
                IsForeignKey: col.IsForeignKey,
                ExcludeFromApi: col.ExcludeFromApi,
                Tags:         make([]string, len(col.Tags)),
            }
            copy(colCopy.Tags, col.Tags)
            return &colCopy, true, nil
        }
    }
    
    return nil, false, nil
}

// GetColumnByName maintains backward compatibility
func (ti *TableInfo) GetColumnByName(name string) (*api2go.ColumnInfo, bool) {
    col, found, err := ti.GetColumnByNameSecure(name)
    if err != nil {
        return nil, false
    }
    return col, found
}

// GetRelationByNameSecure provides secure relation lookup with validation
func (ti *TableInfo) GetRelationByNameSecure(name string) (*api2go.TableRelation, bool, error) {
    if err := validateName(name); err != nil {
        return nil, false, fmt.Errorf("invalid relation name: %v", err)
    }
    
    if ti.Relations == nil {
        return nil, false, nil
    }
    
    if len(ti.Relations) > MaxRelationCount {
        return nil, false, fmt.Errorf("too many relations to search")
    }
    
    for _, relation := range ti.Relations {
        if relation.SubjectName == name || relation.ObjectName == name {
            // Return a copy to prevent external modification
            relationCopy := api2go.TableRelation{
                Subject:       relation.Subject,
                SubjectName:   relation.SubjectName,
                Object:        relation.Object,
                ObjectName:    relation.ObjectName,
                Relation:      relation.Relation,
                SubjectColumn: relation.SubjectColumn,
                ObjectColumn:  relation.ObjectColumn,
            }
            return &relationCopy, true, nil
        }
    }
    
    return nil, false, nil
}

// GetRelationByName maintains backward compatibility
func (ti *TableInfo) GetRelationByName(name string) (*api2go.TableRelation, bool) {
    relation, found, err := ti.GetRelationByNameSecure(name)
    if err != nil {
        return nil, false
    }
    return relation, found
}

// AddRelationSecure provides secure relation addition with validation
func (ti *TableInfo) AddRelationSecure(relations ...api2go.TableRelation) error {
    if ti.Relations == nil {
        ti.Relations = make([]api2go.TableRelation, 0)
    }
    
    // Check total relation count limit
    if len(ti.Relations)+len(relations) > MaxRelationCount {
        return fmt.Errorf("too many relations: current=%d, adding=%d, maximum=%d", 
            len(ti.Relations), len(relations), MaxRelationCount)
    }
    
    for _, relation := range relations {
        // Validate relation
        if err := validateRelation(&relation); err != nil {
            return fmt.Errorf("invalid relation: %v", err)
        }
        
        exists := false
        hash := relation.Hash()
        
        // Check for duplicates using both hash and content comparison
        for _, existingRelation := range ti.Relations {
            if existingRelation.Hash() == hash {
                // Secondary validation to handle hash collisions
                if relationsEqual(&existingRelation, &relation) {
                    exists = true
                    break
                }
            }
        }
        
        if !exists {
            ti.Relations = append(ti.Relations, relation)
        }
    }
    
    return nil
}

// validateRelation validates relation structure
func validateRelation(relation *api2go.TableRelation) error {
    if relation == nil {
        return fmt.Errorf("relation cannot be nil")
    }
    
    if err := validateName(relation.Subject); err != nil {
        return fmt.Errorf("invalid subject: %v", err)
    }
    
    if err := validateName(relation.Object); err != nil {
        return fmt.Errorf("invalid object: %v", err)
    }
    
    if err := validateName(relation.SubjectName); err != nil {
        return fmt.Errorf("invalid subject name: %v", err)
    }
    
    if err := validateName(relation.ObjectName); err != nil {
        return fmt.Errorf("invalid object name: %v", err)
    }
    
    return nil
}

// relationsEqual provides secondary validation for hash collision handling
func relationsEqual(r1, r2 *api2go.TableRelation) bool {
    return r1.Subject == r2.Subject &&
           r1.SubjectName == r2.SubjectName &&
           r1.Object == r2.Object &&
           r1.ObjectName == r2.ObjectName &&
           r1.Relation == r2.Relation &&
           r1.SubjectColumn == r2.SubjectColumn &&
           r1.ObjectColumn == r2.ObjectColumn
}

// AddRelation maintains backward compatibility
func (ti *TableInfo) AddRelation(relations ...api2go.TableRelation) {
    err := ti.AddRelationSecure(relations...)
    if err != nil {
        // Log error but maintain backward compatibility
        // In a production system, this should be handled appropriately
        return
    }
}

// Validate validates the entire TableInfo structure
func (ti *TableInfo) Validate() error {
    if err := validateName(ti.TableName); err != nil {
        return fmt.Errorf("invalid table name: %v", err)
    }
    
    if len(ti.Columns) > MaxColumnCount {
        return fmt.Errorf("too many columns: %d", len(ti.Columns))
    }
    
    if len(ti.Relations) > MaxRelationCount {
        return fmt.Errorf("too many relations: %d", len(ti.Relations))
    }
    
    // Validate all relations
    for i, relation := range ti.Relations {
        if err := validateRelation(&relation); err != nil {
            return fmt.Errorf("invalid relation at index %d: %v", i, err)
        }
    }
    
    return nil
}

// GetStats returns table statistics for monitoring
func (ti *TableInfo) GetStats() map[string]interface{} {
    return map[string]interface{}{
        "table_name":     ti.TableName,
        "column_count":   len(ti.Columns),
        "relation_count": len(ti.Relations),
        "is_hidden":      ti.IsHidden,
        "is_join_table":  ti.IsJoinTable,
        "audit_enabled":  ti.IsAuditEnabled,
    }
}
```

### Long-term Improvements
1. **Schema Validation:** Implement comprehensive schema validation
2. **Relationship Integrity:** Add referential integrity checking
3. **Performance Optimization:** Optimize lookups with indexing
4. **Concurrency Safety:** Add thread-safe operations for concurrent access
5. **Audit Logging:** Log all table structure modifications

## Edge Cases Identified

1. **Empty Table Structures:** Tables with no columns or relations
2. **Large Table Schemas:** Tables with thousands of columns/relations
3. **Circular Relations:** Self-referencing or circular relationship patterns
4. **Name Collisions:** Columns and relations with identical names
5. **Unicode Names:** Table/column names with unicode characters
6. **Case Sensitivity:** Name matching across different case patterns
7. **Memory Pressure:** Operations under high memory pressure
8. **Concurrent Modifications:** Simultaneous modifications to table structure

## Security Best Practices Adherence

✅ **Good Practices:**
1. Simple data structure design minimizes attack surface
2. Immutable-style operations with minimal side effects
3. Clear separation between data and operations

⚠️ **Areas for Improvement:**
1. Missing input validation for all methods
2. Pointer returns allowing external modification
3. No bounds checking on collections
4. Hash collision handling incomplete

## Critical Issues Summary

1. **Missing Input Validation:** No validation of input parameters in lookup methods
2. **Pointer Return Security:** Returns pointers to internal data allowing external modification
3. **Hash Collision Handling:** Incomplete duplicate detection relying only on hash comparison
4. **Slice Management Issues:** No bounds checking on relation additions

---
**Analysis Date:** 2025-01-27  
**Analyst:** Claude  
**Priority:** LOW - Data structure with minor security considerations requiring input validation improvements