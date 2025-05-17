package actions

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/rootpojo"
	"github.com/daptin/daptin/server/table_info"
	"os"
	"strings"

	"github.com/artpar/api2go/v2"
	fieldtypes "github.com/daptin/daptin/server/columntypes"
	"github.com/daptin/daptin/server/csvmap"
	"github.com/jmoiron/sqlx"
	"github.com/sadlil/go-trigger"
	log "github.com/sirupsen/logrus"
)

type uploadCsvFileToEntityPerformer struct {
	responseAttrs map[string]interface{}
	cruds         map[string]*resource.DbResource
	cmsConfig     *resource.CmsConfig
}

func (d *uploadCsvFileToEntityPerformer) Name() string {
	return "__upload_csv_file_to_entity"
}

func (d *uploadCsvFileToEntityPerformer) DoAction(request actionresponse.Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	//actions := make([]actionresponse.ActionResponse, 0)
	log.Printf("Do action: %v", d.Name())

	files := inFields["data_csv_file"].([]interface{})

	entityName := inFields["entity_name"].(string)
	create_if_not_exists, ok := inFields["create_if_not_exists"].(bool)
	if !ok {
		create_if_not_exists = false
	}
	add_missing_columns, ok := inFields["add_missing_columns"].(bool)
	if !ok {
		add_missing_columns = false
	}

	table := table_info.TableInfo{}
	table.TableName = entityName

	columns := make([]api2go.ColumnInfo, 0)

	allSt := make(map[string]interface{})

	sources := make([]rootpojo.DataFileImport, 0)

	completed := false

	var existingEntity *table_info.TableInfo
	if !create_if_not_exists {
		var ok bool
		dbr, ok := d.cruds[entityName]
		if !ok {
			return nil, nil, []error{fmt.Errorf("no such entity: %v", entityName)}
		}
		existingEntity = dbr.TableInfo()
	}

	schemaFolderDefinedByEnv, _ := os.LookupEnv("DAPTIN_SCHEMA_FOLDER")

	for _, fileInterface := range files {
		file, ok := fileInterface.(map[string]interface{})
		if !ok {
			continue
		}
		fileName := "_uploaded_" + file["name"].(string)
		fileContentsBase64 := file["file"].(string)
		fileBytes, err := base64.StdEncoding.DecodeString(strings.Split(fileContentsBase64, ",")[1])
		log.Printf("Processing file: %v", fileName)

		resource.CheckErr(err, "Uploaded file is not a valid csv file")
		if err != nil {
			return nil, nil, []error{err}
		}

		err = os.WriteFile(schemaFolderDefinedByEnv+string(os.PathSeparator)+fileName, fileBytes, 0644)
		if err != nil {
			log.Errorf("Failed to write xls file to disk: %v", err)
		}

		csvReader := csvmap.NewReader(bytes.NewReader(fileBytes))
		columnNames, err := csvReader.ReadHeader()

		if err != nil {
			return nil, nil, []error{err}
		}

		csvReader.Columns = columnNames
		data, err := csvReader.ReadAll()
		recordCount := len(data)
		if err != nil {
			return nil, nil, []error{err}
		}

		if err != nil {
			return nil, nil, []error{err}
		}

		// identify data type of each column
		for _, colName := range columnNames {

			if colName == "" {
				continue
			}

			var column api2go.ColumnInfo

			if add_missing_columns && existingEntity != nil {
				_, ok := existingEntity.GetColumnByName(colName)
				if !ok {
					// ignore column if it doesn't exists
					continue
				}
			}

			dataMap := map[string]bool{}
			datas := make([]string, 0)

			isNullable := false
			count := 100000
			for _, d := range data {
				if count < 0 {
					break
				}
				i := d[colName]
				var strVal string
				if i == "" {
					isNullable = true
					continue
				} else {
					strVal = i
				}
				if dataMap[strVal] {
					continue
				}
				dataMap[strVal] = true
				datas = append(datas, strVal)
				count -= 1
			}

			eType, _, err := fieldtypes.DetectType(datas)
			if err != nil {
				column.ColumnType = "label"
				column.DataType = "varchar(100)"
			} else {
				column.ColumnType = EntityTypeToColumnTypeMap[eType]
				column.DataType = entityTypeToDataTypeMap[eType]
			}

			if len(datas) > (recordCount / 10) {
				column.IsIndexed = true
			}

			if len(datas) == recordCount {
				column.IsUnique = true
			}

			column.IsNullable = isNullable
			column.Name = colName
			column.ColumnName = SmallSnakeCaseText(colName)

			columns = append(columns, column)
		}
		table.Columns = columns
		completed = true
		sources = append(sources, rootpojo.DataFileImport{
			FilePath: fileName,
			Entity:   table.TableName,
			FileType: "csv",
		},
		)

	}

	if completed {

		if create_if_not_exists {
			allSt["tables"] = []table_info.TableInfo{table}
		}

		allSt["imports"] = sources

		jsonStr, err := json.Marshal(allSt)
		if err != nil {
			resource.InfoErr(err, "Failed to convert object to json")
			return nil, nil, []error{err}
		}

		jsonFileName := fmt.Sprintf(schemaFolderDefinedByEnv+string(os.PathSeparator)+"schema_uploaded_%v_daptin.json", entityName)
		err = os.WriteFile(jsonFileName, jsonStr, 0644)
		if err != nil {
			return nil, nil, []error{err}
		}
		log.Printf("File %v written to disk for upload", jsonFileName)

		if create_if_not_exists || add_missing_columns {
			//go resource.Restart()
		} else {
			resource.ImportDataFiles(sources, transaction, d.cruds)
		}
		trigger.Fire("clean_up_uploaded_files")

		return nil, successResponses, nil
	} else {
		return nil, failedResponses, nil
	}

}

func NewUploadCsvFileToEntityPerformer(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {

	handler := uploadCsvFileToEntityPerformer{
		cruds:     cruds,
		cmsConfig: initConfig,
	}

	return &handler, nil

}
