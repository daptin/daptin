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
	"regexp"
	"strings"
)

var renameColumnIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type renameWorldColumnPerformer struct {
	cmsConfig *resource.CmsConfig
	cruds     map[string]*resource.DbResource
}

func (d *renameWorldColumnPerformer) Name() string {
	return "world.column.rename"
}

func (d *renameWorldColumnPerformer) DoAction(request actionresponse.Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	worldName, _ := inFields["world_name"].(string)
	columnToRename, _ := inFields["column_name"].(string)
	columnToNew, _ := inFields["new_column_name"].(string)

	columnToNew = strings.ReplaceAll(columnToNew, " ", "_")
	for _, name := range []string{worldName, columnToRename, columnToNew} {
		if !renameColumnIdentifier.MatchString(name) {
			return nil, nil, []error{errors.New("invalid column rename identifier")}
		}
	}

	if columnToRename == columnToNew {
		return nil, []actionresponse.ActionResponse{}, nil
	}
	if resource.IsReservedWord(strings.ToLower(columnToNew)) {
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
	schemaString, ok := schemaJson.(string)
	if !ok {
		return nil, nil, []error{errors.New("invalid table schema")}
	}

	var tableSchema table_info.TableInfo
	if err := json.Unmarshal([]byte(schemaString), &tableSchema); err != nil {
		return nil, nil, []error{err}
	}
	if !renameColumnIdentifier.MatchString(tableSchema.TableName) {
		return nil, nil, []error{errors.New("invalid table identifier")}
	}

	indexToRename := -1
	newColumns := make([]api2go.ColumnInfo, 0)
	for i, col := range tableSchema.Columns {
		if strings.EqualFold(col.ColumnName, columnToNew) || strings.EqualFold(col.Name, columnToNew) {
			return nil, nil, []error{errors.New("column already exists")}
		}
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
	if err != nil {
		return nil, nil, []error{err}
	}

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
