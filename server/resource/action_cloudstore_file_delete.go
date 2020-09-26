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
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	"strings"
)

type cloudStoreFileDeleteActionPerformer struct {
	cruds map[string]*DbResource
}

func (d *cloudStoreFileDeleteActionPerformer) Name() string {
	return "cloudstore.file.delete"
}

func (d *cloudStoreFileDeleteActionPerformer) DoAction(request Outcome, inFields map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	responses := make([]ActionResponse, 0)
	var err error

	CheckErr(err, "Failed to create temp tempDirectoryPath for rclone upload")
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
	log.Infof("Delete target path: %v", rootPath)

	var token *oauth2.Token
	oauthConf := &oauth2.Config{}
	oauthTokenId1 := inFields["oauth_token_id"]
	if oauthTokenId1 == nil {
		log.Infof("No oauth token set for target store")
	} else {
		oauthTokenId := oauthTokenId1.(string)
		token, oauthConf, err = d.cruds["oauth_token"].GetTokenByTokenReferenceId(oauthTokenId)
		CheckErr(err, "Failed to get oauth2 token for store sync")
	}

	jsonToken, err := json.Marshal(token)
	CheckErr(err, "Failed to marshal access token to json")

	storeProvider := inFields["store_provider"].(string)
	config.FileSet(storeProvider, "client_id", oauthConf.ClientID)
	config.FileSet(storeProvider, "type", storeProvider)
	config.FileSet(storeProvider, "client_secret", oauthConf.ClientSecret)
	config.FileSet(storeProvider, "token", string(jsonToken))
	config.FileSet(storeProvider, "client_scopes", strings.Join(oauthConf.Scopes, ","))
	config.FileSet(storeProvider, "redirect_url", oauthConf.RedirectURL)

	fsrc := cmd.NewFsSrc(args)
	cobraCommand := &cobra.Command{
		Use: fmt.Sprintf("Delete file action at [%v]", atPath),
	}
	fs.Config.LogLevel = fs.LogLevelNotice

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
