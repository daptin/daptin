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
	return "cloudstore.file.delete"
}

func (d *cloudStoreFileDeleteActionPerformer) DoAction(request actionresponse.Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	responses := make([]actionresponse.ActionResponse, 0)
	var err error

	atPath, ok := inFields["path"].(string)
	if !ok {
		return nil, nil, []error{errors.New("path is missing")}
	}

	rootPath := inFields["root_path"].(string)
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

	credentialName, ok := inFields["credential_name"]
	if ok && credentialName != nil && credentialName != "" {
		cred, err := d.cruds["credential"].GetCredentialByName(credentialName.(string), transaction)
		resource.CheckErr(err, fmt.Sprintf("Failed to get credential for [%s]", credentialName))
		name := strings.Split(rootPath, ":")[0]
		if cred.DataMap != nil {
			for key, val := range cred.DataMap {
				config.Data().SetValue(name, key, fmt.Sprintf("%s", val))
			}
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
