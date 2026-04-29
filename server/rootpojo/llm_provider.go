package rootpojo

import (
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/permission"
	"time"
)

// ModelPricing holds per-model pricing in USD per million tokens.
type ModelPricing struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cache_read,omitempty"`
	CacheWrite float64 `json:"cache_write,omitempty"`
}

type LLMProvider struct {
	Id                 int64
	Name               string
	ProviderType       string
	BaseUrl            string
	Models             string
	CredentialName     string
	ProviderParameters map[string]interface{}
	ModelPricing       map[string]ModelPricing
	Enable             bool
	Version            int
	CreatedAt          *time.Time
	UpdatedAt          *time.Time
	DeletedAt          *time.Time
	ReferenceId        daptinid.DaptinReferenceId
	Permission         permission.PermissionInstance
}
