package resource

import (
	"context"
	"errors"
	"fmt"
	"github.com/artpar/rclone/cmd"
	"github.com/artpar/rclone/fs"
	"github.com/artpar/rclone/fs/operations"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"

	"github.com/artpar/api2go"
	"github.com/artpar/rclone/fs/config"
	"github.com/artpar/rclone/fs/sync"
	hugoCommand "github.com/gohugoio/hugo/commands"
	"strings"
)

type syncSiteStorageActionPerformer struct {
	cruds map[string]*DbResource
}

func (d *syncSiteStorageActionPerformer) Name() string {
	return "site.storage.sync"
}

func (d *syncSiteStorageActionPerformer) DoAction(request Outcome, inFields map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	responses := make([]ActionResponse, 0)

	cloudStoreId := inFields["cloud_store_id"].(string)
	siteId := inFields["site_id"].(string)
	path := inFields["path"].(string)
	cloudStore, err := d.cruds["cloud_store"].GetCloudStoreByReferenceId(cloudStoreId)
	if err != nil {
		return nil, nil, []error{err}
	}

	oauthTokenId := cloudStore.OAutoTokenId
	siteCacheFolder := d.cruds["cloud_store"].SubsiteFolderCache[siteId]
	if siteCacheFolder == nil {
		log.Infof("No sub-site cache found on local")
		return nil, nil, []error{errors.New("no site found here")}
	}

	var token *oauth2.Token
	var oauthConf *oauth2.Config

	if cloudStore.StoreProvider != "local" {
		token, oauthConf, err = d.cruds["oauth_token"].GetTokenByTokenReferenceId(oauthTokenId)
		//CheckErr(err, "Failed to get oauth2 token for storage sync")
	}

	jsonToken, err := json.Marshal(token)
	CheckErr(err, "Failed to convert token to json")
	if oauthConf != nil {
		config.FileSet(cloudStore.StoreProvider, "client_id", oauthConf.ClientID)
		config.FileSet(cloudStore.StoreProvider, "type", cloudStore.StoreProvider)
		config.FileSet(cloudStore.StoreProvider, "client_secret", oauthConf.ClientSecret)
		config.FileSet(cloudStore.StoreProvider, "token", string(jsonToken))
		config.FileSet(cloudStore.StoreProvider, "client_scopes", strings.Join(oauthConf.Scopes, ","))
		config.FileSet(cloudStore.StoreProvider, "redirect_url", oauthConf.RedirectURL)
	}

	tempDirectoryPath := path
	if tempDirectoryPath == "" {
		tempDirectoryPath = siteCacheFolder.LocalSyncPath
	}

	daptinSite, _, err := d.cruds["site"].GetSingleRowByReferenceId("site", siteId, nil)
	if err != nil {
		return nil, nil, []error{err}
	}
	is_hugo_site := daptinSite["site_type"] == "hugo"

	path = siteCacheFolder.Keyname
	if !EndsWithCheck(cloudStore.RootPath, "/") && !BeginsWith(path, "/") {
		path = "/" + path
	}
	args := []string{
		cloudStore.RootPath + path,
		tempDirectoryPath,
	}

	fsrc, srcFileName, fdst := cmd.NewFsSrcFileDst(args)
	log.Infof("Temp dir for site [%v]/%v ==> %v", cloudStore.Name, cloudStore.RootPath, tempDirectoryPath)
	cobraCommand := &cobra.Command{
		Use: fmt.Sprintf("Sync site storage [%v]", cloudStoreId),
	}
	defaultConfig := fs.GetConfig(nil)
	defaultConfig.LogLevel = fs.LogLevelNotice

	go cmd.Run(true, false, cobraCommand, func() error {
		if fsrc == nil || fdst == nil {
			log.Errorf("Either source or destination is empty")
			return nil
		}

		ctx := context.Background()
		//log.Infof("Starting to copy drive for site base from [%v] to [%v]", fsrc.String(), fdst.String())
		if fsrc == nil || fdst == nil {
			log.Errorf("Source or destination is null")
			return nil
		}

		defaultConfig := fs.GetConfig(nil)
		defaultConfig.LogLevel = fs.LogLevelNotice
		defaultConfig.DeleteMode = fs.DeleteModeBefore
		defaultConfig.AutoConfirm = true

		if srcFileName == "" {
			err = sync.Sync(ctx, fdst, fsrc, true)
		} else {
			err = operations.CopyFile(ctx, fdst, fsrc, srcFileName, srcFileName)
		}

		if is_hugo_site && err == nil {
			log.Infof("Starting hugo build for %v", tempDirectoryPath)
			hugoCommandResponse := hugoCommand.Execute([]string{"--source", tempDirectoryPath, "--destination", tempDirectoryPath + "/" + "public", "--verbose", "--verboseLog"})
			log.Infof("Hugo command response for [%v] [%v]: %v", tempDirectoryPath, tempDirectoryPath+"/"+"public", hugoCommandResponse)
		}

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

func NewSyncSiteStorageActionPerformer(cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := syncSiteStorageActionPerformer{
		cruds: cruds,
	}

	return &handler, nil

}
