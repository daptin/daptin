package resource

import (
	"context"
	"errors"
	"fmt"
	"github.com/artpar/api2go"
	"github.com/artpar/rclone/cmd"
	"github.com/artpar/rclone/fs"
	"github.com/artpar/rclone/fs/config"
	"github.com/artpar/rclone/fs/operations"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type cloudStoreFileDeleteActionPerformer struct {
	cruds map[string]*DbResource
}

func (d *cloudStoreFileDeleteActionPerformer) Name() string {
	return "cloudstore.file.delete"
}

func (d *cloudStoreFileDeleteActionPerformer) DoAction(request Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []ActionResponse, []error) {

	responses := make([]ActionResponse, 0)
	var err error

	atPath, ok := inFields["path"].(string)
	if !ok {
		return nil, nil, []error{errors.New("path is missing")}
	}

	rootPath := inFields["root_path"].(string)
	if atPath != "" {

		if !EndsWithCheck(rootPath, "/") && !BeginsWith(atPath, "/") {
			rootPath = rootPath + "/"
		}
		rootPath = rootPath + atPath
	}
	args := []string{
		rootPath,
	}
	log.Printf("Delete target path: %v", rootPath)

	credentialName, ok := inFields["credential_name"]
	if ok && credentialName != nil {
		cred, err := d.cruds["credential"].GetCredentialByName(credentialName.(string), transaction)
		CheckErr(err, fmt.Sprintf("Failed to get credential for [%s]", credentialName))
		name := inFields["name"].(string)
		if cred.DataMap != nil {
			for key, val := range cred.DataMap {
				config.Data().SetValue(name, key, fmt.Sprintf("%s", val))
			}
		}
	}

	//config.FileSet(name, "client_id", oauthConf.ClientID)
	//config.FileSet(name, "type", inFields["store_type"].(string))
	//config.FileSet(name, "provider", storeProvider)
	//config.FileSet(name, "client_secret", oauthConf.ClientSecret)
	//config.FileSet(name, "token", string(jsonToken))
	//config.FileSet(name, "client_scopes", strings.Join(oauthConf.Scopes, ","))
	//config.FileSet(name, "redirect_url", oauthConf.RedirectURL)

	fsrc := cmd.NewFsSrc(args)
	cobraCommand := &cobra.Command{
		Use: fmt.Sprintf("Delete file action at [%v]", atPath),
	}
	defaultConfig := fs.GetConfig(nil)
	defaultConfig.LogLevel = fs.LogLevelNotice

	go cmd.Run(true, false, cobraCommand, func() error {
		if fsrc == nil {
			log.Errorf("path is null for delete operation")
			return nil
		}

		ctx := context.Background()

		err = operations.Delete(ctx, fsrc)
		if err != nil {
			err = operations.Purge(ctx, fsrc, "")
		}

		InfoErr(err, "Failed to delete purge path [%v] in cloud store", rootPath)
		return err
	})

	restartAttrs := make(map[string]interface{})
	restartAttrs["type"] = "success"
	restartAttrs["message"] = "Cloud storage path deleted"
	restartAttrs["title"] = "Success"
	actionResponse := NewActionResponse("client.notify", restartAttrs)
	responses = append(responses, actionResponse)

	return nil, responses, nil
}

func NewCloudStoreFileDeleteActionPerformer(cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := cloudStoreFileDeleteActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
