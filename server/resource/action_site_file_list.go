package resource

import (
	"errors"
	"github.com/artpar/api2go"
)

type CloudStoreFileListActionPerformer struct {
	cruds map[string]*DbResource
}

func (d *CloudStoreFileListActionPerformer) Name() string {
	return "site.file.list"
}

func (d *CloudStoreFileListActionPerformer) DoAction(request Outcome, inFields map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

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

	handler := CloudStoreFileListActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
