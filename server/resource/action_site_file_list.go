package resource

import (
	"errors"
	"github.com/artpar/api2go"
	"github.com/jmoiron/sqlx"
)

type cloudStoreFileListActionPerformer struct {
	cruds map[string]*DbResource
}

func (d *cloudStoreFileListActionPerformer) Name() string {
	return "site.file.list"
}

func (d *cloudStoreFileListActionPerformer) DoAction(request Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []ActionResponse, []error) {

	responses := make([]ActionResponse, 0)

	path := inFields["path"].(string)
	siteReferenceId := inFields["site_id"].(string)

	siteCacheFolder := d.cruds["cloud_store"].SubsiteFolderCache[siteReferenceId]

	if siteCacheFolder == nil {

		restartAttrs := make(map[string]interface{})
		restartAttrs["type"] = "failed"
		restartAttrs["message"] = "Site cache not found"
		restartAttrs["title"] = "Failed"
		actionResponse := NewActionResponse("client.notify", restartAttrs)
		responses = append(responses, actionResponse)

		return nil, responses, []error{errors.New("site not found")}
	}

	contents, _ := siteCacheFolder.GetPathContents(path)

	fileListResponse := NewResponse(nil, api2go.NewApi2GoModelWithData("file", nil, 0, nil, map[string]interface{}{
		"files": contents,
	}), 200, nil)
	responses = append(responses, NewActionResponse("file", map[string]interface{}{
		"list": contents,
	}))

	return fileListResponse, responses, nil
}

func NewCloudStoreFileListActionPerformer(cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := cloudStoreFileListActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
