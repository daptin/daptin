package actions

import (
	"bytes"
	"encoding/csv"
	"fmt"

	"github.com/artpar/xlsx/v2"
	"github.com/daptin/daptin/server/resource"
	"github.com/jung-kurt/gofpdf"
	log "github.com/sirupsen/logrus"
)

// StreamingJSONWriter implements streaming JSON export
type StreamingJSONWriter struct {
	buffer       *bytes.Buffer
	isFirstRow   bool
	isFirstTable bool
}

// Initialize prepares the JSON writer
func (w *StreamingJSONWriter) Initialize(tableNames []string, includeHeaders bool, selectedColumns map[string][]string) error {
	w.buffer = &bytes.Buffer{}
	w.isFirstTable = true
	w.isFirstRow = true

	// Start the JSON object
	w.buffer.WriteString("{")
	return nil
}

// WriteTable writes a table name header
func (w *StreamingJSONWriter) WriteTable(tableName string) error {
	if !w.isFirstTable {
		w.buffer.WriteString(",")
	}
	w.isFirstTable = false
	w.isFirstRow = true

	// Write table name as key
	w.buffer.WriteString(fmt.Sprintf("\n\"%s\": [", tableName))
	return nil
}

// WriteHeaders is a no-op for JSON
func (w *StreamingJSONWriter) WriteHeaders(tableName string, columns []string) error {
	// No headers needed for JSON
	return nil
}

// WriteRows writes a batch of rows in JSON format
func (w *StreamingJSONWriter) WriteRows(tableName string, rows []map[string]interface{}) error {
	for _, row := range rows {
		if !w.isFirstRow {
			w.buffer.WriteString(",")
		}
		w.isFirstRow = false

		rowBytes, err := json.Marshal(row)
		if err != nil {
			return err
		}

		w.buffer.WriteString("\n")
		w.buffer.Write(rowBytes)
	}
	return nil
}

// Finalize completes the JSON export
func (w *StreamingJSONWriter) Finalize() ([]byte, error) {
	// Close the array and object
	w.buffer.WriteString("\n]}")
	return w.buffer.Bytes(), nil
}

// StreamingCSVWriter implements streaming CSV export
type StreamingCSVWriter struct {
	buffer          *bytes.Buffer
	writer          *csv.Writer
	includeHeaders  bool
	selectedColumns map[string][]string
	currentTable    string
	columnsWritten  map[string]bool
}

// Initialize prepares the CSV writer
func (w *StreamingCSVWriter) Initialize(tableNames []string, includeHeaders bool, selectedColumns map[string][]string) error {
	w.buffer = &bytes.Buffer{}
	w.writer = csv.NewWriter(w.buffer)
	w.includeHeaders = includeHeaders
	w.selectedColumns = selectedColumns
	w.columnsWritten = make(map[string]bool)
	return nil
}

// WriteTable writes a table name header if multiple tables
func (w *StreamingCSVWriter) WriteTable(tableName string) error {
	w.currentTable = tableName

	// Write table name as a header row if multiple tables
	if len(w.selectedColumns) > 1 {
		w.writer.Write([]string{fmt.Sprintf("Table: %s", tableName)})
		w.writer.Flush()
	}

	return nil
}

// WriteHeaders writes column headers for CSV
func (w *StreamingCSVWriter) WriteHeaders(tableName string, columns []string) error {
	if w.includeHeaders && !w.columnsWritten[tableName] {
		err := w.writer.Write(columns)
		if err != nil {
			return err
		}
		w.writer.Flush()
		w.columnsWritten[tableName] = true
	}
	return nil
}

// WriteRows writes a batch of rows in CSV format
func (w *StreamingCSVWriter) WriteRows(tableName string, rows []map[string]interface{}) error {
	columns := w.selectedColumns[tableName]

	for _, row := range rows {
		csvRow := make([]string, len(columns))

		for i, col := range columns {
			if val, ok := row[col]; ok && val != nil {
				csvRow[i] = fmt.Sprintf("%v", val)
			} else {
				csvRow[i] = ""
			}
		}

		if err := w.writer.Write(csvRow); err != nil {
			return err
		}
	}

	w.writer.Flush()
	return nil
}

// Finalize completes the CSV export
func (w *StreamingCSVWriter) Finalize() ([]byte, error) {
	w.writer.Flush()
	return w.buffer.Bytes(), nil
}

// StreamingXLSXWriter implements streaming XLSX export
type StreamingXLSXWriter struct {
	file            *xlsx.File
	sheets          map[string]*xlsx.Sheet
	includeHeaders  bool
	selectedColumns map[string][]string
	columnsWritten  map[string]bool
}

// Initialize prepares the XLSX writer
func (w *StreamingXLSXWriter) Initialize(tableNames []string, includeHeaders bool, selectedColumns map[string][]string) error {
	w.file = xlsx.NewFile()
	w.sheets = make(map[string]*xlsx.Sheet)
	w.includeHeaders = includeHeaders
	w.selectedColumns = selectedColumns
	w.columnsWritten = make(map[string]bool)

	return nil
}

// WriteTable creates a sheet for the table
func (w *StreamingXLSXWriter) WriteTable(tableName string) error {
	sheet, err := w.file.AddSheet(tableName)
	if err != nil {
		return err
	}
	w.sheets[tableName] = sheet
	return nil
}

// WriteHeaders writes column headers for XLSX
func (w *StreamingXLSXWriter) WriteHeaders(tableName string, columns []string) error {
	if w.includeHeaders && !w.columnsWritten[tableName] {
		sheet := w.sheets[tableName]
		row := sheet.AddRow()

		for _, col := range columns {
			cell := row.AddCell()
			cell.Value = col
		}

		w.columnsWritten[tableName] = true
	}
	return nil
}

