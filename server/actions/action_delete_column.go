package actions

import (
	"context"
	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	daptinid "github.com/daptin/daptin/server/id"
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

	worldID := daptinid.InterfaceToDIR(inFields["world_id"])
	if worldID == daptinid.NullReferenceId {
		return nil, nil, []error{errors.New("world id is a null reference")}
	}
	columnToDelete, ok := inFields["column_name"].(string)
	if !ok || columnToDelete == "" {
		return nil, nil, []error{errors.New("column name is required")}
	}
	sourceRequest, ok := inFields["httpRequest"].(*http.Request)
	if !ok || sourceRequest == nil {
		return nil, nil, []error{errors.New("action request is missing")}
	}

	sessionUser := inFields["sessionUser"]

	table, err := d.cruds["world"].GetReferenceIdToObjectWithTransaction("world", worldID, transaction)
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

	httpReq = httpReq.WithContext(context.WithValue(sourceRequest.Context(), "user", sessionUser))
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
	if err != nil {
		return nil, nil, []error{err}
	}

	_, err = transaction.ExecContext(sourceRequest.Context(), "alter table "+tableSchema.TableName+" drop column "+columnToDelete)
	if err != nil {
		return nil, nil, []error{err}
	}

	updateObj := api2go.NewApi2GoModelWithData("world", nil, 0, nil, tableData)
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
