package actions

import (
	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/table_info"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"net/http"
	"net/url"
	"strings"
)

type renameWorldColumnPerformer struct {
	cmsConfig *resource.CmsConfig
	cruds     map[string]*resource.DbResource
}

func (d *renameWorldColumnPerformer) Name() string {
	return "world.column.rename"
}

func (d *renameWorldColumnPerformer) DoAction(request actionresponse.Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	worldName := inFields["world_name"].(string)
	columnToRename := inFields["column_name"].(string)
	columnToNew := inFields["new_column_name"].(string)

	columnToNew = strings.ReplaceAll(columnToNew, " ", "_")

	if columnToRename == columnToNew {
		return nil, []actionresponse.ActionResponse{}, nil
	}
	if resource.IsReservedWord(columnToNew) {
		return nil, []actionresponse.ActionResponse{}, []error{errors.New(columnToNew + " is a reserved word")}
	}
	ur, _ := url.Parse("/world")

	req := api2go.Request{
		PlainRequest: &http.Request{
			Method: "GET",
			URL:    ur,
		},
	}
	tableObj, err := d.cruds["world"].GetObjectByWhereClause("world", "table_name", worldName, transaction)
	if err != nil {
		return nil, nil, []error{err}
	}
	tableData := api2go.NewApi2GoModelWithData(
		worldName, nil, 0, nil, tableObj)
	schemaJson := tableData.GetAttributes()["world_schema_json"]

	var tableSchema table_info.TableInfo
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

	_, err = transaction.Exec("alter table " + tableSchema.TableName + " rename column " + columnToRename + " to " + columnToNew)
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

	return nil, []actionresponse.ActionResponse{resource.NewActionResponse("client.notify",
		resource.NewClientNotification("message", "Column renamed", "Success"))}, nil
}

func NewRenameWorldColumnPerformer(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {

	handler := renameWorldColumnPerformer{
		cruds:     cruds,
		cmsConfig: initConfig,
	}

	return &handler, nil

}
