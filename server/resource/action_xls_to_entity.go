package resource

import (
	"encoding/base64"
	"fmt"
	"github.com/artpar/api2go"
	"github.com/artpar/conform"
	"github.com/artpar/xlsx/v2"
	"github.com/daptin/daptin/server/columntypes"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"strconv"
	"strings"
)

type UploadXlsFileToEntityPerformer struct {
	responseAttrs map[string]interface{}
	cruds         map[string]*DbResource
	cmsConfig     *CmsConfig
}

func (d *UploadXlsFileToEntityPerformer) Name() string {
	return "__upload_xlsx_file_to_entity"
}

var EntityTypeToDataTypeMap = map[fieldtypes.EntityType]string{
	fieldtypes.DateTime:    "datetime",
	fieldtypes.Id:          "varchar(100)",
	fieldtypes.Time:        "time",
	fieldtypes.Date:        "date",
	fieldtypes.Ipaddress:   "varchar(100)",
	fieldtypes.Money:       "float(11)",
	fieldtypes.Rating5:     "int(4)",
	fieldtypes.Rating10:    "int(4)",
	fieldtypes.Rating100:   "int(4)",
	fieldtypes.Timestamp:   "timestamp",
	fieldtypes.NumberInt:   "int(5)",
	fieldtypes.NumberFloat: "float(11)",
	fieldtypes.Boolean:     "bool",
	fieldtypes.Latitude:    "float(11)",
	fieldtypes.Longitude:   "float(11)",
	fieldtypes.City:        "varchar(100)",
	fieldtypes.Country:     "varchar(100)",
	fieldtypes.Continent:   "varchar(100)",
	fieldtypes.State:       "varchar(100)",
	fieldtypes.Pincode:     "varchar(20)",
	fieldtypes.None:        "varchar(100)",
	fieldtypes.Label:       "varchar(100)",
	fieldtypes.Name:        "varchar(100)",
	fieldtypes.Email:       "varchar(100)",
	fieldtypes.Content:     "text",
	fieldtypes.Json:        "text",
	fieldtypes.Color:       "varchar(10)",
	fieldtypes.Alias:       "varchar(100)",
	fieldtypes.Namespace:   "varchar(100)",
}

var EntityTypeToColumnTypeMap = map[fieldtypes.EntityType]string{
	fieldtypes.DateTime:    "datetime",
	fieldtypes.Id:          "label",
	fieldtypes.Time:        "time",
	fieldtypes.Date:        "date",
	fieldtypes.Ipaddress:   "label",
	fieldtypes.Money:       "measurement",
	fieldtypes.Rating5:     "measurement",
	fieldtypes.Rating10:    "measurement",
	fieldtypes.Rating100:   "measurement",
	fieldtypes.Timestamp:   "timestamp",
	fieldtypes.NumberInt:   "measurement",
	fieldtypes.NumberFloat: "measurement",
	fieldtypes.Boolean:     "truefalse",
	fieldtypes.Latitude:    "location.latitude",
	fieldtypes.Longitude:   "location.longitude",
	fieldtypes.City:        "label",
	fieldtypes.Country:     "label",
	fieldtypes.Continent:   "label",
	fieldtypes.State:       "label",
	fieldtypes.Pincode:     "label",
	fieldtypes.None:        "content",
	fieldtypes.Label:       "label",
	fieldtypes.Name:        "name",
	fieldtypes.Email:       "email",
	fieldtypes.Content:     "content",
	fieldtypes.Json:        "json",
	fieldtypes.Color:       "color",
	fieldtypes.Alias:       "alias",
	fieldtypes.Namespace:   "namespace",
}

