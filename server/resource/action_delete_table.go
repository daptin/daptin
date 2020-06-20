package resource

import (
	"context"
	"fmt"
	"github.com/Masterminds/squirrel"
	"github.com/artpar/api2go"
	"github.com/pkg/errors"
	"log"
	"net/http"
)

type DeleteWorldPerformer struct {
	cmsConfig *CmsConfig
	cruds     map[string]*DbResource
}

func (d *DeleteWorldPerformer) Name() string {
	return "world.delete"
}

func (d *DeleteWorldPerformer) DoAction(request Outcome, inFields map[string]interface{}) (api2go.Responder, []ActionResponse, []error) {

	worldId := inFields["world_id"].(string)

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
	relations := tableSchema.Relations

	var tablesToRemove []string
	errorsList := make([]error, 0)
	for _, relation := range relations {
		switch relation.Relation {

		case "belongs_to":
			if relation.Subject == tableSchema.TableName {
				// nothing to do
			} else {
				// we can delete just the index or the index and the referencing column as well
				_, err = d.cruds["world"].db.Exec("alter table " + relation.Subject + " drop column " + relation.ObjectName)
				if err != nil {
					errorsList = append(errorsList, err)
				}
			}
		case "has_one":
			if relation.Subject == tableSchema.TableName {
				// nothing to do
			} else {
				// we can delete just the index or the index and the referencing column as well
				_, err = d.cruds["world"].db.Exec("alter table " + relation.Subject + " drop column " + relation.ObjectName)
				if err != nil {
					errorsList = append(errorsList, err)
				}
			}

		case "has_many_and_belongs_to_many":
		case "has_many":
			_, err = d.cruds["world"].db.Exec("drop table " + relation.GetJoinTableName())
			if err != nil {
				errorsList = append(errorsList, err)
			}
			refId, err := d.cruds["world"].GetReferenceIdByWhereClause("world", squirrel.Eq{"table_name": relation.GetJoinTableName()})
			if len(refId) < 1 {
				errorsList = append(errorsList, fmt.Errorf("failed to find reference id of the join table '%s' when deleting table '%s'", relation.GetJoinTableName(), tableSchema.TableName))
			}
			tablesToRemove = append(tablesToRemove, refId[0])
			if err != nil {
				errorsList = append(errorsList, err)
			}

		}

		otherTable := relation.Subject
		if relation.Subject == tableSchema.TableName {
			otherTable = relation.Object
		}

		otherTableData, err := d.cruds["world"].GetObjectByWhereClause("world", "table_name", otherTable)
		if err != nil {
			errorsList = append(errorsList, err)
			continue
		}

		var otherTableSchema TableInfo
		err = json.Unmarshal([]byte(otherTableData["world_schema_json"].(string)), &otherTableSchema)
		if err != nil {
			errorsList = append(errorsList, err)
			continue
		}
		updatedRelations := make([]api2go.TableRelation, 0)

		for _, rel := range otherTableSchema.Relations {
			if rel.Hash() == relation.Hash() {
				log.Printf("Deleting relation %s from table %s", rel.Hash(), otherTableSchema.TableName)
				// this relation is going to be deleted
			} else {
				updatedRelations = append(updatedRelations, rel)
			}
		}
		otherTableSchema.Relations = updatedRelations
		updatedSchema, err := json.Marshal(otherTableSchema)
		if err != nil {
			errorsList = append(errorsList, err)
			continue
		}

		updatedObject := api2go.NewApi2GoModelWithData("world", nil, 0, nil, otherTableData)
		updatedObject.SetAttributes(map[string]interface{}{
			"world_schema_json": updatedSchema,
		})

		_, err = d.cruds["world"].UpdateWithoutFilters(updatedObject, *req)
		if err != nil {
			errorsList = append(errorsList, err)
			continue

		}

	}

	tablesToRemove = append(tablesToRemove, tableData.GetID())

	_, err = d.cruds["world"].db.Exec("drop table " + tableData.Data["table_name"].(string))
	if err != nil {
		errorsList = append(errorsList, err)
	}

	for _, table := range tablesToRemove {
		err = d.cruds["world"].DeleteWithoutFilters(table, *req)
		if err != nil {
			errorsList = append(errorsList, err)
		}
	}

	restart()

	return nil, []ActionResponse{NewActionResponse("client.notify", NewClientNotification("message", "Table deleted", "Success"))}, errorsList
}

func NewDeleteWorldPerformer(initConfig *CmsConfig, cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := DeleteWorldPerformer{
		cruds:     cruds,
		cmsConfig: initConfig,
	}

	return &handler, nil

}
