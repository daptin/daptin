package actions

import (
	"context"
	"errors"
	"fmt"
	"github.com/artpar/api2go/v2"
	"github.com/artpar/rclone/cmd"
	"github.com/artpar/rclone/fs"
	"github.com/artpar/rclone/fs/config"
	"github.com/artpar/rclone/fs/operations"
	"github.com/daptin/daptin/server/actionresponse"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type cloudStoreFileDeleteActionPerformer struct {
	cruds map[string]*resource.DbResource
}

func (d *cloudStoreFileDeleteActionPerformer) Name() string {
	return "site.file.delete"
}

func (d *cloudStoreFileDeleteActionPerformer) DoAction(request actionresponse.Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	responses := make([]actionresponse.ActionResponse, 0)

	log.Infof("[DELETE DEBUG] inFields: %+v", inFields)

	atPath, ok := inFields["path"].(string)
	if !ok {
		return nil, nil, []error{errors.New("path is missing")}
	}

	// Get root_path either directly or via site_id lookup
	rootPath, ok := inFields["root_path"].(string)
	var siteCredentials map[string]interface{}
	if !ok || rootPath == "" {
		// Try to get root_path from site_id via SubsiteFolderCache
		siteId := daptinid.InterfaceToDIR(inFields["site_id"])
		if siteId == daptinid.NullReferenceId {
			return nil, nil, []error{errors.New("root_path or valid site_id is required")}
		}

		siteCacheFolder, found := d.cruds["cloud_store"].SubsiteFolderCache(siteId)
		if !found || siteCacheFolder == nil {
			return nil, nil, []error{errors.New("site cache not found")}
		}

		rootPath = siteCacheFolder.CloudStore.RootPath
		if siteCacheFolder.Keyname != "" {
			if !EndsWithCheck(rootPath, "/") {
				rootPath = rootPath + "/"
			}
			rootPath = rootPath + siteCacheFolder.Keyname
		}
		siteCredentials = siteCacheFolder.Credentials
	}

	if atPath != "" {
		if !EndsWithCheck(rootPath, "/") && !resource.BeginsWith(atPath, "/") {
			rootPath = rootPath + "/"
		}
		rootPath = rootPath + atPath
	}
	rootPath = path.Clean(rootPath)
	args := []string{
		rootPath,
	}
	log.Infof("[49] Delete target path: %v", rootPath)

	// Set credentials from inFields or from site cache
	credentialName, ok := inFields["credential_name"]
	storeName := strings.Split(rootPath, ":")[0]
	if ok && credentialName != nil && credentialName != "" {
		cred, err := d.cruds["credential"].GetCredentialByName(credentialName.(string), transaction)
		resource.CheckErr(err, fmt.Sprintf("Failed to get credential for [%s]", credentialName))
		if cred != nil && cred.DataMap != nil {
			for key, val := range cred.DataMap {
				config.Data().SetValue(storeName, key, fmt.Sprintf("%s", val))
			}
		}
	} else if siteCredentials != nil {
		// Use credentials from site cache
		for key, val := range siteCredentials {
			config.Data().SetValue(storeName, key, fmt.Sprintf("%s", val))
		}
	}

	fsrc := cmd.NewFsSrc(args)
	cobraCommand := &cobra.Command{
		Use: fmt.Sprintf("Delete file action at [%v]", atPath),
	}
	ctx := context.Background()
	// Don't use filter - the rootPath already specifies what to delete
	// Using a filter can interfere with directory deletion
	defaultConfig := fs.ConfigInfo{}
	defaultConfig.LogLevel = fs.LogLevelNotice

	// Execute delete asynchronously
	go cmd.Run(true, false, cobraCommand, func() error {
		if fsrc == nil {
			log.Errorf("path is null for delete operation")
			return errors.New("delete path is null")
		}

		var err error
		if strings.Contains(rootPath, ":") {
			// Remote storage (S3, MinIO, etc.)
			// Detect if path is a directory (ends with / or has no extension)
			isDirectory := strings.HasSuffix(atPath, "/") ||
				(atPath != "" && !strings.Contains(filepath.Base(atPath), "."))

			if isDirectory {
				// Use Purge for directories
				log.Infof("Purging directory: %v", rootPath)
				err = operations.Purge(ctx, fsrc, "")
				if err != nil {
					log.Errorf("Purge failed for directory [%v]: %v", rootPath, err)
					return err
				}
				log.Infof("Directory purged successfully: %v", rootPath)
			} else {
				// Use Delete for files
				log.Infof("Deleting file: %v", rootPath)
				err = operations.Delete(ctx, fsrc)
				if err != nil {
					log.Errorf("Delete failed for file [%v]: %v", rootPath, err)
					return err
				}
				log.Infof("File deleted successfully: %v", rootPath)
			}
		} else {
			// Local filesystem
			absPath := rootPath
			if !filepath.IsAbs(rootPath) {
				absPath, _ = filepath.Abs(rootPath)
			}
			stat, statErr := os.Stat(absPath)
			if statErr != nil {
				log.Errorf("Cannot stat path [%v]: %v", absPath, statErr)
				return statErr
			}

			if stat.IsDir() {
				// Directory - use os.RemoveAll for complete removal
				log.Infof("Removing directory tree: %v", absPath)
				err = os.RemoveAll(absPath)
				if err != nil {
					log.Errorf("Failed to remove directory tree [%v]: %v", absPath, err)
					return err
				}
				log.Infof("Directory removed successfully: %v", absPath)
			} else {
				// File - use os.Remove
				log.Infof("Deleting file: %v", absPath)
				err = os.Remove(absPath)
				if err != nil {
					log.Errorf("Failed to delete file [%v]: %v", absPath, err)
					return err
				}
				log.Infof("File deleted successfully: %v", absPath)
			}
		}

		log.Infof("Delete operation completed successfully for path: %v", rootPath)
		return nil
	})

	// Return immediately with queued status
	restartAttrs := make(map[string]interface{})
	restartAttrs["type"] = "success"
	restartAttrs["message"] = "Cloud storage deletion queued"
	restartAttrs["title"] = "Success"
	actionResponse := resource.NewActionResponse("client.notify", restartAttrs)
	responses = append(responses, actionResponse)

	return nil, responses, nil
}

func NewCloudStoreFileDeleteActionPerformer(cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {

	handler := cloudStoreFileDeleteActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
