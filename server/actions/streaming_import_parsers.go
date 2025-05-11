package actions

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"strings"

	"github.com/artpar/xlsx/v2"
	log "github.com/sirupsen/logrus"
)

// ImportFormat represents supported import formats
type ImportFormat string

const (
	// ImportFormatJSON imports data from JSON
	ImportFormatJSON ImportFormat = "json"
	// ImportFormatCSV imports data from CSV
	ImportFormatCSV ImportFormat = "csv"
	// ImportFormatXLSX imports data from Excel spreadsheet
	ImportFormatXLSX ImportFormat = "xlsx"
)

// StreamingImportParser defines the interface for import parsers
type StreamingImportParser interface {
	// Initialize prepares the parser with the file content
	Initialize(fileContent []byte) error

	// GetTableNames returns the names of tables found in the import file
	GetTableNames() ([]string, error)

	// GetColumnsForTable returns the column names for a specific table
	GetColumnsForTable(tableName string) ([]string, error)

	// ParseRows processes rows for a specific table and calls the handler for each batch
	ParseRows(tableName string, batchSize int, handler func(rows []map[string]interface{}) error) error

	// GetFormat returns the format of this parser
	GetFormat() ImportFormat
}

// StreamingJSONParser implements JSON import parsing
type StreamingJSONParser struct {
	data       map[string][]map[string]interface{}
	tableNames []string
}

// Initialize prepares the JSON parser
func (p *StreamingJSONParser) Initialize(fileContent []byte) error {
	log.Debugf("Initializing JSON parser with %d bytes", len(fileContent))
	// Parse the JSON content
	var jsonData map[string]interface{}
	err := json.Unmarshal(fileContent, &jsonData)
	if err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Initialize data map
	p.data = make(map[string][]map[string]interface{})
	p.tableNames = make([]string, 0)

	// Process each table in the JSON
	for tableName, tableData := range jsonData {
		// Each table should contain an array of objects
		tableArray, ok := tableData.([]interface{})
		if !ok {
			return fmt.Errorf("invalid JSON format for table '%s': expected array", tableName)
		}

		// Convert each row to a map
		tableRows := make([]map[string]interface{}, 0, len(tableArray))
		for _, rowData := range tableArray {
			row, ok := rowData.(map[string]interface{})
			if !ok {
				return fmt.Errorf("invalid JSON format for table '%s': expected object in array", tableName)
			}
			tableRows = append(tableRows, row)
		}

		p.data[tableName] = tableRows
		p.tableNames = append(p.tableNames, tableName)
	}

	return nil
}

// GetTableNames returns the names of tables in the JSON
func (p *StreamingJSONParser) GetTableNames() ([]string, error) {
	if len(p.tableNames) == 0 {
		return nil, errors.New("no tables found in JSON")
	}
	return p.tableNames, nil
}

// GetColumnsForTable returns the column names for a specific table
func (p *StreamingJSONParser) GetColumnsForTable(tableName string) ([]string, error) {
	tableData, ok := p.data[tableName]
	if !ok || len(tableData) == 0 {
		return nil, fmt.Errorf("[101] table '%s' not found or empty", tableName)
	}

	// Get columns from the first row
	firstRow := tableData[0]
	columns := make([]string, 0, len(firstRow))
	for col := range firstRow {
		columns = append(columns, col)
	}

	return columns, nil
}

// ParseRows processes rows for a specific table
func (p *StreamingJSONParser) ParseRows(tableName string, batchSize int, handler func(rows []map[string]interface{}) error) error {
	tableData, ok := p.data[tableName]
	if !ok {
		return fmt.Errorf("[118] table '%s' not found", tableName)
	}

	// Process in batches
	for i := 0; i < len(tableData); i += batchSize {
		end := i + batchSize
		if end > len(tableData) {
			end = len(tableData)
		}

		batch := tableData[i:end]
		if err := handler(batch); err != nil {
			return err
		}
	}

	return nil
}

// GetFormat returns the format of this parser
func (p *StreamingJSONParser) GetFormat() ImportFormat {
	return ImportFormatJSON
}

// StreamingCSVParser implements CSV import parsing
type StreamingCSVParser struct {
	content        []byte
	headers        []string
	rows           [][]string
	hasTableHeader bool
	tableMap       map[string][][]string // Maps table names to their rows
}

