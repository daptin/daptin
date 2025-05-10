package actions

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

// ExportFormat represents supported export formats
type ExportFormat string

const (
	// FormatJSON exports data as JSON
	FormatJSON ExportFormat = "json"
	// FormatCSV exports data as CSV
	FormatCSV ExportFormat = "csv"
	// FormatXLSX exports data as Excel spreadsheet
	FormatXLSX ExportFormat = "xlsx"
	// FormatPDF exports data as PDF
	FormatPDF ExportFormat = "pdf"
	// FormatHTML exports data as HTML (HTML document)
	FormatHTML ExportFormat = "html"
)

// exportDataPerformer handles data export in various formats
type exportDataPerformer struct {
	cmsConfig *resource.CmsConfig
	cruds     map[string]*resource.DbResource
}

// Name returns the name of this action
func (d *exportDataPerformer) Name() string {
	return "__data_export"
}

// DoAction performs the export action
func (d *exportDataPerformer) DoAction(request actionresponse.Outcome, inFields map[string]interface{},
	transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	responses := make([]actionresponse.ActionResponse, 0)

	// Get export format, default to JSON if not specified
	formatStr, formatOk := inFields["format"]
	format := FormatJSON
	if formatOk && formatStr != nil {
		format = ExportFormat(strings.ToLower(formatStr.(string)))
	}

	// Get table name if specified
	tableName, tableOk := inFields["table_name"]
	finalName := "complete"

	// Get additional export options
	includeHeaders := true
	if includeHeadersVal, ok := inFields["include_headers"]; ok && includeHeadersVal != nil {
		includeHeaders, _ = includeHeadersVal.(bool)
	}

	// Get page size for pagination
	pageSize := 1000 // Default page size
	if pageSizeVal, ok := inFields["page_size"]; ok && pageSizeVal != nil {
		if pageSizeInt, ok := pageSizeVal.(int); ok && pageSizeInt > 0 {
			pageSize = pageSizeInt
		}
	}

	// Get specific columns to export if specified
	selectedColumnsMap := make(map[string][]string)
	columnsVal, ok := inFields["columns"]
	if ok && columnsVal != nil {
		var selectedColumns []string

		if columnsArr, ok := columnsVal.([]interface{}); ok {
			for _, col := range columnsArr {
				if colStr, ok := col.(string); ok {
					selectedColumns = append(selectedColumns, colStr)
				}
			}
		} else if columnsString, ok := columnsVal.(string); ok {
			colArr := make([]interface{}, 0)
			err := json.Unmarshal([]byte(columnsString), &colArr)
			if err == nil {
				for _, col := range colArr {
					if colStr, ok := col.(string); ok {
						selectedColumns = append(selectedColumns, colStr)
					}
				}
			}
		}

		// If we're exporting a specific table, store its columns
		if tableOk && tableName != nil {
			tableNameStr := tableName.(string)
			selectedColumnsMap[tableNameStr] = selectedColumns
		} else {
			// For all tables, we'll set the columns later
			for _, tableInfo := range d.cmsConfig.Tables {
				selectedColumnsMap[tableInfo.TableName] = selectedColumns
			}
		}
	}

	// Create streaming writer for the selected format
	writer, err := CreateStreamingExportWriter(format)
	if err != nil {
		log.Errorf("Failed to create streaming writer: %v", err)
		return nil, nil, []error{err}
	}

	// Determine tables to export
	var tablesToExport []string
	if tableOk && tableName != nil {
		tableNameStr := tableName.(string)
		tablesToExport = []string{tableNameStr}
		finalName = tableNameStr
	} else {
		for _, tableInfo := range d.cmsConfig.Tables {
			tablesToExport = append(tablesToExport, tableInfo.TableName)
		}
	}

	// Initialize the writer
	err = writer.Initialize(tablesToExport, includeHeaders, selectedColumnsMap)
	if err != nil {
		log.Errorf("Failed to initialize streaming writer: %v", err)
		return nil, nil, []error{err}
	}

	// Process each table
	for _, currentTable := range tablesToExport {
		// Skip if we don't have access to this table
		if _, ok := d.cruds[currentTable]; !ok {
			log.Warnf("Skipping table [%s]: not accessible", currentTable)
			continue
		}

		// Notify writer of new table
		err = writer.WriteTable(currentTable)
		if err != nil {
			log.Errorf("Failed to write table header for [%s]: %v", currentTable, err)
			continue
		}

		// Determine columns for this table
		var columns []string
		if tableColumns, ok := selectedColumnsMap[currentTable]; ok && len(tableColumns) > 0 {
			columns = tableColumns
		} else {
			// Get first row to determine columns
			firstRowResult := make([]map[string]interface{}, 0)
			err = d.cruds[currentTable].GetAllRawObjectsWithPaginationAndTransaction(
				currentTable,
				1, // Just get one row to determine columns
				transaction,
				func(rows []map[string]interface{}) error {
					firstRowResult = rows
					return nil
				},
			)

			if err != nil {
				log.Errorf("Failed to get column names for [%s]: %v", currentTable, err)
				continue
			}

			if len(firstRowResult) > 0 {
				for key := range firstRowResult[0] {
					columns = append(columns, key)
				}
			}

			// Store columns for later use
			selectedColumnsMap[currentTable] = columns
		}

		// Write headers if we have columns
		if len(columns) > 0 {
			err = writer.WriteHeaders(currentTable, columns)
			if err != nil {
				log.Errorf("Failed to write headers for [%s]: %v", currentTable, err)
				continue
			}
		} else {
			log.Warnf("No columns found for table [%s], skipping", currentTable)
			continue
		}

		// Stream data in batches
		err = d.cruds[currentTable].GetAllRawObjectsWithPaginationAndTransaction(
			currentTable,
			pageSize,
			transaction,
			func(rows []map[string]interface{}) error {
				return writer.WriteRows(currentTable, rows)
			},
		)

		if err != nil {
			log.Errorf("Error streaming data for table [%s]: %v", currentTable, err)
			continue
		}
	}

	// Finalize the export
	content, err := writer.Finalize()
	if err != nil {
		log.Errorf("Failed to finalize export: %v", err)
		// Fallback to empty content
		content = []byte{}
	}

	// Determine content type and file extension
	var contentType string
	var fileExtension string

	switch format {
	case FormatJSON:
		contentType = "application/json"
		fileExtension = "json"
	case FormatCSV:
		contentType = "text/csv"
		fileExtension = "csv"
	case FormatXLSX:
		contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		fileExtension = "xlsx"
	case FormatPDF:
		contentType = "application/pdf"
		fileExtension = "pdf"
	case FormatHTML:
		contentType = "text/html"
		fileExtension = "html"
	default:
		contentType = "application/json"
		fileExtension = "json"
	}

	// Create response with the exported data
	responseAttrs := make(map[string]interface{})
	responseAttrs["content"] = base64.StdEncoding.EncodeToString(content)
	responseAttrs["name"] = fmt.Sprintf("daptin_export_%v.%s", finalName, fileExtension)
	responseAttrs["contentType"] = contentType
	responseAttrs["message"] = fmt.Sprintf("Downloading data as %s", format)

	actionResponse := resource.NewActionResponse("client.file.download", responseAttrs)
	responses = append(responses, actionResponse)

	return nil, responses, nil
}

// NewExportDataPerformer creates a new instance of the export data performer
func NewExportDataPerformer(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {
	handler := exportDataPerformer{
		cmsConfig: initConfig,
		cruds:     cruds,
	}

	return &handler, nil
}
