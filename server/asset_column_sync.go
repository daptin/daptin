package server

import (
	"github.com/daptin/daptin/server/assetcachepojo"
	"github.com/daptin/daptin/server/cloud_store"
	"github.com/daptin/daptin/server/dbresourceinterface"
	"github.com/daptin/daptin/server/rootpojo"
	"github.com/daptin/daptin/server/task"
	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
)

func CreateAssetColumnSync(cruds map[string]dbresourceinterface.DbResourceInterface, transaction *sqlx.Tx) map[string]map[string]*assetcachepojo.AssetFolderCache {
	logrus.Tracef("CreateAssetColumnSync")

	stores, err := cloud_store.GetAllCloudStores(cruds["cloud_store"], transaction)
	assetCache := make(map[string]map[string]*assetcachepojo.AssetFolderCache)

	if err != nil || len(stores) == 0 {
		return assetCache
	}
	cloudStoreMap := make(map[string]rootpojo.CloudStore)

	for _, store := range stores {
		cloudStoreMap[store.Name] = store
	}

	for tableName, tableCrud := range cruds {

		colCache := make(map[string]*assetcachepojo.AssetFolderCache)

		tableInfo := tableCrud.TableInfo()
		for _, column := range tableInfo.Columns {

			if column.IsForeignKey && column.ForeignKeyData.DataSource == "cloud_store" {

				columnName := column.ColumnName

				cloudStore := cloudStoreMap[column.ForeignKeyData.Namespace]
				tempDirectoryPath, err := ioutil.TempDir(os.Getenv("DAPTIN_CACHE_FOLDER"), tableName+"_"+columnName)

				if cloudStore.StoreProvider != "local" {
					err = cruds["task"].SyncStorageToPath(cloudStore, column.ForeignKeyData.KeyName, tempDirectoryPath, transaction)
					if CheckErr(err, "Failed to setup sync to path for table column [%v][%v]", tableName, column.ColumnName) {
						continue
					}
				} else {
					tempDirectoryPath = cloudStore.RootPath + "/" + column.ForeignKeyData.KeyName
				}

				assetCacheFolder := &assetcachepojo.AssetFolderCache{
					CloudStore:    cloudStore,
					LocalSyncPath: tempDirectoryPath,
					Keyname:       column.ForeignKeyData.KeyName,
				}

				colCache[columnName] = assetCacheFolder
				logrus.Infof("Sync table column [%v][%v] at %v", tableName, columnName, tempDirectoryPath)

				if cloudStore.StoreProvider != "local" {
					err = TaskScheduler.AddTask(task.Task{
						EntityName: "world",
						ActionName: "sync_column_storage",
						Attributes: map[string]interface{}{
							"table_name":  tableInfo.TableName,
							"column_name": columnName,
						},
						AsUserEmail: cruds["user_account"].GetAdminEmailId(transaction),
						Schedule:    "@every 30m",
					})
				}

			}

		}

		assetCache[tableName] = colCache

	}
	logrus.Tracef("Completed CreateAssetColumnSync")

	return assetCache

}
