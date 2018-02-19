package resource

import (
	"encoding/base64"
	"fmt"
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/columntypes"
	"github.com/gin-gonic/gin/json"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"strings"
	"github.com/daptin/daptin/server/csvmap"
	"bytes"
)

type UploadCsvFileToEntityPerformer struct {
	responseAttrs map[string]interface{}
	cruds         map[string]*DbResource
	cmsConfig     *CmsConfig
}

func (d *UploadCsvFileToEntityPerformer) Name() string {
	return "__upload_csv_file_to_entity"
}

func (d *UploadCsvFileToEntityPerformer) DoAction(request ActionRequest, inFields map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	//actions := make([]ActionResponse, 0)
	log.Infof("Do action: %v", d.Name())

	files := inFields["data_csv_file"].([]interface{})

	entityName := inFields["entity_name"].(string)
	create_if_not_exists := inFields["create_if_not_exists"].(bool)
	add_missing_columns := inFields["add_missing_columns"].(bool)

	table := TableInfo{}
	table.TableName = SmallSnakeCaseText(entityName)

	columns := make([]api2go.ColumnInfo, 0)

	allSt := make(map[string]interface{})

	sources := make([]DataFileImport, 0)

	completed := false

	var existingEntity *TableInfo
	if !create_if_not_exists {
		var ok bool
		dbr, ok := d.cruds[entityName]
		if !ok {
			return nil, nil, []error{fmt.Errorf("no such entity: %v", entityName)}
		}
		existingEntity = dbr.tableInfo
	}

	for _, fileInterface := range files {
		file := fileInterface.(map[string]interface{})
		fileName := file["name"].(string)
		fileContentsBase64 := file["file"].(string)
		fileBytes, err := base64.StdEncoding.DecodeString(strings.Split(fileContentsBase64, ",")[1])
		log.Infof("Processing file: %v", fileName)

		CheckErr(err, "Uploaded file is not a valid csv file")
		if err != nil {
			return nil, nil, []error{err}
		}

		csvReader := csvmap.NewReader(bytes.NewReader(fileBytes))
		columnNames, err := csvReader.ReadHeader()

		if err != nil {
			return nil, nil, []error{err}
		}

		csvReader.Columns = columnNames
		data, err := csvReader.ReadAll()
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
			count := 100
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
				column.DataType = EntityTypeToDataTypeMap[eType]
			}
			column.IsNullable = isNullable
			column.Name = colName
			column.ColumnName = colName

			columns = append(columns, column)
		}
		table.Columns = columns
		completed = true
		sources = append(sources, DataFileImport{FilePath: fileName, Entity: table.TableName, FileType: "csv"})

	}

	if completed {

		if create_if_not_exists {
			allSt["tables"] = []TableInfo{table}
		}

		allSt["imports"] = sources

		jsonStr, err := json.Marshal(allSt)
		if err != nil {
			log.Errorf("Failed to convert table to json string")
			return nil, nil, []error{err}
		}

		jsonFileName := fmt.Sprintf("schema_%v_daptin.json", entityName)
		ioutil.WriteFile(jsonFileName, jsonStr, 0644)
		ImportDataFiles(d.cmsConfig, d.cruds[entityName].db, d.cruds)
		//go restart()
		return nil, successResponses, nil
	} else {
		return nil, failedResponses, nil
	}

}

func NewUploadCsvFileToEntityPerformer(initConfig *CmsConfig, cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := UploadCsvFileToEntityPerformer{
		cruds:     cruds,
		cmsConfig: initConfig,
	}

	return &handler, nil

}
