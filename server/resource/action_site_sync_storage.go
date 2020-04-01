package resource

import (
	"context"
	"fmt"
	"github.com/artpar/rclone/cmd"
	"github.com/artpar/rclone/fs"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/artpar/api2go"
	"github.com/artpar/rclone/fs/config"
	"github.com/artpar/rclone/fs/sync"
	"strings"
)

type SyncSiteStorageActionPerformer struct {
	cruds map[string]*DbResource
}

func (d *SyncSiteStorageActionPerformer) Name() string {
	return "site.storage.sync"
}

func (d *SyncSiteStorageActionPerformer) DoAction(request Outcome, inFields map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	responses := make([]ActionResponse, 0)

	cloudStoreId := inFields["cloud_store_id"].(string)
	tempDirectoryPath := inFields["path"].(string)
	cloudStore, err := d.cruds["cloud_store"].GetCloudStoreByReferenceId(cloudStoreId)
	if err != nil {
		return nil, nil, []error{err}
	}

	oauthTokenId := cloudStore.OAutoTokenId

	token, oauthConf, err := d.cruds["oauth_token"].GetTokenByTokenReferenceId(oauthTokenId)
	CheckErr(err, "Failed to get oauth2 token for storage sync")

	jsonToken, err := json.Marshal(token)
	CheckErr(err, "Failed to convert token to json")
	config.FileSet(cloudStore.StoreProvider, "client_id", oauthConf.ClientID)
	config.FileSet(cloudStore.StoreProvider, "type", cloudStore.StoreProvider)
	config.FileSet(cloudStore.StoreProvider, "client_secret", oauthConf.ClientSecret)
	config.FileSet(cloudStore.StoreProvider, "token", string(jsonToken))
	config.FileSet(cloudStore.StoreProvider, "client_scopes", strings.Join(oauthConf.Scopes, ","))
	config.FileSet(cloudStore.StoreProvider, "redirect_url", oauthConf.RedirectURL)

	args := []string{
		cloudStore.RootPath,
		tempDirectoryPath,
	}

	fsrc, fdst := cmd.NewFsSrcDst(args)
	log.Infof("Temp dir for site [%v]/%v ==> %v", cloudStore.Name, cloudStore.RootPath, tempDirectoryPath)
	cobraCommand := &cobra.Command{
		Use: fmt.Sprintf("Sync site storage [%v]", cloudStoreId),
	}
	fs.Config.LogLevel = fs.LogLevelNotice

	go cmd.Run(true, false, cobraCommand, func() error {
		if fsrc == nil || fdst == nil {
			log.Errorf("Either source or destination is empty")
			return nil
		}

		ctx := context.Background()
		log.Infof("Starting to copy drive for site base from [%v] to [%v]", fsrc.String(), fdst.String())
		if fsrc == nil || fdst == nil {
			log.Errorf("Source or destination is null")
			return nil
		}
		dir := sync.CopyDir(ctx, fdst, fsrc, true)
		return dir
	})

	restartAttrs := make(map[string]interface{})
	restartAttrs["type"] = "success"
	restartAttrs["message"] = "Cloud storage file upload queued"
	restartAttrs["title"] = "Success"
	actionResponse := NewActionResponse("client.notify", restartAttrs)
	responses = append(responses, actionResponse)

	return nil, responses, nil
}

func NewSyncSiteStorageActionPerformer(cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := SyncSiteStorageActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
