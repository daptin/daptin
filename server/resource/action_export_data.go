package resource

import (
	"encoding/base64"
	"fmt"
	"github.com/artpar/api2go"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

type exportDataPerformer struct {
	cmsConfig *CmsConfig
	cruds     map[string]*DbResource
}

func (d *exportDataPerformer) Name() string {
	return "__data_export"
}

func (d *exportDataPerformer) DoAction(request Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []ActionResponse, []error) {

	responses := make([]ActionResponse, 0)

	tableName, ok := inFields["table_name"]

	finalName := "complete"

	var finalString []byte
	result := make(map[string]interface{})

	if ok && tableName != nil {

		tableNameStr := tableName.(string)
		log.Printf("Export data for table: %v", tableNameStr)

		objects, err := d.cruds[tableNameStr].GetAllRawObjectsWithTransaction(tableNameStr, transaction)
		if err != nil {
			log.Errorf("Failed to get all objects of type [%v] : %v", tableNameStr, err)
		}

		result[tableNameStr] = objects
		finalName = tableNameStr
	} else {

		for _, tableInfo := range d.cmsConfig.Tables {
			data, err := d.cruds[tableInfo.TableName].GetAllRawObjectsWithTransaction(tableInfo.TableName, transaction)
			if err != nil {
				log.Errorf("Failed to export objects of type [%v]: %v", tableInfo.TableName, err)
				continue
			}
			result[tableInfo.TableName] = data
		}

	}

	finalString, err := json.Marshal(result)
	if err != nil {
		log.Errorf("Failed to marshal objects as json: %v", err)
	}

	responseAttrs := make(map[string]interface{})
	responseAttrs["content"] = base64.StdEncoding.EncodeToString(finalString)
	responseAttrs["name"] = fmt.Sprintf("daptin_dump_%v.json", finalName)
	responseAttrs["contentType"] = "application/json"
	responseAttrs["message"] = "Downloading data"

	actionResponse := NewActionResponse("client.file.download", responseAttrs)

	responses = append(responses, actionResponse)

	return nil, responses, nil
}

func NewExportDataPerformer(initConfig *CmsConfig, cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := exportDataPerformer{
		cmsConfig: initConfig,
		cruds:     cruds,
	}

	return &handler, nil

}
