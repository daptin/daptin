package server

import (
	"net"

	"github.com/daptin/daptin/server/assetcachepojo"
	"github.com/daptin/daptin/server/cloud_store"
	"github.com/daptin/daptin/server/dbresourceinterface"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/rootpojo"
	"github.com/daptin/daptin/server/subsite"
	ftpserver "github.com/fclairamb/ftpserver/server"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

func CreateFtpServers(resources map[string]*resource.DbResource, resourcesInterfaces map[string]dbresourceinterface.DbResourceInterface, certManager *resource.CertificateManager, ftpInterface string, transaction *sqlx.Tx) (*ftpserver.FtpServer, *ConnectionTracker, error) {

	subsites, err := subsite.GetAllSites(resourcesInterfaces["site"], transaction)
	if err != nil {
		return nil, nil, err
	}
	cloudStores, err := cloud_store.GetAllCloudStores(resourcesInterfaces["cloud_store"], transaction)

	if err != nil {
		return nil, nil, err
	}
	cloudStoreMap := make(map[uuid.UUID]rootpojo.CloudStore)
	for _, cloudStore := range cloudStores {
		re, _ := uuid.FromBytes(cloudStore.ReferenceId[:])
		cloudStoreMap[re] = cloudStore
	}
	var driver *DaptinFtpDriver

	sites := make([]SubSiteAssetCache, 0)
	for _, ftpServer := range subsites {

		if !ftpServer.FtpEnabled {
			continue
		}

		assetCacheFolder, ok := resourcesInterfaces["site"].SubsiteFolderCache(ftpServer.ReferenceId)
		if !ok {
			continue
		}
		site := SubSiteAssetCache{
			SubSite:          ftpServer,
			AssetFolderCache: assetCacheFolder,
		}
		sites = append(sites, site)

	}

	driver, err = NewDaptinFtpDriver(resources, certManager, ftpInterface, sites)
	resource.CheckErr(err, "Failed to create daptin ftp driver [%v]", driver)
	if err != nil {
		return nil, nil, err
	}
	listener, err := net.Listen("tcp", ftpInterface)
	if err != nil {
		return nil, nil, err
	}
	connections := NewConnectionTracker()
	driver.DaptinFtpServerSettings.Server.Listener = connections.Wrap(listener)
	ftpServer := ftpserver.NewFtpServer(driver)
	if err = ftpServer.Listen(); err != nil {
		_ = listener.Close()
		return nil, nil, err
	}
	return ftpServer, connections, nil

}

type SubSiteAssetCache struct {
	subsite.SubSite
	*assetcachepojo.AssetFolderCache
}
