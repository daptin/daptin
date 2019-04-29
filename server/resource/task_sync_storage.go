package resource

import (
	"encoding/json"
	"github.com/artpar/rclone/cmd"
	"github.com/artpar/rclone/fs/config"
	"github.com/artpar/rclone/fs/sync"
	"github.com/artpar/rclone/lib/pacer"
	log "github.com/sirupsen/logrus"
	"strings"
)

func (res *DbResource) SyncStorageToPath(cloudStore CloudStore, tempDirectoryPath string) error {

	oauthTokenId := cloudStore.OAutoTokenId

	token, oauthConf, err := res.GetTokenByTokenReferenceId(oauthTokenId)
	CheckErr(err, "Failed to get oauth2 token for scheduled storage sync")

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
	pacer := pacer.Pacer{}
	pacer.SetRetries(5)
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
		dir := sync.CopyDir(fdst, fsrc, true)
		return dir
	})

	return nil
}
