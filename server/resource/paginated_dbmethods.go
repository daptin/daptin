package resource

import (
	"fmt"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

// PaginatedResultCallback is a function that processes each batch of results
type PaginatedResultCallback func([]map[string]interface{}) error

// GetAllRawObjectsWithPaginationAndTransaction fetches objects in batches to avoid memory issues
func (dbResource *DbResource) GetAllRawObjectsWithPaginationAndTransaction(
	typeName string,
	pageSize int,
	transaction *sqlx.Tx,
	callback PaginatedResultCallback, limit int) error {
	log.Infof("Starting paginated export for table [%s] with page size %d", typeName, pageSize)

	if pageSize <= 0 {
		pageSize = 1000 // Default page size
	}

	offset := 0
	hasMore := true

	for hasMore {
		// Build query with pagination
		s, q, err := statementbuilder.Squirrel.
			Select(goqu.L("*")).
			Prepared(true).
			From(typeName).
			Limit(uint(pageSize)).
			Offset(uint(offset)).
			ToSQL()

		if err != nil {
			return fmt.Errorf("failed to build paginated query: %v", err)
		}

		// Prepare statement
		stmt, err := transaction.Preparex(s)
		if err != nil {
			return fmt.Errorf("failed to prepare paginated statement: %v", err)
		}

		// Execute query
		rows, err := stmt.Queryx(q...)
		if err != nil {
			stmt.Close()
			return fmt.Errorf("failed to execute paginated query: %v", err)
		}

		// Process results
		results, err := RowsToMap(rows, typeName)
		rows.Close()
		stmt.Close()

		if err != nil {
			return fmt.Errorf("failed to convert rows to map: %v", err)
		}

		// Check if we have more results
		if len(results) < pageSize {
			hasMore = false
		}

		// Process this batch via callback if we have results
		if len(results) > 0 {
			if err := callback(results); err != nil {
				return fmt.Errorf("callback processing error: %v", err)
			}
		} else {
			hasMore = false
		}

		// Move to next page
		offset += pageSize
		if limit > -1 && offset >= limit {
			break
		}
	}

	return nil
}

// StreamingExportWriter interface for different export format writers
type StreamingExportWriter interface {
	// Initialize prepares the writer for streaming
	Initialize(tableNames []string, includeHeaders bool, selectedColumns map[string][]string) error

	// WriteTable writes a table name header if needed
	WriteTable(tableName string) error

	// WriteHeaders writes column headers for a table
	WriteHeaders(tableName string, columns []string) error

	// WriteRows writes a batch of rows
	WriteRows(tableName string, rows []map[string]interface{}) error

	// Finalize completes the export and returns the final content
	Finalize() ([]byte, error)
}
