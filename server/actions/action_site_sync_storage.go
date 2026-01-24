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
	"github.com/artpar/rclone/fs/sync"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	hugoCommand "github.com/gohugoio/hugo/commands"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"strings"
)

type syncSiteStorageActionPerformer struct {
	cruds map[string]*resource.DbResource
}

func (d *syncSiteStorageActionPerformer) Name() string {
	return "site.storage.sync"
}

func (d *syncSiteStorageActionPerformer) DoAction(request actionresponse.Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	responses := make([]actionresponse.ActionResponse, 0)

	cloudStoreId := daptinid.InterfaceToDIR(inFields["cloud_store_id"])
	siteId := daptinid.InterfaceToDIR(inFields["site_id"])
	path := inFields["path"].(string)
	cloudStore, err := d.cruds["cloud_store"].GetCloudStoreByReferenceId(cloudStoreId, transaction)
	if err != nil {
		return nil, nil, []error{err}
	}

	siteCacheFolder, _ := d.cruds["cloud_store"].SubsiteFolderCache(siteId)
	if siteCacheFolder == nil {
		log.Printf("No sub-site cache found on local")
		return nil, nil, []error{errors.New("no site found here")}
	}

	configSetName := cloudStore.Name
	if strings.Index(cloudStore.RootPath, ":") > -1 {
		configSetName = strings.Split(cloudStore.RootPath, ":")[0]
	}
	if cloudStore.CredentialName != "" {
		cred, err := d.cruds["credential"].GetCredentialByName(cloudStore.CredentialName, transaction)
		resource.CheckErr(err, fmt.Sprintf("Failed to get credential for [%s]", cloudStore.CredentialName))
		if cred != nil && cred.DataMap != nil {
			for key, val := range cred.DataMap {
				config.Data().SetValue(configSetName, key, fmt.Sprintf("%s", val))
			}
		}
	}

	tempDirectoryPath := path
	if tempDirectoryPath == "" {
		tempDirectoryPath = siteCacheFolder.LocalSyncPath
	}

	daptinSite, _, err := d.cruds["site"].GetSingleRowByReferenceIdWithTransaction("site", siteId, nil, transaction)
	if err != nil {
		return nil, nil, []error{err}
	}
	is_hugo_site := daptinSite["site_type"] == "hugo"

	path = strings.Trim(siteCacheFolder.Keyname, "/")
	args := []string{
		cloudStore.RootPath + "/" + path,
		tempDirectoryPath,
	}

	fsrc, srcFileName, fdst := cmd.NewFsSrcFileDst(args)
	log.Infof("[84] Temp dir for site sync [%v]/%v ==> %v", cloudStore.Name, cloudStore.RootPath, tempDirectoryPath)
	cobraCommand := &cobra.Command{
		Use: fmt.Sprintf("Sync site storage [%v]", cloudStoreId),
	}

	go cmd.Run(true, false, cobraCommand, func() error {
		if fsrc == nil || fdst == nil {
			log.Errorf("[86] Either source or destination is empty")
			return nil
		}

		ctx := context.Background()
		if fsrc == nil || fdst == nil {
			log.Errorf("Source[%s] or destination[%s] is null", cloudStore.RootPath+path, tempDirectoryPath)
			return nil
		}
		log.Infof("[97] Starting to copy drive for site base from [%v] to [%v]", fsrc.String(), fdst.String())

		defaultConfig := fs.GetConfig(ctx)
		defaultConfig.LogLevel = fs.LogLevelDebug
		//defaultConfig.DeleteMode = fs.DeleteModeAfter
		defaultConfig.AutoConfirm = true
		defaultConfig.NoUpdateModTime = true
		defaultConfig.Inplace = true

		if srcFileName == "" {
			err = sync.Sync(ctx, fdst, fsrc, true)
		} else {
			err = operations.CopyFile(ctx, fdst, fsrc, srcFileName, srcFileName)
		}

		if is_hugo_site && err == nil {
			log.Printf("Starting hugo build for %v", tempDirectoryPath)
			hugoCommandResponse := hugoCommand.Execute([]string{"--source", tempDirectoryPath, "--destination", tempDirectoryPath + "/" + "public", "--verbose", "--verboseLog"})
			log.Printf("Hugo command response for [%v] [%v]: %v", tempDirectoryPath, tempDirectoryPath+"/"+"public", hugoCommandResponse)
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

func NewSyncSiteStorageActionPerformer(cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {

	handler := syncSiteStorageActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
