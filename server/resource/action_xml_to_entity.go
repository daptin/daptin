package resource

import (
	log "github.com/sirupsen/logrus"
	"github.com/tealeg/xlsx"
	"encoding/base64"
	"strings"
	"github.com/artpar/api2go"
	"github.com/pkg/errors"
	"fmt"
	"github.com/artpar/goms/server/columntypes"
	"github.com/artpar/conform"
	"github.com/gin-gonic/gin/json"
	"io/ioutil"
)

type UploadFileToEntityPerformer struct {
	responseAttrs map[string]interface{}
	cruds         map[string]*DbResource
}

func (d *UploadFileToEntityPerformer) Name() string {
	return "__upload_file_to_entity"
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

func (d *UploadFileToEntityPerformer) DoAction(request ActionRequest, inFields map[string]interface{}) ([]ActionResponse, []error) {

	//actions := make([]ActionResponse, 0)
	log.Infof("Do action: %v", d.Name())

	files := inFields["data_xls_file"].([]interface{})

	entityName := inFields["entity_name"].(string)

	table := TableInfo{}
	table.TableName = SmallSnakeCaseText(entityName)

	columns := make([]api2go.ColumnInfo, 0)

	allSt := make(map[string]interface{})

	sources := make([]DataFileImport, 0)

	completed := false

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
			continue
		}
		log.Infof("File has %d sheets", len(xlsFile.Sheets))
		err = ioutil.WriteFile(fileName, fileBytes, 0644)
		if err != nil {
			log.Errorf("Failed to write xls file to disk: %v", err)
		}

		for _, sheet := range xlsFile.Sheets {

			data, columnNames, err := GetDataArray(sheet)

			if err != nil {
				log.Errorf("Failed to get data from sheet [%s]: %v", sheet.Name, err)
				continue
			}

			// identify data type of each column
			for _, colName := range columnNames {
				var column api2go.ColumnInfo

				datas := make([]string, 0)

				count := 10
				for _, d := range data {
					if count < 0 {
						break
					}
					i := d[colName]
					if i == nil {
						continue
					}
					datas = append(datas, i.(string))
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
				column.Name = colName
				column.ColumnName = SmallSnakeCaseText(colName)

				if column.ColumnName == "" {
					continue
				}

				columns = append(columns, column)
			}
			table.Columns = columns
			completed = true
			sources = append(sources, DataFileImport{FilePath: fileName, Entity: table.TableName, FileType: "xlsx"})

			break nextFile
		}
	}

	if completed {
		allSt["tables"] = []TableInfo{table}
		allSt["imports"] = sources

		jsonStr, err := json.Marshal(allSt)
		if err != nil {
			log.Errorf("Failed to convert table to json string")
			return nil, []error{err}
		}

		jsonFileName := fmt.Sprintf("schema_%v_gocms.json", entityName)
		ioutil.WriteFile(jsonFileName, jsonStr, 0644)

		go restart()
	}

	return []ActionResponse{}, nil
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
	if IsReservedWord(transformed) {
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

	columnNames = make([]string, 0)
	properColumnNames := make([]string, 0)

	headerRow := sheet.Rows[0]

	for i := 0; i < columnCount; i++ {
		colName := headerRow.Cells[i].Value
		if len(colName) < 1 {
			err = errors.New(fmt.Sprintf("Column %d name has less then 3 characters", i+1))
			return
		}
		columnNames = append(columnNames, colName)
		properColumnNames = append(properColumnNames, SmallSnakeCaseText(colName))
	}

	for i := 1; i < rowCount; i++ {

		dataMap := make(map[string]interface{})

		currentRow := sheet.Rows[i]
		cCount := len(currentRow.Cells)
		for j := 0; j < cCount; j++ {
			dataMap[properColumnNames[j]] = currentRow.Cells[j].Value
		}

		data = append(data, dataMap)
	}

	return data, properColumnNames, nil

}

func NewUploadFileToEntityPerformer(initConfig *CmsConfig, cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := UploadFileToEntityPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
