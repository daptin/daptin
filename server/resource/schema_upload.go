package resource

import (
	"encoding/base64"
	"errors"
	"strings"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
)

// Schema uploads install definitions within the owning action transaction.
type schemaUploadAction struct {
	install func(string, []byte, *sqlx.Tx) error
	world   *DbResource
}

func NewSchemaUploadAction(install func(string, []byte, *sqlx.Tx) error, world *DbResource) actionresponse.ActionPerformerInterface {
	return &schemaUploadAction{install: install, world: world}
}
func (*schemaUploadAction) Name() string { return "system_json_schema_update" }
func (p *schemaUploadAction) DoAction(_ actionresponse.Outcome, input map[string]interface{}, tx *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	files, ok := input["json_schema"].([]interface{})
	if !ok || len(files) == 0 {
		return nil, nil, []error{errors.New("schema files required")}
	}
	// Schema installation is a read/merge/write of shared definition records.
	// Serialize it on world's own metadata row using the resource row lock.
	if err := p.world.lockRowByWhereWithTransaction(tx, goqu.Ex{"table_name": "world"}); err != nil {
		return nil, nil, []error{err}
	}
	for _, raw := range files {
		file, ok := raw.(map[string]interface{})
		if !ok {
			return nil, nil, []error{errors.New("invalid schema file")}
		}
		name, _ := file["name"].(string)
		if name == "" {
			return nil, nil, []error{errors.New("schema filename required")}
		}
		contents, _ := file["file"].(string)
		if contents == "" {
			contents, _ = file["contents"].(string)
		}
		if strings.HasPrefix(contents, "data:") {
			_, contents, _ = strings.Cut(contents, ",")
		}
		body, err := base64.StdEncoding.DecodeString(contents)
		if err != nil {
			return nil, nil, []error{err}
		}
		if err := p.install(name, body, tx); err != nil {
			return nil, nil, []error{err}
		}
	}
	return nil, []actionresponse.ActionResponse{NewActionResponse("client.notify", map[string]interface{}{"type": "success", "title": "Schema uploaded", "message": "Definitions installed. Invoke restart_daptin to reload runtimes across the cluster."})}, nil
}
