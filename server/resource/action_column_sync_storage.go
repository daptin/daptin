package resource

import (
	"context"
	"errors"
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

type SyncColumnStorageActionPerformer struct {
	cruds map[string]*DbResource
}

func (d *SyncColumnStorageActionPerformer) Name() string {
	return "column.storage.sync"
}

func (d *SyncColumnStorageActionPerformer) DoAction(request Outcome, inFields map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	responses := make([]ActionResponse, 0)

	columnName, ok := inFields["column_name"].(string)
	if !ok {
		return nil, nil, []error{errors.New("missing column name")}

	}
	tableName, ok := inFields["table_name"].(string)
	if !ok {
		return nil, nil, []error{errors.New("missing table name")}
	}

	cacheFolder, ok := d.cruds["world"].AssetFolderCache[tableName][columnName]
	if !ok {
		return nil, nil, []error{errors.New("not a synced folder")}
	}
	cloudStore := cacheFolder.CloudStore

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
		cacheFolder.LocalSyncPath,
	}

	if cacheFolder.Keyname != "" {
		args[0] = args[0] + "/" + cacheFolder.Keyname
	}

	fsrc, fdst := cmd.NewFsSrcDst(args)
	log.Infof("Temp dir for site [%v]/%v ==> %v", cloudStore.Name, args[0], cacheFolder.LocalSyncPath)
	cobraCommand := &cobra.Command{
		Use: fmt.Sprintf("Sync column storage [%v]", columnName),
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

func NewSyncColumnStorageActionPerformer(cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := SyncColumnStorageActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
