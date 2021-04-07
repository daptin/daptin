package resource

import (
	"context"
	"fmt"
	"github.com/artpar/go.uuid"
	"github.com/artpar/rclone/cmd"
	"github.com/artpar/rclone/fs"
	"github.com/artpar/rclone/fs/operations"
	"github.com/artpar/rclone/fs/sync"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"io/ioutil"

	"github.com/artpar/api2go"
	"github.com/artpar/rclone/fs/config"
	"golang.org/x/oauth2"
	"os"
	"strings"
)

type cloudStorePathMoveActionPerformer struct {
	cruds map[string]*DbResource
}

func (d *cloudStorePathMoveActionPerformer) Name() string {
	return "cloudstore.path.move"
}

func (d *cloudStorePathMoveActionPerformer) DoAction(request Outcome, inFields map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	responses := make([]ActionResponse, 0)

	u, _ := uuid.NewV4()
	sourceDirectoryName := "upload-" + u.String()[0:8]
	tempDirectoryPath, err := ioutil.TempDir(os.Getenv("DAPTIN_CACHE_FOLDER"), sourceDirectoryName)
	log.Infof("Temp directory for this upload cloudStorePathMoveActionPerformer: %v", tempDirectoryPath)

	//defer os.RemoveAll(tempDirectoryPath) // clean up

	CheckErr(err, "Failed to create temp tempDirectoryPath for rclone upload")
	sourcePath, _ := inFields["source"].(string)
	destinationPath, _ := inFields["destination"].(string)
	rootPath := inFields["root_path"].(string)

	if len(sourcePath) > 0 && sourcePath[0] != '/' {
		sourcePath = "/" + sourcePath
	}

	if len(destinationPath) > 0 && destinationPath[0] != '/' {
		destinationPath = "/" + destinationPath
	}

	args := []string{
		rootPath + sourcePath,
		rootPath + destinationPath,
	}
	log.Infof("Create move %v %v", sourcePath, destinationPath)

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
		Use: fmt.Sprintf("File upload action from [%v]", tempDirectoryPath),
	}
	defaultConfig := fs.GetConfig(nil)
	defaultConfig.LogLevel = fs.LogLevelNotice

	fsrc, srcFileName, fdst := cmd.NewFsSrcFileDst(args)
	cmd.Run(true, true, cobraCommand, func() error {
		var err error
		if srcFileName == "" {
			err = sync.MoveDir(context.Background(), fdst, fsrc, false, true)
		} else {
			err = operations.MoveFile(context.Background(), fdst, fsrc, srcFileName, srcFileName)
		}

		if err != nil {
			InfoErr(err, "Failed to sync files for upload to cloud")
			err = os.RemoveAll(tempDirectoryPath)
			InfoErr(err, "Failed to remove temp directory after path move")
			return nil
		}
		return err
	})

	restartAttrs := make(map[string]interface{})
	restartAttrs["type"] = "success"
	restartAttrs["message"] = "Cloud storage path moved"
	restartAttrs["title"] = "Success"
	actionResponse := NewActionResponse("client.notify", restartAttrs)
	responses = append(responses, actionResponse)

	return nil, responses, nil
}

func NewCloudStorePathMoveActionPerformer(cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := cloudStorePathMoveActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
