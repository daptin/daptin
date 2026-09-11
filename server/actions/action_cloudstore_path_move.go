package actions

import (
	"context"
	"fmt"
	"github.com/artpar/rclone/cmd"
	"github.com/artpar/rclone/fs"
	"github.com/artpar/rclone/fs/operations"
	"github.com/artpar/rclone/fs/sync"
	"github.com/daptin/daptin/server/actionresponse"
	storagefs "github.com/daptin/daptin/server/filesystem"
	"github.com/daptin/daptin/server/resource"
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

	sourcePath, _ := inFields["source"].(string)
	destinationPath, _ := inFields["destination"].(string)
	rootPath := inFields["root_path"].(string)
	var err error
	sourcePath, err = storagefs.ValidatePath(sourcePath)
	if err != nil {
		return nil, nil, []error{err}
	}
	destinationPath, err = storagefs.ValidatePath(destinationPath)
	if err != nil {
		return nil, nil, []error{err}
	}
	if sourcePath == "" || destinationPath == "" {
		return nil, nil, []error{fmt.Errorf("move source and destination must be below the storage root")}
	}

	var srcFullPath, dstFullPath string
	isLocal, err := isLocalCloudStore(inFields)
	if err != nil {
		return nil, nil, []error{err}
	}
	if isLocal {
		srcFullPath, err = storagefs.ResolveLocalPath(rootPath, sourcePath)
		if err == nil {
			dstFullPath, err = storagefs.ResolveLocalPath(rootPath, destinationPath)
		}
	} else {
		srcFullPath, err = storagefs.ResolvePath(rootPath, sourcePath)
		if err == nil {
			dstFullPath, err = storagefs.ResolvePath(rootPath, destinationPath)
		}
	}
	if err != nil {
		return nil, nil, []error{err}
	}

	// For MoveFile, args[1] must be the destination DIRECTORY (parent of destination file),
	// NOT the full destination file path. We'll pass the filename separately as destFileName.
	destParentDir := filepath.Dir(dstFullPath)

	args := []string{
		srcFullPath,   // Source file full path
		destParentDir, // Destination parent directory
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

		// For local filesystem, use OS operations directly (rclone has issues with MoveFile)
		if isLocal {
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
