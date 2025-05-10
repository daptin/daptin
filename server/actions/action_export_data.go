package actions

import (
	"bytes"
	"encoding/base64"
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/artpar/api2go"
	"github.com/artpar/xlsx/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
	"github.com/jung-kurt/gofpdf"
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
	// FormatDOCX exports data as DOCX (Word document)
	FormatDOCX ExportFormat = "docx"
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

	// Get specific columns to export if specified
	var selectedColumns []string
	columnsVal, ok := inFields["columns"]
	if ok && columnsVal != nil {
		if columnsArr, ok := columnsVal.([]interface{}); ok {
			for _, col := range columnsArr {
				if colStr, ok := col.(string); ok {
					selectedColumns = append(selectedColumns, colStr)
				}
			}
		}
		if columnsString, ok := columnsVal.(string); ok {
			colArr := make([]interface{}, 0)
			err := json.Unmarshal([]byte(columnsString), &colArr)
			if err != nil {
				for _, col := range colArr {
					if colStr, ok := col.(string); ok {
						selectedColumns = append(selectedColumns, colStr)
					}
				}
			}
		}
	}

	// Collect data to export
	result := make(map[string][]map[string]interface{})
	if tableOk && tableName != nil {
		tableNameStr := tableName.(string)
		log.Printf("Export data for table: %v", tableNameStr)

		objects, err := d.cruds[tableNameStr].GetAllRawObjectsWithTransaction(tableNameStr, transaction)
		if err != nil {
			log.Errorf("Failed to get all objects of type [%v] : %v", tableNameStr, err)
		}

		result[tableNameStr] = objects
		finalName = tableNameStr
	} else {
		for _, tableInfo := range d.cmsConfig.Tables {
			data, err := d.cruds[tableInfo.TableName].GetAllRawObjectsWithTransaction(tableInfo.TableName, transaction)
			if err != nil {
				log.Errorf("Failed to export objects of type [%v]: %v", tableInfo.TableName, err)
				continue
			}
			result[tableInfo.TableName] = data
		}
	}

	// Export data in the requested format
	var content []byte
	var contentType string
	var fileExtension string
	var err error

	switch format {
	case FormatJSON:
		content, err = json.Marshal(result)
		contentType = "application/json"
		fileExtension = "json"
	case FormatCSV:
		content, err = exportAsCSV(result, includeHeaders, selectedColumns)
		contentType = "text/csv"
		fileExtension = "csv"
	case FormatXLSX:
		content, err = exportAsXLSX(result, includeHeaders, selectedColumns)
		contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		fileExtension = "xlsx"
	case FormatPDF:
		content, err = exportAsPDF(result, includeHeaders, selectedColumns)
		contentType = "application/pdf"
		fileExtension = "pdf"
	case FormatDOCX:
		content, err = exportAsDOCX(result, includeHeaders, selectedColumns)
		contentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		fileExtension = "docx"
	default:
		// Default to JSON if format is not recognized
		content, err = json.Marshal(result)
		contentType = "application/json"
		fileExtension = "json"
	}

	if err != nil {
		log.Errorf("Failed to export data as %s: %v", format, err)
		// Fallback to JSON if the requested format fails
		content, _ = exportAsJSON(result)
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

// exportAsJSON exports data as JSON
func exportAsJSON(data map[string][]map[string]interface{}) ([]byte, error) {
	return json.MarshalIndent(data, "", "  ")
}

// exportAsCSV exports data as CSV
func exportAsCSV(data map[string][]map[string]interface{}, includeHeaders bool, selectedColumns []string) ([]byte, error) {
	buffer := &bytes.Buffer{}
	writer := csv.NewWriter(buffer)

	// Process each table
	for tableName, records := range data {
		// Skip if no records
		if len(records) == 0 {
			continue
		}

		// Write table name as a header row if multiple tables
		if len(data) > 1 {
			writer.Write([]string{fmt.Sprintf("Table: %s", tableName)})
		}

		// Determine columns to export
		var columns []string
		if len(selectedColumns) > 0 {
			columns = selectedColumns
		} else {
			// Extract all column names from the first record
			for key := range records[0] {
				columns = append(columns, key)
			}
		}

		// Write header row if requested
		if includeHeaders {
			writer.Write(columns)
		}

		// Write data rows
		for _, record := range records {
			row := make([]string, len(columns))
			for i, column := range columns {
				if val, ok := record[column]; ok {
					row[i] = fmt.Sprintf("%v", val)
				}
			}
			writer.Write(row)
		}

		// Add a blank line between tables
		if len(data) > 1 {
			writer.Write([]string{})
		}
	}

	writer.Flush()
	return buffer.Bytes(), nil
}

// exportAsXLSX exports data as Excel spreadsheet
func exportAsXLSX(data map[string][]map[string]interface{}, includeHeaders bool, selectedColumns []string) ([]byte, error) {
	// Create a new Excel file
	file := xlsx.NewFile()

	// Process each table
	for tableName, records := range data {
		// Skip if no records
		if len(records) == 0 {
			continue
		}

		// Create a new sheet for each table
		sheet, err := file.AddSheet(tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to create sheet for table %s: %v", tableName, err)
		}

		// Determine columns to export
		var columns []string
		if len(selectedColumns) > 0 {
			columns = selectedColumns
		} else {
			// Extract all column names from the first record
			for key := range records[0] {
				columns = append(columns, key)
			}
		}

		// Write header row if requested
		if includeHeaders {
			headerRow := sheet.AddRow()
			for _, column := range columns {
				cell := headerRow.AddCell()
				cell.Value = column
			}
		}

		// Write data rows
		for _, record := range records {
			row := sheet.AddRow()
			for _, column := range columns {
				cell := row.AddCell()
				if val, ok := record[column]; ok {
					cell.Value = fmt.Sprintf("%v", val)
				}
			}
		}
	}

	// Write the Excel file to a buffer
	buffer := &bytes.Buffer{}
	err := file.Write(buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to write Excel file: %v", err)
	}

	return buffer.Bytes(), nil
}

// exportAsPDF exports data as PDF
func exportAsPDF(data map[string][]map[string]interface{}, includeHeaders bool, selectedColumns []string) ([]byte, error) {
	// Create a new PDF
	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.SetFont("Arial", "", 10)

	// Process each table
	for tableName, records := range data {
		// Skip if no records
		if len(records) == 0 {
			continue
		}

		// Add a new page for each table
		pdf.AddPage()

		// Add table name as a header
		pdf.SetFont("Arial", "B", 16)
		pdf.Cell(40, 10, fmt.Sprintf("Table: %s", tableName))
		pdf.Ln(15)
		pdf.SetFont("Arial", "", 10)

		// Determine columns to export
		var columns []string
		if len(selectedColumns) > 0 {
			columns = selectedColumns
		} else {
			// Extract all column names from the first record
			for key := range records[0] {
				columns = append(columns, key)
			}
		}

		// Calculate column width
		pageWidth := 270.0 // A4 landscape width in mm (approx)
		colWidth := pageWidth / 4

		// Write header row if requested
		if includeHeaders {
			pdf.SetFillColor(200, 200, 200)
		}

		// Write data rows
		for _, record := range records {
			for _, column := range columns {
				var value string
				if val, ok := record[column]; ok {
					value = fmt.Sprintf("%v", val)
				}
				pdf.SetFont("Arial", "B", 10)
				pdf.Cell(colWidth, 10, column)
				pdf.SetFont("Arial", "", 10)
				pdf.Cell(colWidth, 10, value)
				pdf.Ln(-1)
			}
		}
	}

	// Write the PDF to a buffer
	buffer := &bytes.Buffer{}
	err := pdf.Output(buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to write PDF: %v", err)
	}

	return buffer.Bytes(), nil
}

// exportAsDOCX exports data as DOCX (Word document)
// Note: This is a simplified implementation using basic HTML conversion
// For a more robust solution, consider using a dedicated DOCX library
func exportAsDOCX(data map[string][]map[string]interface{}, includeHeaders bool, selectedColumns []string) ([]byte, error) {
	// For DOCX, we'll create a simple HTML representation and convert it
	// This is a simplified approach - for production use, consider a dedicated DOCX library

	// First, generate HTML content
	buffer := &bytes.Buffer{}

	// Start HTML document
	buffer.WriteString("<html><body>")

	// Process each table
	for tableName, records := range data {
		// Skip if no records
		if len(records) == 0 {
			continue
		}

		// Add table name as a header
		buffer.WriteString(fmt.Sprintf("<h1>Table: %s</h1>", tableName))

		// Determine columns to export
		var columns []string
		if len(selectedColumns) > 0 {
			columns = selectedColumns
		} else {
			// Extract all column names from the first record
			for key := range records[0] {
				columns = append(columns, key)
			}
		}

		// Start table
		buffer.WriteString("<table border='1' cellpadding='3'>")

		// Write header row if requested
		if includeHeaders {
			buffer.WriteString("<tr>")
			for _, column := range columns {
				buffer.WriteString(fmt.Sprintf("<th>%s</th>", column))
			}
			buffer.WriteString("</tr>")
		}

		// Write data rows
		for _, record := range records {
			buffer.WriteString("<tr>")
			for _, column := range columns {
				var value string
				if val, ok := record[column]; ok {
					value = fmt.Sprintf("%v", val)
				}
				buffer.WriteString(fmt.Sprintf("<td>%s</td>", value))
			}
			buffer.WriteString("</tr>")
		}

		// End table
		buffer.WriteString("</table><br/>")
	}

	// End HTML document
	buffer.WriteString("</body></html>")

	// For a real implementation, you would convert this HTML to DOCX
	// Here we're just returning the HTML as a placeholder
	// In a production environment, use a proper DOCX library

	return buffer.Bytes(), nil
}

// NewExportDataPerformer creates a new instance of the export data performer
func NewExportDataPerformer(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {
	handler := exportDataPerformer{
		cmsConfig: initConfig,
		cruds:     cruds,
	}

	return &handler, nil
}
