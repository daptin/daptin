package actions

import (
	"context"
	"errors"
	"fmt"
	"github.com/artpar/api2go/v2"
	"github.com/artpar/rclone/cmd"
	"github.com/artpar/rclone/fs/config"
	"github.com/artpar/rclone/fs/sync"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"strings"
)

type syncColumnStorageActionPerformer struct {
	cruds map[string]*resource.DbResource
}

func (d *syncColumnStorageActionPerformer) Name() string {
	return "column.storage.sync"
}

func (d *syncColumnStorageActionPerformer) DoAction(request actionresponse.Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	responses := make([]actionresponse.ActionResponse, 0)

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

	credentialName, ok := inFields["credential_name"]
	configSetName := cloudStore.Name
	if strings.Index(cloudStore.RootPath, ":") > -1 {
		configSetName = strings.Split(cloudStore.RootPath, ":")[0]
	}
	if ok && credentialName != nil && credentialName != "" {
		cred, err := d.cruds["credential"].GetCredentialByName(credentialName.(string), transaction)
		resource.CheckErr(err, fmt.Sprintf("Failed to get credential for [%s]", credentialName))
		if cred.DataMap != nil {
			for key, val := range cred.DataMap {
				config.Data().SetValue(configSetName, key, fmt.Sprintf("%s", val))
			}
		}
	}

	args := []string{
		cloudStore.RootPath,
		cacheFolder.LocalSyncPath,
	}

	if cacheFolder.Keyname != "" {
		args[0] = args[0] + "/" + cacheFolder.Keyname
	}

	fsrc, fdst := cmd.NewFsSrcDst(args)
	log.Infof("[73] Temp dir for column storage sync [%v]/%v ==> %v", cloudStore.Name, args[0], cacheFolder.LocalSyncPath)
	cobraCommand := &cobra.Command{
		Use: fmt.Sprintf("Sync column storage [%v]", columnName),
	}
	ctx := context.Background()
	go cmd.Run(true, true, cobraCommand, func() error {
		if fsrc == nil || fdst == nil {
			log.Errorf("[74] Either source [%s] or destination[%s] is empty", cloudStore.Name+"/"+cacheFolder.Keyname, cacheFolder.LocalSyncPath)
			return nil
		}

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
	restartAttrs["message"] = "Cloud storage sync queued"
	restartAttrs["title"] = "Success"
	actionResponse := resource.NewActionResponse("client.notify", restartAttrs)
	responses = append(responses, actionResponse)

	return nil, responses, nil
}

func NewSyncColumnStorageActionPerformer(cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {

	handler := syncColumnStorageActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
