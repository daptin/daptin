package dbresourceinterface

import (
	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/assetcachepojo"
	"github.com/daptin/daptin/server/database"
	"github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/permission"
	"github.com/daptin/daptin/server/rootpojo"
	"github.com/daptin/daptin/server/table_info"
	"github.com/jmoiron/sqlx"
)

type DbResourceInterface interface {
	GetAllObjects(name string, transaction *sqlx.Tx) ([]map[string]interface{}, error)
	GetObjectPermissionByReferenceId(name string, ref daptinid.DaptinReferenceId, tx *sqlx.Tx) permission.PermissionInstance
	TableInfo() *table_info.TableInfo
	GetAdminEmailId(transaction *sqlx.Tx) string
	Connection() database.DatabaseConnection
	HandleActionRequest(request actionresponse.ActionRequest, data api2go.Request, transaction1 *sqlx.Tx) ([]actionresponse.ActionResponse, error)
	GetActionHandler(name string) actionresponse.ActionPerformerInterface
	GetCredentialByName(credentialName string, transaction *sqlx.Tx) (*Credential, error)
	SubsiteFolderCache(id daptinid.DaptinReferenceId) (*assetcachepojo.AssetFolderCache, bool)
	SyncStorageToPath(store rootpojo.CloudStore, name string, path string, transaction *sqlx.Tx) error
}
