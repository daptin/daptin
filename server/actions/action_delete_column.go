package actions

import (
	"context"
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/table_info"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"net/http"
	"net/url"
)

type deleteWorldColumnPerformer struct {
	cmsConfig *resource.CmsConfig
	cruds     map[string]*resource.DbResource
}

func (d *deleteWorldColumnPerformer) Name() string {
	return "world.column.delete"
}

func (d *deleteWorldColumnPerformer) DoAction(request actionresponse.Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {

	worldName := inFields["world_name"].(string)
	columnToDelete := inFields["column_name"].(string)

	sessionUser := request.Attributes["user"]

	table, err := d.cruds["world"].GetObjectByWhereClauseWithTransaction("world", "table_name", worldName, transaction)
	if err != nil {
		return nil, nil, []error{err}
	}

	tableData := table

	schemaJson := tableData["world_schema_json"]

	var tableSchema table_info.TableInfo
	err = json.Unmarshal([]byte(schemaJson.(string)), &tableSchema)
	if err != nil {
		return nil, nil, []error{err}
	}

	ur, _ := url.Parse("/world")

	httpReq := &http.Request{
		Method: "GET",
		URL:    ur,
	}

	httpReq = httpReq.WithContext(context.WithValue(context.Background(), "user", sessionUser))
	req := &api2go.Request{
		PlainRequest: httpReq,
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

	_, err = transaction.Exec("alter table " + tableSchema.TableName + " drop column " + columnToDelete)
	if err != nil {
		return nil, nil, []error{err}
	}

	updateObj := api2go.NewApi2GoModelWithData(tableSchema.TableName, nil, 0, nil, tableData)
	updateObj.SetAttributes(map[string]interface{}{
		"world_schema_json": schemaJson,
	})

	_, err = d.cruds["world"].UpdateWithoutFilters(updateObj, *req, transaction)

	if err != nil {
		return nil, nil, []error{err}
	}

	//Restart()

	return nil, []actionresponse.ActionResponse{resource.NewActionResponse("client.notify", resource.NewClientNotification("message", "Column deleted", "Success"))}, nil
}

func NewDeleteWorldColumnPerformer(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource) (actionresponse.ActionPerformerInterface, error) {

	handler := deleteWorldColumnPerformer{
		cruds:     cruds,
		cmsConfig: initConfig,
	}

	return &handler, nil

}
