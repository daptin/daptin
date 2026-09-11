package actions

import (
	"context"
	"fmt"
	"github.com/artpar/rclone/cmd"
	"github.com/artpar/rclone/fs"
	"github.com/artpar/rclone/fs/filter"
	"github.com/artpar/rclone/fs/operations"
	"github.com/daptin/daptin/server/actionresponse"
	storagefs "github.com/daptin/daptin/server/filesystem"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"strings"

	"github.com/artpar/api2go/v2"
	"github.com/artpar/rclone/fs/config"
	"os"
	"path"
)

type cloudStoreFolderCreateActionPerformer struct {
	cruds map[string]*resource.DbResource
}

func (d *cloudStoreFolderCreateActionPerformer) Name() string {
	return "cloudstore.folder.create"
}

func (d *cloudStoreFolderCreateActionPerformer) DoAction(request actionresponse.Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	responses := make([]actionresponse.ActionResponse, 0)

	atPath, _ := inFields["path"].(string)
	folderName, _ := inFields["name"].(string)
	rootPath := inFields["root_path"].(string)
	folderPath, err := storagefs.ValidatePath(path.Join(atPath, folderName))
	if err != nil {
		return nil, nil, []error{err}
	}
	if folderPath == "" {
		return nil, nil, []error{fmt.Errorf("folder path must be below the storage root")}
	}
	isLocal, err := isLocalCloudStore(inFields)
	if err != nil {
		return nil, nil, []error{err}
	}
	var localFolderPath string
	if isLocal {
		localFolderPath, err = storagefs.ResolveLocalPath(rootPath, folderPath)
		if err != nil {
			return nil, nil, []error{err}
		}
	}
	args := []string{
		rootPath,
	}
	log.Printf("Create folder target %v", folderPath)
	storeName := strings.Split(rootPath, ":")[0]

	credentialName, ok := inFields["credential_name"]
	if ok && credentialName != nil && credentialName != "" {
		cred, err := d.cruds["credential"].GetCredentialByName(credentialName.(string), transaction)
		resource.CheckErr(err, fmt.Sprintf("Failed to get credential for [%s]", credentialName))
		if cred != nil && cred.DataMap != nil {
			for key, val := range cred.DataMap {
				config.Data().SetValue(storeName, key, fmt.Sprintf("%s", val))
			}
		}
	}

	fsrc := cmd.NewFsSrc(args)
	cobraCommand := &cobra.Command{
		Use: fmt.Sprintf("Create folder action at [%v]", folderPath),
	}
	ctx := context.Background()
	newFilter, _ := filter.NewFilter(nil)
	ctx = filter.ReplaceConfig(ctx, newFilter)
	defaultConfig := fs.ConfigInfo{}

	defaultConfig.LogLevel = fs.LogLevelNotice

	go cmd.Run(true, false, cobraCommand, func() error {
		if fsrc == nil {
			log.Errorf("Source or destination is null")
			return nil
		}

		var err error
		if localFolderPath != "" {
			err = os.MkdirAll(localFolderPath, 0777)
		} else {
			err = operations.Mkdir(ctx, fsrc, folderPath)
		}
		if err != nil {
			resource.InfoErr(err, "Failed to sync files for upload to cloud")
			return err
		}
		return err
	})

	restartAttrs := make(map[string]interface{})
	restartAttrs["type"] = "success"
	restartAttrs["message"] = "Cloud storage file upload queued"
	restartAttrs["title"] = "Success"
	actionResponse := resource.NewActionResponse("client.notify", restartAttrs)
	responses = append(responses, actionResponse)

	return nil, responses, nil
}

func NewCloudStoreFolderCreateActionPerformer(cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {

	handler := cloudStoreFolderCreateActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
