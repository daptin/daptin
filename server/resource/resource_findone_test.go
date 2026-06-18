package resource

import (
	"testing"

	"github.com/artpar/api2go/v2"
)

func TestAppendFindOneIncludeSkipsCloudStoreFileIncludeTypes(t *testing.T) {
	dbResource := &DbResource{Cruds: map[string]*DbResource{}}
	model := api2go.NewApi2GoModelWithData("mail", nil, 0, nil, map[string]interface{}{})

	dbResource.appendFindOneInclude(&model, map[string]interface{}{
		"__type":   "gzip",
		"name":     "message.eml",
		"contents": "U3ViamVjdDogdGVzdA==",
	})

	if len(model.Includes) != 0 {
		t.Fatalf("expected non-resource file include to be skipped, got %d includes", len(model.Includes))
	}
}
