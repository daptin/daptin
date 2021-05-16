package resource

import (
	"context"
	"errors"
	"fmt"
	"github.com/artpar/rclone/cmd"
	"github.com/artpar/rclone/fs"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"

	"github.com/artpar/api2go"
	"github.com/artpar/rclone/fs/config"
	"github.com/artpar/rclone/fs/sync"
	"strings"
)

type syncColumnStorageActionPerformer struct {
	cruds map[string]*DbResource
}

func (d *syncColumnStorageActionPerformer) Name() string {
	return "column.storage.sync"
}

func (d *syncColumnStorageActionPerformer) DoAction(request Outcome, inFields map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

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

	var token *oauth2.Token
	var oauthConf *oauth2.Config
	var err error

	if cloudStore.StoreProvider != "local" {
		token, oauthConf, err = d.cruds["oauth_token"].GetTokenByTokenReferenceId(oauthTokenId)
		CheckErr(err, "Failed to get oauth2 token for storage sync")
	}

	jsonToken, err := json.Marshal(token)
	CheckErr(err, "Failed to convert token to json")
	if jsonToken != nil {
		config.FileSet(cloudStore.StoreProvider, "token", string(jsonToken))
	}
	if oauthConf != nil {
		config.FileSet(cloudStore.StoreProvider, "client_id", oauthConf.ClientID)
		config.FileSet(cloudStore.StoreProvider, "type", cloudStore.StoreProvider)
		config.FileSet(cloudStore.StoreProvider, "client_secret", oauthConf.ClientSecret)
		config.FileSet(cloudStore.StoreProvider, "client_scopes", strings.Join(oauthConf.Scopes, ","))
		config.FileSet(cloudStore.StoreProvider, "redirect_url", oauthConf.RedirectURL)
	}

	args := []string{
		cloudStore.RootPath,
		cacheFolder.LocalSyncPath,
	}

	if cacheFolder.Keyname != "" {
		args[0] = args[0] + "/" + cacheFolder.Keyname
	}

	fsrc, fdst := cmd.NewFsSrcDst(args)
	log.Printf("Temp dir for site [%v]/%v ==> %v", cloudStore.Name, args[0], cacheFolder.LocalSyncPath)
	cobraCommand := &cobra.Command{
		Use: fmt.Sprintf("Sync column storage [%v]", columnName),
	}
	defaultConfig := fs.GetConfig(nil)
	defaultConfig.LogLevel = fs.LogLevelNotice
	go cmd.Run(true, false, cobraCommand, func() error {
		if fsrc == nil || fdst == nil {
			log.Errorf("Either source or destination is empty")
			return nil
		}

		ctx := context.Background()
		//log.Printf("Starting to copy drive for site base from [%v] to [%v]", fsrc.String(), fdst.String())
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

	handler := syncColumnStorageActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
