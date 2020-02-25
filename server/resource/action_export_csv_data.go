package resource

import (
	"encoding/base64"
	"encoding/csv"
	"fmt"
	"github.com/artpar/api2go"
	"github.com/gocarina/gocsv"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"time"
)

type ExportCsvDataPerformer struct {
	cmsConfig *CmsConfig
	cruds     map[string]*DbResource
}

func (d *ExportCsvDataPerformer) Name() string {
	return "__csv_data_export"
}

func (d *ExportCsvDataPerformer) DoAction(request Outcome, inFields map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	responses := make([]ActionResponse, 0)

	tableName, ok := inFields["table_name"]

	finalName := "complete"

	result := make(map[string]interface{})

	if ok && tableName != nil {

		tableNameStr := tableName.(string)
		log.Infof("Export data for table: %v", tableNameStr)

		objects, err := d.cruds[tableNameStr].GetAllRawObjects(tableNameStr)
		if err != nil {
			log.Errorf("Failed to get all objects of type [%v] : %v", tableNameStr, err)
		}

		result[tableNameStr] = objects
		finalName = tableNameStr
	} else {

		for _, tableInfo := range d.cmsConfig.Tables {
			data, err := d.cruds[tableInfo.TableName].GetAllRawObjects(tableInfo.TableName)
			if err != nil {
				log.Errorf("Failed to export objects of type [%v]: %v", tableInfo.TableName, err)
				continue
			}
			result[tableInfo.TableName] = data
		}

	}

	currentDate := time.Now()
	prefix := currentDate.Format("2006-01-02-15-04-05")
	csvFile, err := ioutil.TempFile("", prefix)

	for outTableName, contents := range result {

		if tableName != nil {
			csvFile.WriteString(outTableName)
		}
		csvFileWriter := csv.NewWriter(csvFile)
		contentArray := contents.([]map[string]interface{})

		if len(contentArray) == 0 {
			csvFile.WriteString("No data\n")
		}

		var columnKeys []string
		csvWriter := gocsv.NewSafeCSVWriter(csvFileWriter)
		firstRow := contentArray[0]

		for colName := range firstRow {
			columnKeys = append(columnKeys, colName)
		}

		csvWriter.Write(columnKeys)

		for _, row := range contentArray {
			var dataRow []string
			for _, colName := range columnKeys {
				dataRow = append(dataRow, fmt.Sprintf("%v", row[colName]))
			}
			csvWriter.Write(dataRow)
		}
		csvFile.WriteString("\n")
	}

	csvFile.Close()

	csvFileName := csvFile.Name()
	csvFileContents, err := ioutil.ReadFile(csvFileName)
	if InfoErr(err, "Failed to read csv file to download") {
		actionResponse := NewActionResponse("client.notify", NewClientNotification("error", "Failed to generate csv: "+err.Error(), "Failed"))
		responses = append(responses, actionResponse)
		return nil, responses, nil
	}

	responseAttrs := make(map[string]interface{})
	responseAttrs["content"] = base64.StdEncoding.EncodeToString(csvFileContents)
	responseAttrs["name"] = fmt.Sprintf("daptin_dump_%v.csv", finalName)
	responseAttrs["contentType"] = "application/csv"
	responseAttrs["message"] = "Downloading csv "

	actionResponse := NewActionResponse("client.file.download", responseAttrs)

	responses = append(responses, actionResponse)

	return nil, responses, nil
}

func NewExportCsvDataPerformer(initConfig *CmsConfig, cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := ExportCsvDataPerformer{
		cmsConfig: initConfig,
		cruds:     cruds,
	}

	return &handler, nil

}
