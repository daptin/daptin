package csvmap

import (
	"encoding/csv"
	"fmt"
	"io"
)

// A Reader reads records from a CSV-encoded file.
type Reader struct {
	Reader  *csv.Reader
	Columns []string
}

// NewReader returns a new Reader that reads from r.
func NewReader(r io.Reader) *Reader {
	return &Reader{
		Reader: csv.NewReader(r),
	}
}

// ReadHeader simply wraps csv.Reader.Read().
func (r *Reader) ReadHeader() (columns []string, err error) {
	return r.Reader.Read()
}

// Read wraps csv.Reader.Read, creating a map of column name to field value.
// If the line has fewer columns than Reader.Columns, the map will not contain keys for these columns;
// thus we can distinguish between missing columns and columns with empty values.
// If the line has more columns than Reader.Columns, Reader.Read() ignores them.
func (r *Reader) Read() (record map[string]string, err error) {
	var rawRecord []string
	rawRecord, err = r.Reader.Read()
	length := min(len(rawRecord), len(r.Columns))
	record = make(map[string]string)
	for index := 0; index < length; index++ {
		column := r.Columns[index]
		if _, exists := record[column]; exists {
			return nil, fmt.Errorf("Multiple indices with the same name '%s'", column)
		}
		record[column] = rawRecord[index]
	}
	return
}

// ReadAll reads all the remaining records from r. Each record is a map of column name to field value.
func (r *Reader) ReadAll() (records []map[string]string, err error) {
	var record map[string]string
	for record, err = r.Read(); err == nil; record, err = r.Read() {
		records = append(records, record)
	}
	if err != nil && err != io.EOF {
		return nil, err
	}
	return records, nil
}

func min(x, y int) int {
	if x <= y {
		return x
	}
	return y
}
