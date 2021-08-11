package resource

import (
	"github.com/artpar/api2go"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"net/http"
	"strings"
)

type renameWorldColumnPerformer struct {
	cmsConfig *CmsConfig
	cruds     map[string]*DbResource
}

func (d *renameWorldColumnPerformer) Name() string {
	return "world.column.rename"
}

func (d *renameWorldColumnPerformer) DoAction(request Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []ActionResponse, []error) {

	worldName := inFields["world_name"].(string)
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
	tableObj, err := d.cruds["world"].GetObjectByWhereClause("world", "table_name", worldName)
	if err != nil {
		return nil, nil, []error{err}
	}
	tableData := api2go.Api2GoModel{
		Data: tableObj,
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

	tableData.SetAttributes(map[string]interface{}{
		"world_schema_json": schemaJson,
	})

	_, err = d.cruds["world"].UpdateWithoutFilters(tableData, req, transaction)
	if err != nil {
		return nil, nil, []error{err}
	}

	return nil, []ActionResponse{NewActionResponse("client.notify", NewClientNotification("message", "Column renamed", "Success"))}, nil
}

func NewRenameWorldColumnPerformer(initConfig *CmsConfig, cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := renameWorldColumnPerformer{
		cruds:     cruds,
		cmsConfig: initConfig,
	}

	return &handler, nil

}
