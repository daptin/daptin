package actions

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

// importDataPerformer handles data import from various formats
type importDataPerformer struct {
	cmsConfig *resource.CmsConfig
	cruds     map[string]*resource.DbResource
}

// Name returns the name of this action
func (d *importDataPerformer) Name() string {
	return "__data_import"
}

// DoAction performs the import action
func (d *importDataPerformer) DoAction(request actionresponse.Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	tableName, ok := inFields["table_name"].(string)
	if !ok || tableName == "" || d.cruds[tableName] == nil {
		return nil, nil, []error{fmt.Errorf("import target table is unavailable")}
	}
	sessionUser, ok := inFields["sessionUser"].(*auth.SessionUser)
	if !ok || sessionUser == nil || sessionUser.UserId == 0 {
		return nil, nil, []error{fmt.Errorf("import user is unavailable")}
	}
	// Get import options
	truncateBeforeInsert := false
	if val, ok := inFields["truncate_before_insert"]; ok && val != nil {
		if boolVal, ok := val.(bool); ok {
			truncateBeforeInsert = boolVal
		}
	}
	batchSize := 100
	if val, ok := inFields["batch_size"]; ok && val != nil {
		switch size := val.(type) {
		case int:
			if size > 0 {
				batchSize = size
			}
		case float64:
			if size > 0 && size == float64(int(size)) {
				batchSize = int(size)
			}
		}
	}

	// Get files to import
	files, ok := inFields["dump_file"].([]interface{})
	if !ok || len(files) == 0 {
		return nil, nil, []error{fmt.Errorf("no files provided for import")}
	}

	startTime := time.Now()
	totalRowsImported := 0
	_, hasOwnerColumn := d.cruds[tableName].TableInfo().GetColumnByName(resource.USER_ACCOUNT_ID_COLUMN)
	if truncateBeforeInsert {
		if err := d.cruds[tableName].TruncateTable(tableName, false, transaction); err != nil {
			return nil, nil, []error{fmt.Errorf("failed to truncate table %q: %w", tableName, err)}
		}
	}

	// Process each file
	for fileIndex, fileInterface := range files {
		file, ok := fileInterface.(map[string]interface{})
		if !ok {
			return nil, nil, []error{fmt.Errorf("invalid import file at index %d", fileIndex)}
		}

		fileName, ok := file["name"].(string)
		if !ok || fileName == "" {
			return nil, nil, []error{fmt.Errorf("missing import file name at index %d", fileIndex)}
		}

		fileContentsBase64, ok := file["file"].(string)
		if !ok {
			return nil, nil, []error{fmt.Errorf("missing import file content at index %d", fileIndex)}
		}

		// Decode base64 content
		contentParts := strings.Split(fileContentsBase64, ",")
		var fileBytes []byte
		var err error

		if len(contentParts) > 1 {
			fileBytes, err = base64.StdEncoding.DecodeString(contentParts[1])
		} else {
			fileBytes, err = base64.StdEncoding.DecodeString(contentParts[0])
		}

		if err != nil {
			return nil, nil, []error{fmt.Errorf("failed to decode file %q: %w", fileName, err)}
		}

		log.Infof("Processing import file: %s (%d bytes)", fileName, len(fileBytes))

		// Detect file format and create appropriate parser
		format := DetectFileFormat(fileBytes, fileName)
		parser, err := CreateStreamingImportParser(format)
		if err != nil {
			return nil, nil, []error{fmt.Errorf("failed to create parser for file %q: %w", fileName, err)}
		}

		// Initialize the parser with file content
		err = parser.Initialize(fileBytes, tableName)
		if err != nil {
			return nil, nil, []error{fmt.Errorf("failed to parse file %q: %w", fileName, err)}
		}

		// Get table names from the import file
		tableNames, err := parser.GetTableNames()
		if err != nil {
			return nil, nil, []error{fmt.Errorf("failed to get table name from file %q: %w", fileName, err)}
		}
		if len(tableNames) != 1 || tableNames[0] != tableName {
			return nil, nil, []error{fmt.Errorf("file %q does not contain only table %q", fileName, tableName)}
		}
		// Process rows in batches through the raw import path.
		err = parser.ParseRows(tableName, batchSize, func(rows []map[string]interface{}) error {
			for _, row := range rows {
				if hasOwnerColumn {
					row[resource.USER_ACCOUNT_ID_COLUMN] = sessionUser.UserId
				}

				err := d.cruds[tableName].DirectInsert(tableName, row, transaction)
				if err != nil {
					return err
				}
				totalRowsImported++
			}
			return nil
		})

		if err != nil {
			return nil, nil, []error{fmt.Errorf("failed to import file %q into table %q: %w", fileName, tableName, err)}
		}
	}
	if totalRowsImported == 0 {
		return nil, nil, []error{fmt.Errorf("no rows found to import into table %q", tableName)}
	}

	// Create response with import summary
	duration := time.Since(startTime)
	responseAttrs := make(map[string]interface{})
	responseAttrs["message"] = fmt.Sprintf("Import completed in %v. %d rows imported successfully across 1 table.", duration.Round(time.Millisecond), totalRowsImported)
	responseAttrs["rows_imported"] = totalRowsImported
	responseAttrs["successful_tables"] = 1
	responseAttrs["failed_tables"] = 0

	actionResponse := resource.NewActionResponse("client.notify", responseAttrs)
	return nil, []actionresponse.ActionResponse{actionResponse}, nil
}

// NewImportDataPerformer creates a new instance of the import data performer
func NewImportDataPerformer(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {
	handler := importDataPerformer{
		cmsConfig: initConfig,
		cruds:     cruds,
	}

	return &handler, nil
}
