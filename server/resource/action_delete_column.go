package resource

import (
	"context"
	"github.com/artpar/api2go"
	"github.com/json-iterator/go"
	"github.com/pkg/errors"
	"net/http"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type DeleteWorldColumnPerformer struct {
	cmsConfig *CmsConfig
	cruds     map[string]*DbResource
}

func (d *DeleteWorldColumnPerformer) Name() string {
	return "world.column.delete"
}

func (d *DeleteWorldColumnPerformer) DoAction(request Outcome, inFields map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	worldId := inFields["world_id"].(string)
	columnToDelete := inFields["column_name"].(string)

	sessionUser := request.Attributes["user"]
	httpReq := &http.Request{
		Method: "GET",
	}

	httpReq = httpReq.WithContext(context.WithValue(context.Background(), "user", sessionUser))
	req := &api2go.Request{
		PlainRequest: httpReq,
	}

	table, err := d.cruds["world"].FindOne(worldId, *req)
	if err != nil {
		return nil, nil, []error{err}
	}

	res := table.Result()
	tableData, ok := res.(*api2go.Api2GoModel)
	if !ok {
		return nil, nil, []error{errors.New("failed to find the table")}
	}

	schemaJson := tableData.Data["world_schema_json"]

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

	tableData.Data["world_schema_json"] = schemaJson
	delete(tableData.Data, "version")

	_, err = d.cruds["world"].UpdateWithoutFilters(tableData, *req)
	if err != nil {
		return nil, nil, []error{err}
	}

	restart()

	return nil, []ActionResponse{NewActionResponse("client.notify", NewClientNotification("message", "Column deleted", "Success"))}, nil
}

func NewDeleteWorldColumnPerformer(initConfig *CmsConfig, cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := DeleteWorldColumnPerformer{
		cruds:     cruds,
		cmsConfig: initConfig,
	}

	return &handler, nil

}
