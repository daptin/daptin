package actions

import (
	"context"
	"errors"
	"fmt"
	"github.com/artpar/api2go/v2"
	"github.com/artpar/rclone/cmd"
	"github.com/artpar/rclone/fs"
	"github.com/artpar/rclone/fs/config"
	"github.com/artpar/rclone/fs/filter"
	"github.com/artpar/rclone/fs/operations"
	"github.com/daptin/daptin/server/actionresponse"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
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
	var err error

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
	newFilter, _ := filter.NewFilter(nil)
	ctx = filter.ReplaceConfig(ctx, newFilter)
	defaultConfig := fs.ConfigInfo{}
	defaultConfig.LogLevel = fs.LogLevelNotice

	go cmd.Run(true, false, cobraCommand, func() error {
		if fsrc == nil {
			log.Errorf("path is null for delete operation")
			return nil
		}

		err = operations.Delete(ctx, fsrc)
		if err != nil {
			err = operations.Purge(ctx, fsrc, "")
		}

		resource.InfoErr(err, "Failed to delete purge path [%v] in cloud store", rootPath)
		return err
	})

	restartAttrs := make(map[string]interface{})
	restartAttrs["type"] = "success"
	restartAttrs["message"] = "Cloud storage path deleted"
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