// WriteRows writes a batch of rows in XLSX format
func (w *StreamingXLSXWriter) WriteRows(tableName string, rows []map[string]interface{}) error {
	sheet := w.sheets[tableName]
	columns := w.selectedColumns[tableName]

	for _, rowData := range rows {
		row := sheet.AddRow()

		for _, col := range columns {
			cell := row.AddCell()

			if val, ok := rowData[col]; ok && val != nil {
				cell.Value = fmt.Sprintf("%v", val)
			} else {
				cell.Value = ""
			}
		}
	}

	return nil
}

// Finalize completes the XLSX export
func (w *StreamingXLSXWriter) Finalize() ([]byte, error) {
	buffer := &bytes.Buffer{}
	err := w.file.Write(buffer)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// StreamingPDFWriter implements streaming PDF export
type StreamingPDFWriter struct {
	pdf             *gofpdf.Fpdf
	includeHeaders  bool
	selectedColumns map[string][]string
	columnsWritten  map[string]bool
	currentPage     int
	rowsPerPage     int
	rowCount        int
}

// Initialize prepares the PDF writer
func (w *StreamingPDFWriter) Initialize(tableNames []string, includeHeaders bool, selectedColumns map[string][]string) error {
	w.pdf = gofpdf.New("L", "mm", "A4", "")
	w.includeHeaders = includeHeaders
	w.selectedColumns = selectedColumns
	w.columnsWritten = make(map[string]bool)
	w.currentPage = 0
	w.rowsPerPage = 40 // Approximate rows per page
	w.rowCount = 0

	// Set up PDF
	w.pdf.SetFont("Arial", "", 12)
	return nil
}

// WriteTable adds a new page for each table
func (w *StreamingPDFWriter) WriteTable(tableName string) error {
	w.pdf.AddPage()
	w.currentPage++
	w.rowCount = 0

	// Add table title
	w.pdf.SetFont("Arial", "B", 16)
	w.pdf.Cell(40, 10, fmt.Sprintf("Table: %s", tableName))
	w.pdf.Ln(15)
	w.pdf.SetFont("Arial", "", 12)

	return nil
}

// WriteHeaders writes column headers for PDF
func (w *StreamingPDFWriter) WriteHeaders(tableName string, columns []string) error {
	//if w.includeHeaders && !w.columnsWritten[tableName] {
	//	//w.pdf.SetFont("Arial", "B", 12)
	//
	//	// Calculate column width
	//	//pageWidth := 277.0 // A4 landscape width in mm (approx)
	//	//colWidth := pageWidth / float64(len(columns))
	//
	//	//for _, col := range columns {
	//	//	w.pdf.Cell(float64(colWidth), 10, col)
	//	//}
	//
	//	//w.pdf.Ln(10)
	//	//w.pdf.SetFont("Arial", "", 12)
	//	w.columnsWritten[tableName] = true
	//	//w.rowCount++
	//}
	return nil
}

// WriteRows writes a batch of rows in PDF format
func (w *StreamingPDFWriter) WriteRows(tableName string, rows []map[string]interface{}) error {
	columns := w.selectedColumns[tableName]

	// Calculate column width
	pageWidth := 277.0 // A4 landscape width in mm (approx)
	colWidth := pageWidth / 4

	for _, rowData := range rows {
		// Check if we need a new page
		if w.rowCount >= w.rowsPerPage {
			w.pdf.AddPage()
			w.currentPage++
			w.rowCount = 0
		}

		for _, column := range columns {
			var value string
			if val, ok := rowData[column]; ok {
				value = fmt.Sprintf("%v", val)
			}
			if w.includeHeaders {
				w.pdf.SetFillColor(200, 200, 200)
			}

			w.pdf.SetFont("Arial", "B", 10)
			w.pdf.Cell(colWidth, 10, column)
			w.pdf.SetFont("Arial", "", 10)
			w.pdf.SetFillColor(40, 200, 400)
			w.pdf.Cell(colWidth, 10, value)
			w.pdf.Ln(-1)
		}
		w.pdf.SetFillColor(200, 200, 200)

		// Add a small vertical gap
		w.pdf.Ln(5)

		// Add a horizontal line (like <hr>)
		// Get current x and y position
		_, y := w.pdf.GetXY()

		// Set line color and width
		w.pdf.SetDrawColor(0, 0, 0) // Black
		w.pdf.SetLineWidth(0.5)     // 0.5mm width

		// Draw the line across the page
		w.pdf.Line(10, y, pageWidth-10, y) // 10mm from left, 287mm is nearly right edge of A4 landscape

		// Move down after the line
		w.pdf.Ln(5)
		w.pdf.AddPage()

		w.pdf.Ln(10)
		w.rowCount++
	}

	return nil
}

// Finalize completes the PDF export
func (w *StreamingPDFWriter) Finalize() ([]byte, error) {
	buffer := new(bytes.Buffer)
	err := w.pdf.Output(buffer)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// CreateStreamingExportWriter creates the appropriate writer based on format
func CreateStreamingExportWriter(format ExportFormat) (resource.StreamingExportWriter, error) {
	switch format {
	case FormatJSON:
		return &StreamingJSONWriter{}, nil
	case FormatCSV:
		return &StreamingCSVWriter{}, nil
	case FormatXLSX:
		return &StreamingXLSXWriter{}, nil
	case FormatPDF:
		return &StreamingPDFWriter{}, nil
	case FormatHTML:
		// DOCX streaming is more complex, fallback to JSON for now
		log.Warn("HTML streaming export not fully implemented, falling back to JSON")
		return &StreamingJSONWriter{}, nil
	default:
		return &StreamingJSONWriter{}, nil
	}
}
