package resource

import (
	"github.com/artpar/api2go"
	"github.com/pkg/errors"
	"net/http"
	"strings"
)

type RenameWorldColumnPerformer struct {
	cmsConfig *CmsConfig
	cruds     map[string]*DbResource
}

func (d *RenameWorldColumnPerformer) Name() string {
	return "world.column.rename"
}

func (d *RenameWorldColumnPerformer) DoAction(request Outcome, inFields map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	worldId := inFields["world_id"].(string)
	columnToRename := inFields["column_name"].(string)
	columnToNew := inFields["new_column_name"].(string)

	columnToNew = strings.ReplaceAll(columnToNew, " ", "_")

	if columnToRename == columnToNew {
		return nil, []ActionResponse{}, nil
	}
	if IsReservedWord(columnToNew) {
		return nil, []ActionResponse{}, []error{errors.New(columnToNew + " is a reserved word")}
	}

	req := api2go.Request{
		PlainRequest: &http.Request{
			Method: "GET",
		},
	}
	table, err := d.cruds["world"].FindOne(worldId, req)
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
	json.Unmarshal([]byte(schemaJson.(string)), &tableSchema)

	indexToRename := -1
	newColumns := make([]api2go.ColumnInfo, 0)
	for i, col := range tableSchema.Columns {
		if col.Name == columnToRename {
			col.Name = columnToNew
			col.ColumnName = columnToNew
			indexToRename = i
			//continue
		}
		newColumns = append(newColumns, col)
	}

	if indexToRename == -1 {
		return nil, nil, []error{errors.New("no such column")}
	}

	tableSchema.Columns = newColumns

	schemaJson, err = json.Marshal(tableSchema)

	_, err = d.cruds["world"].db.Exec("alter table " + tableSchema.TableName + " rename column " + columnToRename + " to " + columnToNew)
	if err != nil {
		return nil, nil, []error{err}
	}

	tableData.Data["world_schema_json"] = schemaJson
	delete(tableData.Data, "version")

	_, err = d.cruds["world"].UpdateWithoutFilters(tableData, req)
	if err != nil {
		return nil, nil, []error{err}
	}

	return nil, []ActionResponse{NewActionResponse("client.notify", NewClientNotification("message", "Column renamed", "Success"))}, nil
}

func NewRenameWorldColumnPerformer(initConfig *CmsConfig, cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := RenameWorldColumnPerformer{
		cruds:     cruds,
		cmsConfig: initConfig,
	}

	return &handler, nil

}
