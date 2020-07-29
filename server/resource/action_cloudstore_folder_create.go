package resource

import (
	"context"
	"fmt"
	"github.com/artpar/go.uuid"
	"github.com/artpar/rclone/cmd"
	"github.com/artpar/rclone/fs"
	"github.com/artpar/rclone/fs/operations"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"io/ioutil"

	"github.com/artpar/api2go"
	"github.com/artpar/rclone/fs/config"
	"golang.org/x/oauth2"
	"os"
	"strings"
)

type CloudStoreFolderCreateActionPerformer struct {
	cruds map[string]*DbResource
}

func (d *CloudStoreFolderCreateActionPerformer) Name() string {
	return "cloudstore.folder.create"
}

func (d *CloudStoreFolderCreateActionPerformer) DoAction(request Outcome, inFields map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	responses := make([]ActionResponse, 0)

	u, _ := uuid.NewV4()
	sourceDirectoryName := "upload-" + u.String()
	tempDirectoryPath, err := ioutil.TempDir(os.Getenv("DAPTIN_CACHE_FOLDER"), sourceDirectoryName)
	log.Infof("Temp directory for this upload: %v", tempDirectoryPath)

	//defer os.RemoveAll(tempDirectoryPath) // clean up

	CheckErr(err, "Failed to create temp tempDirectoryPath for rclone upload")
	atPath, _ := inFields["path"].(string)
	folderName, _ := inFields["name"].(string)
	rootPath := inFields["root_path"].(string)

	if len(atPath) > 0 && atPath[len(atPath)-1] != '/' {
		atPath = atPath + "/"
	}

	folderPath := atPath + folderName
	args := []string{
		rootPath,
	}
	log.Infof("Create foler target %v", folderPath)

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
	fs.Config.LogLevel = fs.LogLevelNotice

	go cmd.Run(true, false, cobraCommand, func() error {
		if fsrc == nil {
			log.Errorf("Source or destination is null")
			return nil
		}

		ctx := context.Background()

		err := operations.Mkdir(ctx, fsrc, folderPath)
		InfoErr(err, "Failed to sync files for upload to cloud")
		err = os.RemoveAll(tempDirectoryPath)
		InfoErr(err, "Failed to remove temp directory after upload")
		return err
	})

	restartAttrs := make(map[string]interface{})
	restartAttrs["type"] = "success"
	restartAttrs["message"] = "Cloud storage file upload queued"
	restartAttrs["title"] = "Success"
	actionResponse := NewActionResponse("client.notify", restartAttrs)
	responses = append(responses, actionResponse)

	return nil, responses, nil
}

func NewCloudStoreFolderCreateActionPerformer(cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := CloudStoreFolderCreateActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
