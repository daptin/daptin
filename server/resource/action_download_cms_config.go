package resource

import (
	"encoding/base64"
	"github.com/artpar/api2go"
	log "github.com/sirupsen/logrus"
)

type DownloadCmsConfigActionPerformer struct {
	responseAttrs map[string]interface{}
}

func (d *DownloadCmsConfigActionPerformer) Name() string {
	return "__download_cms_config"
}

func (d *DownloadCmsConfigActionPerformer) DoAction(request Outcome, inFields map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	responses := make([]ActionResponse, 0)

	actionResponse := NewActionResponse("client.file.download", d.responseAttrs)

	responses = append(responses, actionResponse)

	return nil, responses, nil
}

func NewDownloadCmsConfigPerformer(initConfig *CmsConfig) (ActionPerformerInterface, error) {

	js, err := json.MarshalIndent(*initConfig, "", "  ")
	if err != nil {
		log.Errorf("Failed to marshal initconfig: %v", err)
		return nil, err
	}

	responseAttrs := make(map[string]interface{})
	responseAttrs["content"] = base64.StdEncoding.EncodeToString(js)
	responseAttrs["name"] = "schema.json"
	responseAttrs["contentType"] = "application/json"
	responseAttrs["message"] = "Downloading system schema"

	handler := DownloadCmsConfigActionPerformer{
		responseAttrs: responseAttrs,
	}

	return &handler, nil

}
