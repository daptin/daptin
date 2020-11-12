package resource

import (
	"context"
	"github.com/artpar/api2go"
	"github.com/json-iterator/go"
	"github.com/pkg/errors"
	"net/http"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type deleteWorldColumnPerformer struct {
	cmsConfig *CmsConfig
	cruds     map[string]*DbResource
}

func (d *deleteWorldColumnPerformer) Name() string {
	return "world.column.delete"
}

func (d *deleteWorldColumnPerformer) DoAction(request Outcome, inFields map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	worldName := inFields["world_name"].(string)
	columnToDelete := inFields["column_name"].(string)

	sessionUser := request.Attributes["user"]
	httpReq := &http.Request{
		Method: "GET",
	}

	httpReq = httpReq.WithContext(context.WithValue(context.Background(), "user", sessionUser))
	req := &api2go.Request{
		PlainRequest: httpReq,
	}

	table, err := d.cruds["world"].GetObjectByWhereClause("world", "table_name", worldName)
	if err != nil {
		return nil, nil, []error{err}
	}

	tableData := table

	schemaJson := tableData["world_schema_json"]

	var tableSchema TableInfo
	err = json.Unmarshal([]byte(schemaJson.(string)), &tableSchema)
	if err != nil {
		return nil, nil, []error{err}
	}

	indexToDelete := -1
	newColumns := make([]api2go.ColumnInfo, 0)
	for i, col := range tableSchema.Columns {
		if col.Name == columnToDelete {
			indexToDelete = i
			continue
		}
		newColumns = append(newColumns, col)
	}

	if indexToDelete == -1 {
		return nil, nil, []error{errors.New("no such column")}
	}
	tableSchema.Columns = newColumns

	schemaJson, err = json.Marshal(tableSchema)

	_, err = d.cruds["world"].db.Exec("alter table " + tableSchema.TableName + " drop column " + columnToDelete)
	if err != nil {
		return nil, nil, []error{err}
	}

	tableData["world_schema_json"] = schemaJson
	delete(tableData, "version")

	_, err = d.cruds["world"].UpdateWithoutFilters(tableData, *req)
	if err != nil {
		return nil, nil, []error{err}
	}

	restart()

	return nil, []ActionResponse{NewActionResponse("client.notify", NewClientNotification("message", "Column deleted", "Success"))}, nil
}

func NewDeleteWorldColumnPerformer(initConfig *CmsConfig, cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := deleteWorldColumnPerformer{
		cruds:     cruds,
		cmsConfig: initConfig,
	}

	return &handler, nil

}
