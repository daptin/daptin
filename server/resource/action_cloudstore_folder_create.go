package resource

import (
	"context"
	"fmt"
	"github.com/artpar/rclone/cmd"
	"github.com/artpar/rclone/fs"
	"github.com/artpar/rclone/fs/operations"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/artpar/api2go"
	"github.com/artpar/rclone/fs/config"
	"os"
)

type cloudStoreFolderCreateActionPerformer struct {
	cruds map[string]*DbResource
}

func (d *cloudStoreFolderCreateActionPerformer) Name() string {
	return "cloudstore.folder.create"
}

func (d *cloudStoreFolderCreateActionPerformer) DoAction(request Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []ActionResponse, []error) {

	responses := make([]ActionResponse, 0)

	u, _ := uuid.NewV7()
	sourceDirectoryName := "upload-" + u.String()[0:8]
	tempDirectoryPath, err := os.MkdirTemp(os.Getenv("DAPTIN_CACHE_FOLDER"), sourceDirectoryName)
	log.Printf("Temp directory for this upload cloudStoreFolderCreateActionPerformer: %v", tempDirectoryPath)

	//defer os.RemoveAll(tempDirectoryPath) // clean up

	CheckErr(err, "Failed to create temp tempDirectoryPath for rclone upload")
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
	storeName := inFields["name"].(string)

	credentialName, ok := inFields["credential_name"]
	if ok && credentialName != nil && credentialName != "" {
		cred, err := d.cruds["credential"].GetCredentialByName(credentialName.(string), transaction)
		CheckErr(err, fmt.Sprintf("Failed to get credential for [%s]", credentialName))
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
	defaultConfig := fs.GetConfig(nil)
	defaultConfig.LogLevel = fs.LogLevelNotice

	go cmd.Run(true, false, cobraCommand, func() error {
		if fsrc == nil {
			log.Errorf("Source or destination is null")
			return nil
		}

		ctx := context.Background()

		err := operations.Mkdir(ctx, fsrc, folderPath)
		if err != nil {
			InfoErr(err, "Failed to sync files for upload to cloud")
			return err
		}
		err = os.RemoveAll(tempDirectoryPath)
		if err != nil {
			InfoErr(err, "Failed to remove temp directory after folder create")
		}
		return err
	})

	restartAttrs := make(map[string]interface{})
	restartAttrs["type"] = "success"
	restartAttrs["message"] = "Cloud storage file upload queued"
	restartAttrs["title"] = "Success"
	actionResponse := NewActionResponse("client.notify", restartAttrs)
	responses = append(responses, actionResponse)

	return nil, responses, nil
}

func NewCloudStoreFolderCreateActionPerformer(cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := cloudStoreFolderCreateActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
