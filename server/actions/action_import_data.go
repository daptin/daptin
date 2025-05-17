package actions

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	daptinid "github.com/daptin/daptin/server/id"
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
	responses := make([]actionresponse.ActionResponse, 0)
	errors := make([]error, 0)

	// Get the target table name if specified
	tableName, isSubjected := inFields["table_name"]

	// Get user information if present
	user, isUserPresent := inFields["user"]
	var userIdInt int64 = 1
	if isUserPresent {
		userMap := user.(map[string]interface{})
		userReferenceId := daptinid.InterfaceToDIR(userMap["reference_id"])
		var err error
		userIdInt, err = d.cruds[resource.USER_ACCOUNT_TABLE_NAME].GetReferenceIdToId(resource.USER_ACCOUNT_TABLE_NAME, userReferenceId, transaction)
		if err != nil {
			log.Errorf("Failed to get user id from user reference id: %v", err)
		}
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
		if intVal, ok := val.(int); ok && intVal > 0 {
			batchSize = intVal
		}
	}

	// Get files to import
	files, ok := inFields["dump_file"].([]interface{})
	if !ok || len(files) == 0 {
		err := fmt.Errorf("no files provided for import")
		errors = append(errors, err)
		return nil, responses, errors
	}

	startTime := time.Now()
	totalRowsImported := 0
	successfulImports := 0
	failedImports := 0

	// Process each file
	for fileIndex, fileInterface := range files {
		file, ok := fileInterface.(map[string]interface{})
		if !ok {
			log.Errorf("Invalid file format at index %d", fileIndex)
			continue
		}

		fileName, ok := file["name"].(string)
		if !ok {
			log.Errorf("Missing file name at index %d", fileIndex)
			continue
		}

		fileContentsBase64, ok := file["file"].(string)
		if !ok {
			log.Errorf("Missing file content at index %d", fileIndex)
			continue
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
			log.Errorf("Failed to decode file contents as base64: %v", err)
			errors = append(errors, fmt.Errorf("failed to decode file '%s': %w", fileName, err))
			continue
		}

		log.Infof("Processing import file: %s (%d bytes)", fileName, len(fileBytes))

		// Detect file format and create appropriate parser
		format := DetectFileFormat(fileBytes, fileName)
		parser, err := CreateStreamingImportParser(format)
		if err != nil {
			log.Errorf("Failed to create parser for file '%s': %v", fileName, err)
			errors = append(errors, fmt.Errorf("failed to create parser for file '%s': %w", fileName, err))
			continue
		}

		tableNameString := tableName.(string)
		// Initialize the parser with file content
		err = parser.Initialize(fileBytes, tableNameString)
		if err != nil {
			log.Errorf("Failed to parse file '%s': %v", fileName, err)
			errors = append(errors, fmt.Errorf("failed to parse file '%s': %w", fileName, err))
			continue
		}

		// Get table names from the import file
		tableNames, err := parser.GetTableNames()
		if err != nil {
			log.Errorf("Failed to get table names from file '%s': %v", fileName, err)
			errors = append(errors, fmt.Errorf("failed to get table names from file '%s': %w", fileName, err))
			continue
		}

		// Filter tables if a specific table is requested
		if isSubjected && tableName != nil {
			targetTable := tableName.(string)

			tableNames = []string{targetTable}
		}

		// Process each table in the file
		for _, currentTable := range tableNames {
			// Skip if we don't have access to this table
			if _, ok := d.cruds[currentTable]; !ok {
				log.Warnf("Skipping table [%s]: not accessible", currentTable)
				continue
			}

			// Truncate table if requested
			if truncateBeforeInsert {
				instance, ok := d.cruds[currentTable]
				if !ok {
					log.Warnf("Wanted to truncate table '%s', but no instance available", currentTable)
					continue
				}

				err := instance.TruncateTable(currentTable, false, transaction)
				if err != nil {
					log.Errorf("Failed to truncate table '%s': %v", currentTable, err)
					errors = append(errors, fmt.Errorf("failed to truncate table '%s': %w", currentTable, err))
				} else {
					log.Infof("Truncated table '%s' before import", currentTable)
				}
			}

			tableRowCount := 0
			tableFailCount := 0

			// Process rows in batches
			err = parser.ParseRows(currentTable, batchSize, func(rows []map[string]interface{}) error {
				for _, row := range rows {
					// Add user reference if present
					if isUserPresent {
						row[resource.USER_ACCOUNT_TABLE_NAME] = userIdInt
					}

					err := d.cruds[currentTable].DirectInsert(currentTable, row, transaction)
					if err != nil {
						log.Errorf("Failed to insert row into table '%s': %v", currentTable, err)
						tableFailCount++
						failedImports++
					} else {
						tableRowCount++
						totalRowsImported++
					}
				}
				return nil
			})

			if err != nil {
				log.Errorf("Error processing rows for table '%s': %v", currentTable, err)
				errors = append(errors, fmt.Errorf("error processing rows for table '%s': %w", currentTable, err))
			} else {
				log.Infof("Successfully imported %d rows into table '%s' (failed: %d)", tableRowCount, currentTable, tableFailCount)
				successfulImports++
			}
		}
	}

	// Create response with import summary
	duration := time.Since(startTime)
	responseAttrs := make(map[string]interface{})
	responseAttrs["message"] = fmt.Sprintf("Import completed in %v. %d rows imported successfully across %d tables.", duration.Round(time.Millisecond), totalRowsImported, successfulImports)
	responseAttrs["rows_imported"] = totalRowsImported
	responseAttrs["successful_tables"] = successfulImports
	responseAttrs["failed_tables"] = failedImports

	actionResponse := resource.NewActionResponse("client.notify", responseAttrs)
	responses = append(responses, actionResponse)

	return nil, responses, errors
}

// NewImportDataPerformer creates a new instance of the import data performer
func NewImportDataPerformer(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {
	handler := importDataPerformer{
		cmsConfig: initConfig,
		cruds:     cruds,
	}

	return &handler, nil
}
