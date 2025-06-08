package actions

import (
	"context"
	"fmt"
	"github.com/artpar/rclone/cmd"
	"github.com/artpar/rclone/fs"
	"github.com/artpar/rclone/fs/filter"
	"github.com/artpar/rclone/fs/operations"
	"github.com/artpar/rclone/fs/sync"
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

type cloudStorePathMoveActionPerformer struct {
	cruds map[string]*resource.DbResource
}

func (d *cloudStorePathMoveActionPerformer) Name() string {
	return "cloudstore.path.move"
}

func (d *cloudStorePathMoveActionPerformer) DoAction(request actionresponse.Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	responses := make([]actionresponse.ActionResponse, 0)

	u, _ := uuid.NewV7()
	sourceDirectoryName := "upload-" + u.String()[0:8]
	tempDirectoryPath, err := os.MkdirTemp(os.Getenv("DAPTIN_CACHE_FOLDER"), sourceDirectoryName)
	log.Debugf("Temp directory for upload cloudStorePathMoveActionPerformer: %v", tempDirectoryPath)

	//defer os.RemoveAll(tempDirectoryPath) // clean up

	resource.CheckErr(err, "Failed to create temp tempDirectoryPath for rclone upload")
	sourcePath, _ := inFields["source"].(string)
	destinationPath, _ := inFields["destination"].(string)
	rootPath := inFields["root_path"].(string)

	if len(sourcePath) > 0 && sourcePath[0] != '/' {
		sourcePath = "/" + sourcePath
	}

	if len(destinationPath) > 0 && destinationPath[0] != '/' {
		destinationPath = "/" + destinationPath
	}

	args := []string{
		rootPath + sourcePath,
		rootPath + destinationPath,
	}
	log.Printf("Create move %v %v", sourcePath, destinationPath)

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

	fsrc, srcFileName, fdst := cmd.NewFsSrcFileDst(args)
	cmd.Run(true, true, cobraCommand, func() error {
		var err error
		if srcFileName == "" {
			err = sync.MoveDir(ctx, fdst, fsrc, false, true)
		} else {
			err = operations.MoveFile(ctx, fdst, fsrc, srcFileName, srcFileName)
		}

		if err != nil {
			resource.InfoErr(err, "Failed to sync files for upload to cloud")
			err = os.RemoveAll(tempDirectoryPath)
			resource.InfoErr(err, "Failed to remove temp directory after path move")
			return nil
		}
		return err
	})

	restartAttrs := make(map[string]interface{})
	restartAttrs["type"] = "success"
	restartAttrs["message"] = "Cloud storage path moved"
	restartAttrs["title"] = "Success"
	actionResponse := resource.NewActionResponse("client.notify", restartAttrs)
	responses = append(responses, actionResponse)

	return nil, responses, nil
}

func NewCloudStorePathMoveActionPerformer(cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {

	handler := cloudStorePathMoveActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
