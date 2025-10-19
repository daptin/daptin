package actions

import (
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"io"

	"github.com/artpar/api2go/v2"
	"github.com/jmoiron/sqlx"
)

type cloudStoreFileGetActionPerformer struct {
	cruds map[string]*resource.DbResource
}

func (d *cloudStoreFileGetActionPerformer) Name() string {
	return "site.file.get"
}

func (d *cloudStoreFileGetActionPerformer) DoAction(request actionresponse.Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

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

	contents, _ := siteCacheFolder.GetFileByName(path)
	defer contents.Close()

	// Read with size limit protection (max 10MB for API responses)
	limitedReader := io.LimitReader(contents, 10*1024*1024+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, responses, []error{err}
	}

	// Check if file was too large
	if len(data) > 10*1024*1024 {
		return nil, responses, []error{fmt.Errorf("file too large: %d bytes > 10MB limit", len(data))}
	}
	dataBase64 := base64.StdEncoding.EncodeToString(data)
	fileListResponse := resource.NewResponse(nil, api2go.NewApi2GoModelWithData("file", nil, 0, nil, map[string]interface{}{
		"data": dataBase64,
	}), 200, nil)
	responses = append(responses, resource.NewActionResponse("file", map[string]interface{}{
		"data": dataBase64,
	}))

	return fileListResponse, responses, nil
}

func NewCloudStoreFileGetActionPerformer(cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {

	handler := cloudStoreFileGetActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
