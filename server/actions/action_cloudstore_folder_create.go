package actions

import (
	"context"
	"fmt"
	"github.com/artpar/rclone/cmd"
	"github.com/artpar/rclone/fs"
	"github.com/artpar/rclone/fs/filter"
	"github.com/artpar/rclone/fs/operations"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"strings"

	"github.com/artpar/api2go/v2"
	"github.com/artpar/rclone/fs/config"
	"os"
)

type cloudStoreFolderCreateActionPerformer struct {
	cruds map[string]*resource.DbResource
}

func (d *cloudStoreFolderCreateActionPerformer) Name() string {
	return "cloudstore.folder.create"
}

func (d *cloudStoreFolderCreateActionPerformer) DoAction(request actionresponse.Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	responses := make([]actionresponse.ActionResponse, 0)

	u, _ := uuid.NewV7()
	sourceDirectoryName := "upload-" + u.String()[0:8]
	tempDirectoryPath, err := os.MkdirTemp(os.Getenv("DAPTIN_CACHE_FOLDER"), sourceDirectoryName)
	log.Printf("Temp directory for this upload cloudStoreFolderCreateActionPerformer: %v", tempDirectoryPath)

	//defer os.RemoveAll(tempDirectoryPath) // clean up

	resource.CheckErr(err, "Failed to create temp tempDirectoryPath for rclone upload")
	atPath, _ := inFields["path"].(string)
	folderName, _ := inFields["name"].(string)
	rootPath := inFields["root_path"].(string)

	if len(atPath) > 0 && atPath[len(atPath)-1] != '/' {
		atPath = atPath + "/"
	}

	folderPath := atPath + folderName
	args := []string{
		rootPath,
	}
	log.Printf("Create folder target %v", folderPath)
	storeName := strings.Split(rootPath, ":")[0]

	credentialName, ok := inFields["credential_name"]
	if ok && credentialName != nil && credentialName != "" {
		cred, err := d.cruds["credential"].GetCredentialByName(credentialName.(string), transaction)
		resource.CheckErr(err, fmt.Sprintf("Failed to get credential for [%s]", credentialName))
		if cred.DataMap != nil {
			for key, val := range cred.DataMap {
				config.Data().SetValue(storeName, key, fmt.Sprintf("%s", val))
			}
		}
	}

	fsrc := cmd.NewFsSrc(args)
	cobraCommand := &cobra.Command{
		Use: fmt.Sprintf("File upload action from [%v]", tempDirectoryPath),
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

		err := operations.Mkdir(ctx, fsrc, folderPath)
		if err != nil {
			resource.InfoErr(err, "Failed to sync files for upload to cloud")
			return err
		}
		err = os.RemoveAll(tempDirectoryPath)
		if err != nil {
			resource.InfoErr(err, "Failed to remove temp directory after folder create")
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
