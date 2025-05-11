package server

import (
	"github.com/daptin/daptin/server/assetcachepojo"
	"github.com/daptin/daptin/server/cloud_store"
	"github.com/daptin/daptin/server/dbresourceinterface"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/rootpojo"
	"github.com/daptin/daptin/server/subsite"
	"github.com/fclairamb/ftpserver/server"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

func CreateFtpServers(resources map[string]*resource.DbResource, resourcesInterfaces map[string]dbresourceinterface.DbResourceInterface, certManager *resource.CertificateManager, ftp_interface string, transaction *sqlx.Tx) (*server.FtpServer, error) {

	subsites, err := subsite.GetAllSites(resourcesInterfaces["site"], transaction)
	if err != nil {
		return nil, err
	}
	cloudStores, err := cloud_store.GetAllCloudStores(resourcesInterfaces["cloud_store"], transaction)

	if err != nil {
		return nil, err
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

	driver, err = NewDaptinFtpDriver(resources, certManager, ftp_interface, sites)
	ftpS := server.NewFtpServer(driver)
	resource.CheckErr(err, "Failed to create daptin ftp driver [%v]", driver)
	return ftpS, err

}

type SubSiteAssetCache struct {
	subsite.SubSite
	*assetcachepojo.AssetFolderCache
}