// Initialize prepares the CSV parser
func (p *StreamingCSVParser) Initialize(fileContent []byte) error {
	log.Debugf("Initializing CSV parser with %d bytes", len(fileContent))
	reader := csv.NewReader(bytes.NewReader(fileContent))

	// Read all CSV records
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to parse CSV: %w", err)
	}

	if len(records) < 1 {
		return errors.New("CSV file is empty")
	}

	p.rows = records
	p.tableMap = make(map[string][][]string)

	// Check if the first row is a table header (starts with "Table:" or similar)
	p.hasTableHeader = false
	currentTable := "default"
	tableRows := make([][]string, 0)

	for i, row := range records {
		if len(row) > 0 && strings.HasPrefix(strings.ToLower(row[0]), "table:") {
			// This is a table header row
			if i > 0 && len(tableRows) > 0 {
				// Save the previous table's rows
				p.tableMap[currentTable] = tableRows
			}

			// Extract the new table name
			currentTable = strings.TrimSpace(strings.TrimPrefix(row[0], "Table:"))
			if currentTable == "" {
				currentTable = fmt.Sprintf("table_%d", i)
			}

			p.hasTableHeader = true
			tableRows = make([][]string, 0)
		} else if i == 0 && !p.hasTableHeader {
			// First row is headers if no table markers
			p.headers = row
			tableRows = append(tableRows, row) // Include headers in the rows
		} else {
			// Regular data row
			tableRows = append(tableRows, row)
		}
	}

	// Save the last table
	if len(tableRows) > 0 {
		p.tableMap[currentTable] = tableRows
	}

	return nil
}

// GetTableNames returns the names of tables in the CSV
func (p *StreamingCSVParser) GetTableNames() ([]string, error) {
	if !p.hasTableHeader {
		// If no table headers, we have only one default table
		return []string{"default"}, nil
	}

	tableNames := make([]string, 0, len(p.tableMap))
	for name := range p.tableMap {
		tableNames = append(tableNames, name)
	}

	if len(tableNames) == 0 {
		return nil, errors.New("no tables found in CSV")
	}

	return tableNames, nil
}

// GetColumnsForTable returns the column names for a specific table
func (p *StreamingCSVParser) GetColumnsForTable(tableName string) ([]string, error) {
	tableRows, ok := p.tableMap[tableName]
	if !ok || len(tableRows) == 0 {
		return nil, fmt.Errorf("[231] table '%s' not found or empty", tableName)
	}

	// First row contains headers
	return tableRows[0], nil
}

// ParseRows processes rows for a specific table
func (p *StreamingCSVParser) ParseRows(tableName string, batchSize int, handler func(rows []map[string]interface{}) error) error {
	tableRows, ok := p.tableMap[tableName]
	if !ok {
		return fmt.Errorf("[242] table '%s' not found", tableName)
	}

	if len(tableRows) <= 1 {
		// Only headers, no data
		return nil
	}

	headers := tableRows[0]
	dataRows := tableRows[1:] // Skip headers

	// Process in batches
	for i := 0; i < len(dataRows); i += batchSize {
		end := i + batchSize
		if end > len(dataRows) {
			end = len(dataRows)
		}

		batch := make([]map[string]interface{}, 0, end-i)
		for j := i; j < end; j++ {
			row := dataRows[j]
			rowMap := make(map[string]interface{})

			// Map each column value to its header
			for k, header := range headers {
				if k < len(row) {
					rowMap[header] = row[k]
				}
			}

			batch = append(batch, rowMap)
		}

		if err := handler(batch); err != nil {
			return err
		}
	}

	return nil
}

// GetFormat returns the format of this parser
func (p *StreamingCSVParser) GetFormat() ImportFormat {
	return ImportFormatCSV
}

// StreamingXLSXParser implements Excel spreadsheet import parsing
type StreamingXLSXParser struct {
	file     *xlsx.File
	sheetMap map[string]*xlsx.Sheet
	tableMap map[string][][]string
}

// Initialize prepares the XLSX parser
func (p *StreamingXLSXParser) Initialize(fileContent []byte) error {
	log.Debugf("Initializing XLSX parser with %d bytes", len(fileContent))
	var err error

	// Parse the XLSX content
	p.file, err = xlsx.OpenBinary(fileContent)
	if err != nil {
		return fmt.Errorf("failed to parse XLSX: %w", err)
	}

	if len(p.file.Sheets) == 0 {
		return errors.New("XLSX file has no sheets")
	}

	p.sheetMap = make(map[string]*xlsx.Sheet)
	p.tableMap = make(map[string][][]string)

	// Each sheet is treated as a separate table
	for _, sheet := range p.file.Sheets {
		p.sheetMap[sheet.Name] = sheet

		// Convert sheet data to string arrays
		rows := make([][]string, 0)
		for i := 0; i < sheet.MaxRow; i++ {
			row, _ := sheet.Row(i)
			if row == nil || row.Sheet == nil {
				continue // Skip empty rows
			}

			stringRow := make([]string, 0, sheet.MaxCol)
			for j := 0; j < sheet.MaxCol; j++ {
				cell := row.GetCell(j)
				stringRow = append(stringRow, cell.String())
			}
			rows = append(rows, stringRow)
		}

		if len(rows) > 0 {
			p.tableMap[sheet.Name] = rows
		}
	}

	return nil
}