func (d *UploadXlsFileToEntityPerformer) DoAction(request Outcome, inFields map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	//actions := make([]ActionResponse, 0)
	log.Infof("Do action: %v", d.Name())

	files := inFields["data_xls_file"].([]interface{})

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

nextFile:
	for _, fileInterface := range files {
		file := fileInterface.(map[string]interface{})
		fileName := file["name"].(string)
		fileContentsBase64 := file["file"].(string)
		fileBytes, err := base64.StdEncoding.DecodeString(strings.Split(fileContentsBase64, ",")[1])
		log.Infof("Processing file: %v", fileName)

		xlsFile, err := xlsx.OpenBinary(fileBytes)
		CheckErr(err, "Uploaded file is not a valid xls file")
		if err != nil {
			return nil, nil, []error{fmt.Errorf("Failed to read file: %v", err)}
		}
		log.Infof("File has %d sheets", len(xlsFile.Sheets))
		err = ioutil.WriteFile(fileName, fileBytes, 0644)
		if err != nil {
			log.Errorf("Failed to write xls file to disk: %v", err)
		}

		for _, sheet := range xlsFile.Sheets {

			data, columnNames, err := GetDataArray(sheet)
			recordCount := len(data)

			if err != nil {
				log.Errorf("Failed to get data from sheet [%s]: %v", sheet.Name, err)
				return nil, nil, []error{fmt.Errorf("Failed to get data from sheet [%s]: %v", sheet.Name, err)}
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
				maxLen := 100
				for _, d := range data {
					if count < 0 {
						break
					}
					i := d[colName]
					var strVal string
					if i == nil {
						strVal = ""
						isNullable = true
						continue
					} else {
						strVal = i.(string)
					}
					if dataMap[strVal] {
						continue
					}
					dataMap[strVal] = true
					datas = append(datas, strVal)
					if maxLen < len(strVal) {
						maxLen = len(strVal)
					}
					count -= 1
				}

				eType, _, err := fieldtypes.DetectType(datas)
				if err != nil {
					log.Infof("Unable to identify column type for %v", colName)
					column.ColumnType = "label"
					column.DataType = fmt.Sprintf("varchar(%v)", maxLen)
				} else {
					log.Infof("Column %v was identified as %v", colName, eType)
					column.ColumnType = EntityTypeToColumnTypeMap[eType]

					dbDataType := EntityTypeToDataTypeMap[eType]
					if strings.Index(dbDataType, "varchar") == 0 {
						dbDataType = fmt.Sprintf("varchar(%v)", maxLen+100)
					}
					column.DataType = dbDataType

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
			sources = append(sources, DataFileImport{FilePath: fileName, Entity: table.TableName, FileType: "xlsx"})

			break nextFile
		}
	}

	if completed {

		if create_if_not_exists {
			allSt["tables"] = []TableInfo{table}
		}

		allSt["imports"] = sources

		jsonStr, err := json.Marshal(allSt)
		if err != nil {
			InfoErr(err, "Failed to convert object to json")
			return nil, nil, []error{err}
		}

		jsonFileName := fmt.Sprintf("schema_%v_daptin.json", entityName)
		err = ioutil.WriteFile(jsonFileName, jsonStr, 0644)
		CheckErr(err, "Failed to write json to schema file [%v]", jsonFileName)
		log.Printf("File %v written to disk for upload", jsonFileName)

		if create_if_not_exists || add_missing_columns {
			go restart()
		} else {
			ImportDataFiles(sources, d.cruds[entityName].db, d.cruds)
		}

		return nil, successResponses, nil
	} else {
		return nil, failedResponses, nil
	}

}

var successResponses = []ActionResponse{
	NewActionResponse("client.notify", map[string]interface{}{
		"type":    "success",
		"message": "Initiating system update.",
		"title":   "Success",
	}),
	NewActionResponse("client.redirect", map[string]interface{}{
		"location": "/",
		"window":   "self",
		"delay":    15000,
	}),
}

var failedResponses = []ActionResponse{
	NewActionResponse("client.notify", map[string]interface{}{
		"type":    "error",
		"message": "Failed to import xls",
		"title":   "Failed",
	}),
}

func (s DataFileImport) String() string {
	return fmt.Sprintf("[%v][%v]", s.FileType, s.FilePath)
}

type DataFileImport struct {
	FilePath string
	Entity   string
	FileType string
}

func SmallSnakeCaseText(str string) string {
	transformed := conform.TransformString(str, "lower,snake")
	_, ok := strconv.Atoi(string(transformed[0]))
	if IsReservedWord(transformed) || ok == nil {
		return "col_" + transformed
	}
	return transformed
}

func GetDataArray(sheet *xlsx.Sheet) (dataMap []map[string]interface{}, columnNames []string, err error) {

	data := make([]map[string]interface{}, 0)

	rowCount := sheet.MaxRow
	columnCount := sheet.MaxCol

	log.Infof("Sheet has %d rows", rowCount)
	log.Infof("Sheet has %d cols", columnCount)

	if columnCount < 1 {
		err = errors.New("Sheet has 0 columns")
		return
	}

	if rowCount < 2 {
		err = errors.New("Sheet has less than 2 rows")
		return
	}

	//columnNames = make([]string, 0)
	properColumnNames := make([]string, 0)

	headerRow, _ := sheet.Row(0)

	for i := 0; i < columnCount; i++ {
		colName := headerRow.GetCell(i).Value
		if len(colName) < 1 {
			//err = errors.New(fmt.Sprintf("Column %d name has less then 3 characters", i+1))
			break
		}
		//columnNames = append(columnNames, colName)
		properColumnNames = append(properColumnNames, SmallSnakeCaseText(colName))
	}

	for i := 1; i < rowCount; i++ {
		emptyRow := true

		dataMap := make(map[string]interface{})

		currentRow, _ := sheet.Row(i)
		cCount := columnCount
		for j := 0; j < cCount; j++ {
			i2 := currentRow.GetCell(j).Value
			if strings.TrimSpace(i2) == "" {
				continue
			}
			emptyRow = false
			dataMap[properColumnNames[j]] = i2
		}
		if !emptyRow {
			data = append(data, dataMap)
		}
	}

	return data, properColumnNames, nil

}

func NewUploadFileToEntityPerformer(initConfig *CmsConfig, cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := UploadXlsFileToEntityPerformer{
		cruds:     cruds,
		cmsConfig: initConfig,
	}

	return &handler, nil

}
