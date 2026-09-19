package actions

import (
	"context"
	"fmt"
	"github.com/artpar/rclone/cmd"
	"github.com/artpar/rclone/fs/operations"
	"github.com/artpar/rclone/fs/sync"
	"github.com/daptin/daptin/server/actionresponse"
	storagefs "github.com/daptin/daptin/server/filesystem"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"path/filepath"

	"github.com/artpar/api2go/v2"
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

	// MoveFile uses the parent directory; MoveDir uses the full destination path.
	destParentDir := filepath.Dir(dstFullPath)

	log.Printf("Create move %v to %v (destParent: %v)", sourcePath, destinationPath, destParentDir)

	credentialName, _ := inFields["credential_name"].(string)
	if err := resource.ConfigureCloudStoreCredential(d.cruds["credential"], credentialName, rootPath, !isLocal, transaction); err != nil {
		return nil, nil, []error{err}
	}

	ctx := context.Background()
	if isLocal {
		if err := os.Rename(srcFullPath, dstFullPath); err != nil {
			return nil, nil, []error{err}
		}
		if _, err := os.Stat(dstFullPath); err != nil {
			return nil, nil, []error{err}
		}
	} else {
		fsrc, srcFileName := cmd.NewFsFile(srcFullPath)
		if fsrc == nil {
			return nil, nil, []error{fmt.Errorf("failed to open cloud storage move path")}
		}
		destRoot := destParentDir
		if srcFileName == "" {
			entries, listErr := fsrc.List(ctx, "")
			if listErr != nil {
				return nil, nil, []error{listErr}
			}
			if len(entries) == 0 {
				return nil, nil, []error{fmt.Errorf("move source not found: %s", sourcePath)}
			}
			destRoot = dstFullPath
		}
		fdst := cmd.NewFsDir([]string{destRoot})
		if fdst == nil {
			return nil, nil, []error{fmt.Errorf("failed to open cloud storage destination")}
		}
		destFileName := filepath.Base(destinationPath)
		if destFileName == "" || destFileName == "/" || destFileName == "." {
			destFileName = srcFileName
		}
		if srcFileName == "" {
			err = sync.MoveDir(ctx, fdst, fsrc, false, true)
			if err == nil {
				entries, listErr := fdst.List(ctx, "")
				if listErr != nil {
					err = listErr
				} else if len(entries) == 0 {
					err = fmt.Errorf("move destination is empty: %s", destinationPath)
				}
			}
		} else {
			err = operations.MoveFile(ctx, fdst, fsrc, destFileName, srcFileName)
			if err == nil {
				_, err = fdst.NewObject(ctx, destFileName)
			}
		}
		if err != nil {
			return nil, nil, []error{err}
		}
	}

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
