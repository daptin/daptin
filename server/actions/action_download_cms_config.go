package actions

import (
	"encoding/base64"
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

type downloadCmsConfigActionPerformer struct {
	responseAttrs map[string]interface{}
}

func (d *downloadCmsConfigActionPerformer) Name() string {
	return "__download_cms_config"
}

func (d *downloadCmsConfigActionPerformer) DoAction(request actionresponse.Outcome, inFields map[string]interface{},
	transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	responses := make([]actionresponse.ActionResponse, 0)

	actionResponse := resource.NewActionResponse("client.file.download", d.responseAttrs)

	responses = append(responses, actionResponse)

	return nil, responses, nil
}

func NewDownloadCmsConfigPerformer(initConfig *resource.CmsConfig) (actionresponse.ActionPerformerInterface, error) {

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

	handler := downloadCmsConfigActionPerformer{
		responseAttrs: responseAttrs,
	}

	return &handler, nil

}
