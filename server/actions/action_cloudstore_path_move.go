package actions

import (
	"context"
	"fmt"
	"github.com/artpar/rclone/cmd"
	"github.com/artpar/rclone/fs"
	"github.com/artpar/rclone/fs/operations"
	"github.com/artpar/rclone/fs/sync"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"path/filepath"
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

	// For MoveFile, args[1] must be the destination DIRECTORY (parent of destination file),
	// NOT the full destination file path. We'll pass the filename separately as destFileName.
	destParentDir := filepath.Dir(rootPath + destinationPath)

	args := []string{
		rootPath + sourcePath, // Source file full path
		destParentDir,         // Destination parent directory
	}
	log.Printf("Create move %v to %v (destParent: %v)", sourcePath, destinationPath, destParentDir)

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

	cobraCommand := &cobra.Command{
		Use: fmt.Sprintf("File move action from [%v] to [%v]", sourcePath, destinationPath),
	}
	ctx := context.Background()
	// Don't use filter - it interferes with MoveFile operation
	defaultConfig := fs.ConfigInfo{}
	defaultConfig.LogLevel = fs.LogLevelNotice

	cmd.Run(true, true, cobraCommand, func() error {
		// Create fsrc and fdst inside the callback to ensure fresh context
		fsrc, srcFileName, fdst := cmd.NewFsSrcFileDst(args)

		// Extract destination filename from the destination path
		destFileName := filepath.Base(destinationPath)
		if destFileName == "" || destFileName == "/" || destFileName == "." {
			// If destination is a directory, keep original filename
			destFileName = srcFileName
		}

		srcFullPath := rootPath + sourcePath
		dstFullPath := rootPath + destinationPath

		// For local filesystem, use OS operations directly (rclone has issues with MoveFile)
		if !strings.Contains(rootPath, ":") {
			log.Infof("Using OS rename for local filesystem: %v -> %v", srcFullPath, dstFullPath)
			err := os.Rename(srcFullPath, dstFullPath)
			if err != nil {
				log.Errorf("OS rename failed: %v", err)
				return err
			}
			log.Infof("Move operation completed successfully (OS level)")
			return nil
		}

		// For remote cloud storage, use rclone operations
		log.Infof("Inside callback - fsrc: %v, srcFileName: %v", fsrc, srcFileName)
		log.Infof("Inside callback - fdst: %v, destFileName: %v", fdst, destFileName)
		var err error
		if srcFileName == "" {
			// Moving entire directory
			log.Infof("Using MoveDir for directory move")
			err = sync.MoveDir(ctx, fdst, fsrc, false, true)
		} else {
			// Moving/renaming file
			log.Infof("Using MoveFile: srcRemote=%v, dstRemote=%v, srcFile=%v, dstFile=%v",
				fsrc, fdst, srcFileName, destFileName)
			err = operations.MoveFile(ctx, fdst, fsrc, srcFileName, destFileName)
		}

		if err != nil {
			log.Errorf("Move operation failed: %v", err)
			resource.InfoErr(err, "Failed to move file in cloud storage")
			err = os.RemoveAll(tempDirectoryPath)
			resource.InfoErr(err, "Failed to remove temp directory after path move")
			return nil
		}
		log.Infof("Move operation completed successfully (rclone)")
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
