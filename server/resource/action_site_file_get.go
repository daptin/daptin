package resource

import (
	"encoding/base64"
	"errors"
	"github.com/daptin/daptin/server/id"
	"io"

	"github.com/artpar/api2go"
	"github.com/jmoiron/sqlx"
)

type cloudStoreFileGetActionPerformer struct {
	cruds map[string]*DbResource
}

func (d *cloudStoreFileGetActionPerformer) Name() string {
	return "site.file.get"
}

func (d *cloudStoreFileGetActionPerformer) DoAction(request Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []ActionResponse, []error) {

	responses := make([]ActionResponse, 0)

	path := inFields["path"].(string)
	id := daptinid.InterfaceToDIR(inFields["site_id"])
	siteCacheFolder := d.cruds["cloud_store"].SubsiteFolderCache[id]
	if siteCacheFolder == nil {

		restartAttrs := make(map[string]interface{})
		restartAttrs["type"] = "failed"
		restartAttrs["message"] = "Site cache not found"
		restartAttrs["title"] = "Failed"
		actionResponse := NewActionResponse("client.notify", restartAttrs)
		responses = append(responses, actionResponse)

		return nil, responses, []error{errors.New("site not found")}
	}

	contents, _ := siteCacheFolder.GetFileByName(path)
	data, _ := io.ReadAll(contents)
	contents.Close()
	dataBase64 := base64.StdEncoding.EncodeToString(data)
	fileListResponse := NewResponse(nil, api2go.NewApi2GoModelWithData("file", nil, 0, nil, map[string]interface{}{
		"data": dataBase64,
	}), 200, nil)
	responses = append(responses, NewActionResponse("file", map[string]interface{}{
		"data": dataBase64,
	}))

	return fileListResponse, responses, nil
}

func NewCloudStoreFileGetActionPerformer(cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := cloudStoreFileGetActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
