package resource

import (
	"strings"
	"golang.org/x/oauth2"
	log "github.com/sirupsen/logrus"
	"context"
	"encoding/json"
	"github.com/artpar/rclone/fs/config"
	"github.com/artpar/rclone/cmd"
	"github.com/artpar/rclone/fs/sync"
)

func (res *DbResource) SyncStorageToPath(cloudStore CloudStore, tempDirectoryPath string) error {

	oauthTokenId := cloudStore.OAutoTokenId

	token, err := res.GetTokenByTokenReferenceId(oauthTokenId)
	oauthConf := &oauth2.Config{}
	if err != nil {
		log.Infof("Failed to get oauth token for store sync: %v", err)
	} else {
		oauthConf, err := res.GetOauthDescriptionByTokenReferenceId(oauthTokenId)
		if !token.Valid() {
			ctx := context.Background()
			tokenSource := oauthConf.TokenSource(ctx, token)
			token, err = tokenSource.Token()
			CheckErr(err, "Failed to get new access token")
			if token == nil {
				log.Errorf("we have no token to get the site from storage: %v", cloudStore.ReferenceId)
			} else {
				err = res.UpdateAccessTokenByTokenReferenceId(oauthTokenId, token.AccessToken, token.Expiry.Unix())
				CheckErr(err, "failed to update access token")
			}
		}
	}

	//hostRouter := httprouter.New()

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
	go cmd.Run(true, true, nil, func() error {
		if fsrc == nil || fdst == nil {
			log.Errorf("Either source or destination is empty")
			return nil
		}
		log.Infof("Starting to copy drive for site base from [%v] to [%v]", fsrc.String(), fdst.String())
		if fsrc == nil || fdst == nil {
			log.Errorf("Source or destination is null")
			return nil
		}
		dir := sync.CopyDir(fdst, fsrc)
		return dir
	})

	return nil
}
