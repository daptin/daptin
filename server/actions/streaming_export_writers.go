package actions

import (
	"bytes"
	"encoding/csv"
	"fmt"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"

	"github.com/artpar/xlsx/v2"
	"github.com/daptin/daptin/server/resource"
	"github.com/jung-kurt/gofpdf"
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
	log.Infof("Writing [%d] rows", len(rows))

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

// StreamingHTMLWriter implements streaming HTML table export
type StreamingHTMLWriter struct {
	buffer       *bytes.Buffer
	isFirstRow   bool
	isFirstTable bool
	tableCount   int
	columns      []string
}

// Initialize prepares the HTML writer
func (w *StreamingHTMLWriter) Initialize(tableNames []string, includeHeaders bool, selectedColumns map[string][]string) error {
	w.buffer = &bytes.Buffer{}
	w.isFirstTable = true
	w.isFirstRow = true
	w.tableCount = 0
	w.columns = selectedColumns[tableNames[0]]

	// Write HTML document header with CSS styles
	w.buffer.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Data Tables</title>
    <style>
        :root {
            --primary-color: #3498db;
            --primary-dark: #2980b9;
            --secondary-color: #f8f9fa;
            --text-color: #333;
            --border-color: #ddd;
            --hover-color: #eaf2f8;
            --stripe-color: #f2f6f9;
        }
        
        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            line-height: 1.6;
            color: var(--text-color);
            background-color: #f5f7fa;
            margin: 0;
            padding: 20px;
        }
        
        .container {
            max-width: 1200px;
            margin: 0 auto;
            background-color: white;
            border-radius: 8px;
            box-shadow: 0 2px 10px rgba(0, 0, 0, 0.1);
            padding: 20px;
        }
        
        h1 {
            color: var(--primary-dark);
            margin-top: 0;
            padding-bottom: 10px;
            border-bottom: 1px solid var(--border-color);
        }
        
        .table-container {
            margin-bottom: 30px;
            overflow-x: auto;
        }
        
        table {
            width: 100%;
            border-collapse: collapse;
            margin-bottom: 10px;
            box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
        }
        
        caption {
            font-size: 1.2rem;
            font-weight: 600;
            padding: 10px;
            text-align: left;
            color: var(--primary-dark);
            background-color: white;
            border-top: 1px solid var(--border-color);
            border-left: 1px solid var(--border-color);
            border-right: 1px solid var(--border-color);
            border-top-left-radius: 5px;
            border-top-right-radius: 5px;
        }
        
        thead {
            background-color: var(--primary-color);
            color: white;
        }
        
        th {
            padding: 12px 15px;
            text-align: left;
            font-weight: 600;
            position: sticky;
            top: 0;
            background-color: var(--primary-color);
            box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
            white-space: nowrap;
            cursor: pointer;
        }
        
        th:hover {
            background-color: var(--primary-dark);
        }
        
        td {
            padding: 10px 15px;
            border-bottom: 1px solid var(--border-color);
            vertical-align: top;
        }
        
        tbody tr:nth-child(even) {
            background-color: var(--stripe-color);
        }
        
        tbody tr:hover {
            background-color: var(--hover-color);
        }
        
        .table-footer {
            font-size: 0.9rem;
            color: #6c757d;
            text-align: right;
            padding: 5px 15px;
            border: 1px solid var(--border-color);
            border-top: none;
            border-bottom-left-radius: 5px;
            border-bottom-right-radius: 5px;
            background-color: white;
        }
        
        .search-container {
            margin-bottom: 15px;
        }
        
        .search-box {
            padding: 8px 15px;
            width: 100%;
            border: 1px solid var(--border-color);
            border-radius: 4px;
            font-size: 1rem;
            box-sizing: border-box;
        }
        
        .timestamp {
            text-align: right;
            color: #6c757d;
            font-size: 0.85rem;
            margin-top: 15px;
        }
        
        @media (max-width: 768px) {
            body {
                padding: 10px;
            }
            
            .container {
                padding: 15px;
            }
            
            th, td {
                padding: 8px 10px;
            }
            
            h1 {
                font-size: 1.5rem;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Data Tables</h1>
        <div class="search-container">
            <input type="text" id="tableSearch" class="search-box" placeholder="Search across all tables..." onkeyup="searchTables()">
        </div>
`)

	return nil
}

// WriteTable writes a table with appropriate styling and structure
func (w *StreamingHTMLWriter) WriteTable(tableName string) error {
	w.tableCount++
	if !w.isFirstTable {
		w.buffer.WriteString("</tbody></table><div class='table-footer'><span id='rowCount" + fmt.Sprintf("%d", w.tableCount-1) + "'></span></div></div>")
	}
	w.isFirstTable = false
	w.isFirstRow = true

	// Write table container and table with caption
	w.buffer.WriteString("<div class='table-container'>")
	w.buffer.WriteString("<table id='dataTable" + fmt.Sprintf("%d", w.tableCount) + "'>")
	w.buffer.WriteString("<caption>" + tableName + "</caption>")

	return nil
}

// WriteHeaders writes the table headers with styling
func (w *StreamingHTMLWriter) WriteHeaders(tableName string, columns []string) error {
	w.buffer.WriteString("<thead><tr>")

	w.columns = columns
	for _, column := range w.columns {
		w.buffer.WriteString(fmt.Sprintf("<th onclick='sortTable(%d, %d)'>%s</th>", w.tableCount, len(columns), escapeHTML(column)))
	}

	w.buffer.WriteString("</tr></thead><tbody>")
	return nil
}

// WriteRows writes a batch of rows with alternating colors and hover effects
func (w *StreamingHTMLWriter) WriteRows(tableName string, rows []map[string]interface{}) error {
	log.Infof("Writing [%d] rows", len(rows))
	for _, row := range rows {
		w.buffer.WriteString("<tr>")
		for _, colName := range w.columns {
			w.buffer.WriteString(fmt.Sprintf("<td>%v</td>", formatValue(row[colName])))
		}
		w.buffer.WriteString("</tr>")
	}

	return nil
}

// Finalize completes the HTML export with JavaScript for interactivity
func (w *StreamingHTMLWriter) Finalize() ([]byte, error) {
	// Close the last table if any were written
	if w.tableCount > 0 {
		w.buffer.WriteString("</tbody></table>")
		w.buffer.WriteString("<div class='table-footer'><span id='rowCount" + fmt.Sprintf("%d", w.tableCount) + "'></span></div></div>")
	}

	// Add timestamp
	currentTime := time.Now().Format("January 2, 2006 15:04:05")
	w.buffer.WriteString("<div class='timestamp'>Generated on " + currentTime + "</div>")

	// Add JavaScript for sorting, filtering, and row counting
	w.buffer.WriteString(`
    <script>
        // Initialize row counts for all tables
        document.addEventListener('DOMContentLoaded', function() {
            updateAllRowCounts();
        });
        
        // Function to update row counts for all tables
        function updateAllRowCounts() {
            const tables = document.querySelectorAll('table');
            tables.forEach((table, index) => {
                const rowCount = table.tBodies[0].rows.length;
                const rowCountEl = document.getElementById('rowCount' + (index + 1));
                if (rowCountEl) {
                    rowCountEl.textContent = rowCount + ' rows';
                }
            });
        }
        
        // Table sorting function
        function sortTable(tableIndex, columnIndex) {
            const table = document.getElementById('dataTable' + tableIndex);
            const tbody = table.tBodies[0];
            const rows = Array.from(tbody.rows);
            
            // Determine sort direction
            const th = table.querySelectorAll('th')[columnIndex];
            const asc = !th.classList.contains('asc');
            
            // Reset all headers
            table.querySelectorAll('th').forEach(header => {
                header.classList.remove('asc', 'desc');
            });
            
            // Set new sort direction
            th.classList.toggle('asc', asc);
            th.classList.toggle('desc', !asc);
            
            // Sort rows
            rows.sort((a, b) => {
                const aValue = a.cells[columnIndex].textContent.trim();
                const bValue = b.cells[columnIndex].textContent.trim();
                
                // Check if numbers
                const aNum = parseFloat(aValue);
                const bNum = parseFloat(bValue);
                
                if (!isNaN(aNum) && !isNaN(bNum)) {
                    return asc ? aNum - bNum : bNum - aNum;
                }
                
                // String comparison
                return asc 
                    ? aValue.localeCompare(bValue) 
                    : bValue.localeCompare(aValue);
            });
            
            // Rearrange rows
            rows.forEach(row => tbody.appendChild(row));
        }
        
        // Search function across all tables
        function searchTables() {
            const searchTerm = document.getElementById('tableSearch').value.toLowerCase();
            const tables = document.querySelectorAll('table');
            
            tables.forEach((table, tableIndex) => {
                const tbody = table.tBodies[0];
                const rows = tbody.rows;
                let visibleRows = 0;
                
                for (let i = 0; i < rows.length; i++) {
                    const row = rows[i];
                    let found = false;
                    
                    for (let j = 0; j < row.cells.length; j++) {
                        const cell = row.cells[j];
                        if (cell.textContent.toLowerCase().includes(searchTerm)) {
                            found = true;
                            break;
                        }
                    }
                    
                    if (found || searchTerm === '') {
                        row.style.display = '';
                        visibleRows++;
                    } else {
                        row.style.display = 'none';
                    }
                }
                
                // Update row count to show filtered count
                const rowCountEl = document.getElementById('rowCount' + (tableIndex + 1));
                if (rowCountEl) {
                    if (searchTerm === '') {
                        rowCountEl.textContent = rows.length + ' rows';
                    } else {
                        rowCountEl.textContent = visibleRows + ' of ' + rows.length + ' rows';
                    }
                }
            });
        }
    </script>
    </div>
</body>
</html>`)

	return w.buffer.Bytes(), nil
}

// Helper function to escape HTML content
func escapeHTML(s string) string {
	return strings.Replace(
		strings.Replace(
			strings.Replace(
				strings.Replace(
					strings.Replace(s, "&", "&amp;", -1),
					"<", "&lt;", -1),
				">", "&gt;", -1),
			"\"", "&quot;", -1),
		"'", "&#39;", -1)
}

// Helper function to format values appropriately
func formatValue(value interface{}) string {
	if value == nil {
		return "<span class='null-value'>NULL</span>"
	}

	switch v := value.(type) {
	case time.Time:
		return v.Format("2006-01-02 15:04:05")
	case float64:
		// Format numbers based on their decimal places
		if v == float64(int(v)) {
			return fmt.Sprintf("%.0f", v)
		}
		return fmt.Sprintf("%.2f", v)
	default:
		return escapeHTML(fmt.Sprintf("%v", v))
	}
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
	log.Infof("Writing [%d] rows", len(rows))

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
	log.Infof("Writing [%d] rows", len(rows))

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
	log.Infof("Writing [%d] rows", len(rows))

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
		return &StreamingHTMLWriter{}, nil
	default:
		return &StreamingJSONWriter{}, nil
	}
}
