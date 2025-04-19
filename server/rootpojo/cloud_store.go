package rootpojo

import (
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/permission"
	"time"
)

type CloudStore struct {
	Id              int64
	RootPath        string
	StoreParameters map[string]interface{}
	UserId          daptinid.DaptinReferenceId
	CredentialName  string
	Name            string
	StoreType       string
	StoreProvider   string
	Version         int
	CreatedAt       *time.Time
	UpdatedAt       *time.Time
	DeletedAt       *time.Time
	ReferenceId     daptinid.DaptinReferenceId
	Permission      permission.PermissionInstance
}
