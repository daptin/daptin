package resource

import (
	"encoding/base64"
	"fmt"
	"github.com/gin-gonic/gin/json"
	log "github.com/sirupsen/logrus"
	"github.com/artpar/api2go"
)

type ExportDataPerformer struct {
	cmsConfig *CmsConfig
	cruds     map[string]*DbResource
}

func (d *ExportDataPerformer) Name() string {
	return "__data_export"
}

func (d *ExportDataPerformer) DoAction(request ActionRequest, inFields map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	responses := make([]ActionResponse, 0)

	subjectInstance, ok := inFields["subject"]

	finalName := "complete"

	var finalString []byte
	result := make(map[string]interface{})

	if ok && subjectInstance != nil {

		subjectMap := subjectInstance.(map[string]interface{})
		tableName := subjectMap["table_name"].(string)
		log.Infof("Export data for table: %v", tableName)

		objects, err := d.cruds[tableName].GetAllRawObjects(tableName)
		if err != nil {
			log.Errorf("Failed to get all objects of type [%v] : %v", tableName, err)
		}

		result[tableName] = objects
		finalName = tableName
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

	handler := ExportDataPerformer{
		cmsConfig: initConfig,
		cruds:     cruds,
	}

	return &handler, nil

}
