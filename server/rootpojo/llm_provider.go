package rootpojo

import (
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/permission"
	"time"
)

type LLMProvider struct {
	Id                 int64
	Name               string
	ProviderType       string
	BaseUrl            string
	Models             string
	CredentialName     string
	ProviderParameters map[string]interface{}
	Enable             bool
	Version            int
	CreatedAt          *time.Time
	UpdatedAt          *time.Time
	DeletedAt          *time.Time
	ReferenceId        daptinid.DaptinReferenceId
	Permission         permission.PermissionInstance
}
