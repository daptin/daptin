package resource

import (
	"errors"
	"github.com/artpar/go.uuid"
	log "github.com/sirupsen/logrus"
	"io/ioutil"

	"github.com/artpar/api2go"
)

type CloudStoreFileDeleteActionPerformer struct {
	cruds map[string]*DbResource
}

func (d *CloudStoreFileDeleteActionPerformer) Name() string {
	return "site.file.delete"
}

func (d *CloudStoreFileDeleteActionPerformer) DoAction(request Outcome, inFields map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	responses := make([]ActionResponse, 0)

	u, _ := uuid.NewV4()
	sourceDirectoryName := u.String()
	tempDirectoryPath, _ := ioutil.TempDir("", sourceDirectoryName)
	log.Infof("Temp directory for this upload: %v", tempDirectoryPath)

	//defer os.RemoveAll(tempDirectoryPath) // clean up

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

	err := siteCacheFolder.DeleteFileByName(path)

	fileListResponse := NewResponse(nil, api2go.NewApi2GoModelWithData("file_delete", nil, 0, nil, map[string]interface{}{
		"error": err,
	}), 200, nil)
	responses = append(responses, NewActionResponse("file_delete", map[string]interface{}{
		"error": err,
	}))

	return fileListResponse, responses, nil
}

func NewCloudStoreFileDeleteActionPerformer(cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := CloudStoreFileDeleteActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
