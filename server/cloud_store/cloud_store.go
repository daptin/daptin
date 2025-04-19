package cloud_store

import (
	"encoding/json"
	"github.com/daptin/daptin/server/dbresourceinterface"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/rootpojo"
	"github.com/jmoiron/sqlx"
	"strconv"
	"time"
)

func StringOrEmpty(i interface{}) string {
	s, ok := i.(string)
	if ok {
		return s
	}
	return ""
}

func GetAllCloudStores(dbResource dbresourceinterface.DbResourceInterface, transaction *sqlx.Tx) ([]rootpojo.CloudStore, error) {
	var cloudStores []rootpojo.CloudStore

	rows, err := dbResource.GetAllObjects("cloud_store", transaction)
	if err != nil {
		return cloudStores, err
	}

	for _, storeRowMap := range rows {
		var cloudStore rootpojo.CloudStore

		cloudStore.CredentialName = StringOrEmpty(storeRowMap["credential_name"])
		cloudStore.Name = storeRowMap["name"].(string)

		id, ok := storeRowMap["id"].(int64)
		if !ok {
			id, err = strconv.ParseInt(storeRowMap["id"].(string), 10, 64)
			CheckErr(err, "Failed to parse id as int in loading stores")
		}

		cloudStore.Id = id
		cloudStore.ReferenceId = daptinid.InterfaceToDIR(storeRowMap["reference_id"])
		if cloudStore.ReferenceId == daptinid.NullReferenceId {
			CheckErr(err, "Failed to parse permission as int in loading stores")
		}
		cloudStore.Permission = dbResource.GetObjectPermissionByReferenceId("cloud_store", cloudStore.ReferenceId, transaction)

		if storeRowMap["user_account_id"] != nil {
			cloudStore.UserId = daptinid.InterfaceToDIR(storeRowMap["user_account_id"])
		}

		createdAt, ok := storeRowMap["created_at"].(time.Time)
		if !ok {
			createdAt, _ = time.Parse(storeRowMap["created_at"].(string), "2006-01-02 15:04:05")
		}

		cloudStore.CreatedAt = &createdAt
		if storeRowMap["updated_at"] != nil {
			updatedAt, ok := storeRowMap["updated_at"].(time.Time)
			if !ok {
				updatedAt, _ = time.Parse(storeRowMap["updated_at"].(string), "2006-01-02 15:04:05")
			}
			cloudStore.UpdatedAt = &updatedAt
		}
		storeParameters := storeRowMap["store_parameters"].(string)

		storeParamMap := make(map[string]interface{})

		if len(storeParameters) > 0 {
			err = json.Unmarshal([]byte(storeParameters), &storeParamMap)
			CheckErr(err, "Failed to unmarshal store parameters for store %v", storeRowMap["name"])
		}

		cloudStore.StoreParameters = storeParamMap
		cloudStore.StoreProvider = storeRowMap["store_provider"].(string)
		cloudStore.StoreType = storeRowMap["store_type"].(string)
		cloudStore.RootPath = storeRowMap["root_path"].(string)

		version, ok := storeRowMap["version"].(int64)
		if !ok {
			version, _ = strconv.ParseInt(storeRowMap["version"].(string), 10, 64)
		}

		cloudStore.Version = int(version)

		cloudStores = append(cloudStores, cloudStore)
	}

	return cloudStores, nil

}