// GetTableNames returns the names of sheets in the XLSX
func (p *StreamingXLSXParser) GetTableNames() ([]string, error) {
	tableNames := make([]string, 0, len(p.tableMap))
	for name := range p.tableMap {
		tableNames = append(tableNames, name)
	}

	if len(tableNames) == 0 {
		return nil, errors.New("no tables found in XLSX")
	}

	return tableNames, nil
}

// GetColumnsForTable returns the column names for a specific sheet
func (p *StreamingXLSXParser) GetColumnsForTable(tableName string) ([]string, error) {
	tableRows, ok := p.tableMap[tableName]
	if !ok || len(tableRows) == 0 {
		return nil, fmt.Errorf("[359] table '%s' not found or empty", tableName)
	}

	// First row contains headers
	return tableRows[0], nil
}

// ParseRows processes rows for a specific sheet
func (p *StreamingXLSXParser) ParseRows(tableName string, batchSize int, handler func(rows []map[string]interface{}) error) error {
	tableRows, ok := p.tableMap[tableName]
	if !ok {
		return fmt.Errorf("[370] table '%s' not found", tableName)
	}

	if len(tableRows) <= 1 {
		// Only headers, no data
		return nil
	}

	headers := tableRows[0]
	dataRows := tableRows[1:] // Skip headers

	// Process in batches
	for i := 0; i < len(dataRows); i += batchSize {
		end := i + batchSize
		if end > len(dataRows) {
			end = len(dataRows)
		}

		batch := make([]map[string]interface{}, 0, end-i)
		for j := i; j < end; j++ {
			row := dataRows[j]
			rowMap := make(map[string]interface{})

			// Map each column value to its header
			for k, header := range headers {
				if k < len(row) && header != "" {
					rowMap[header] = row[k]
				}
			}

			batch = append(batch, rowMap)
		}

		if err := handler(batch); err != nil {
			return err
		}
	}

	return nil
}

// GetFormat returns the format of this parser
func (p *StreamingXLSXParser) GetFormat() ImportFormat {
	return ImportFormatXLSX
}

// Helper function to avoid json package name collision
func jsonUnmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// DetectFileFormat attempts to determine the format of the import file
func DetectFileFormat(fileContent []byte, fileName string) ImportFormat {
	// Check by file extension first
	lowerFileName := strings.ToLower(fileName)
	if strings.HasSuffix(lowerFileName, ".json") {
		return ImportFormatJSON
	} else if strings.HasSuffix(lowerFileName, ".csv") {
		return ImportFormatCSV
	} else if strings.HasSuffix(lowerFileName, ".xlsx") || strings.HasSuffix(lowerFileName, ".xls") {
		return ImportFormatXLSX
	}

	// Try to detect by content
	if len(fileContent) > 0 {
		// Check for JSON format (starts with { or [)
		trimmed := bytes.TrimSpace(fileContent)
		if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
			return ImportFormatJSON
		}

		// Check for CSV (contains commas and newlines)
		if bytes.Contains(fileContent, []byte{','}) && bytes.Contains(fileContent, []byte{'\n'}) {
			return ImportFormatCSV
		}

		// XLSX is a binary format, harder to detect by content inspection
		// Excel files start with PK (zip file signature)
		if len(fileContent) > 2 && fileContent[0] == 'P' && fileContent[1] == 'K' {
			return ImportFormatXLSX
		}
	}

	// Default to JSON if we can't determine
	return ImportFormatJSON
}

// CreateStreamingImportParser creates the appropriate parser based on format
func CreateStreamingImportParser(format ImportFormat) (StreamingImportParser, error) {
	switch format {
	case ImportFormatJSON:
		return &StreamingJSONParser{}, nil
	case ImportFormatCSV:
		return &StreamingCSVParser{}, nil
	case ImportFormatXLSX:
		return &StreamingXLSXParser{}, nil
	default:
		return nil, fmt.Errorf("unsupported import format: %s", format)
	}
}
