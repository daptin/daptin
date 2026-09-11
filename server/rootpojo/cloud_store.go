package rootpojo

import (
	"errors"
	storagefs "github.com/daptin/daptin/server/filesystem"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/permission"
	"time"
)

func (store CloudStore) ResolvePath(name string) (string, error) {
	if store.StoreType == "" {
		return "", errors.New("cloud store type is missing")
	}
	if store.StoreType == "local" {
		return storagefs.ResolveLocalPath(store.RootPath, name)
	}
	return storagefs.ResolvePath(store.RootPath, name)
}

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
