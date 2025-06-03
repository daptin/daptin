package resource

import (
	"context"
	"fmt"
	"github.com/artpar/rclone/cmd"
	"github.com/artpar/rclone/fs"
	"github.com/artpar/rclone/fs/config"
	"github.com/artpar/rclone/fs/sync"
	"github.com/artpar/rclone/lib/pacer"
	"github.com/daptin/daptin/server/rootpojo"
	"github.com/jmoiron/sqlx"
	"strings"

	//hugoCommand "github.com/gohugoio/hugo/commands"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func (dbResource *DbResource) SyncStorageToPath(cloudStore rootpojo.CloudStore, path string, tempDirectoryPath string, transaction *sqlx.Tx) error {

	configSetName := cloudStore.StoreProvider
	if strings.Index(cloudStore.RootPath, ":") > -1 {
		configSetName = strings.Split(cloudStore.RootPath, ":")[0]
	}
	if cloudStore.CredentialName != "" {
		cred, err := dbResource.GetCredentialByName(cloudStore.CredentialName, transaction)
		CheckErr(err, fmt.Sprintf("Failed to get credential for [%s]", cloudStore.CredentialName))
		if cred.DataMap != nil {
			for key, val := range cred.DataMap {
				config.Data().SetValue(configSetName, key, fmt.Sprintf("%s", val))
			}
		}
	}

	args := []string{
		cloudStore.RootPath,
		tempDirectoryPath,
	}

	if path != "" && path[0] != '/' && len(args[0]) > 0 && args[0][len(args[0])-1] != '/' {
		path = "/" + path
	}
	args[0] = args[0] + path

	fsrc, fdst := cmd.NewFsSrcDst(args)
	pacer1 := pacer.Pacer{}
	pacer1.SetRetries(3)
	log.Infof("Temp dir for path [%v]%v ==> %v", cloudStore.Name, cloudStore.RootPath, tempDirectoryPath)

	cobraCommand := &cobra.Command{
		Use: fmt.Sprintf("Sync cloud store [%v] to path [%v]", cloudStore.Name, tempDirectoryPath),
	}
	defaultConfig := fs.GetConfig(nil)
	defaultConfig.LogLevel = fs.LogLevelNotice

	go cmd.Run(true, false, cobraCommand, func() error {
		if fsrc == nil || fdst == nil {
			log.Errorf("[53] Either source or destination is empty")
			return nil
		}
		ctx := context.Background()
		log.Infof("Starting to copy drive for path base from [%v] to [%v]", fsrc.String(), fdst.String())
		if fsrc == nil || fdst == nil {
			log.Errorf("Source or destination is null")
			return nil
		}
		dir := sync.CopyDir(ctx, fdst, fsrc, true)

		return dir
	})

	return nil
}
