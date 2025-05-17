package actions

import (
	"errors"
	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
)

type cloudStoreFileListActionPerformer struct {
	cruds map[string]*resource.DbResource
}

func (d *cloudStoreFileListActionPerformer) Name() string {
	return "site.file.list"
}

func (d *cloudStoreFileListActionPerformer) DoAction(request actionresponse.Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	responses := make([]actionresponse.ActionResponse, 0)

	path := inFields["path"].(string)
	id := daptinid.InterfaceToDIR(inFields["site_id"])
	siteCacheFolder, _ := d.cruds["cloud_store"].SubsiteFolderCache(id)

	if siteCacheFolder == nil {

		restartAttrs := make(map[string]interface{})
		restartAttrs["type"] = "failed"
		restartAttrs["message"] = "Site cache not found"
		restartAttrs["title"] = "Failed"
		actionResponse := resource.NewActionResponse("client.notify", restartAttrs)
		responses = append(responses, actionResponse)

		return nil, responses, []error{errors.New("site not found")}
	}

	contents, _ := siteCacheFolder.GetPathContents(path)

	fileListResponse := resource.NewResponse(nil, api2go.NewApi2GoModelWithData("file", nil, 0, nil, map[string]interface{}{
		"files": contents,
	}), 200, nil)
	responses = append(responses, resource.NewActionResponse("file", map[string]interface{}{
		"list": contents,
	}))

	return fileListResponse, responses, nil
}

func NewCloudStoreFileListActionPerformer(cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {

	handler := cloudStoreFileListActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
