package resource

import (
	"context"
	"fmt"
	"github.com/artpar/api2go"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/doug-martin/goqu/v9"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"net/http"
)

type deleteWorldPerformer struct {
	cmsConfig *CmsConfig
	cruds     map[string]*DbResource
}

func (d *deleteWorldPerformer) Name() string {
	return "world.delete"
}

func (d *deleteWorldPerformer) DoAction(request Outcome, inFields map[string]interface{}, transaction *sqlx.Tx) (api2go.Responder, []ActionResponse, []error) {

	var worldIdUuid uuid.UUID
	worldIdUuidString := inFields["world_id"]
	asStr, isStr := worldIdUuidString.(string)
	if isStr {
		if asStr == "<nil>" {
			log.Printf("No oauth token set for target store")
		} else {
			worldIdUuid = uuid.MustParse(asStr)
		}
	} else {
		asDir, isDir := worldIdUuidString.(daptinid.DaptinReferenceId)
		if isDir {
			worldIdUuid = uuid.UUID(asDir)
		}
	}

	sessionUser := request.Attributes["user"]
	httpReq := &http.Request{
		Method: "GET",
	}

	httpReq = httpReq.WithContext(context.WithValue(context.Background(), "user", sessionUser))
	req := &api2go.Request{
		PlainRequest: httpReq,
	}

	table, err := d.cruds["world"].FindOneWithTransaction(daptinid.DaptinReferenceId(worldIdUuid), *req, transaction)
	if err != nil {
		return nil, nil, []error{err}
	}

	res := table.Result()
	tableData, ok := res.(api2go.Api2GoModel)
	if !ok {
		return nil, nil, []error{errors.New("failed to find the table")}
	}

	schemaJson := tableData.GetAttributes()["world_schema_json"]

	var tableSchema TableInfo
	err = json.Unmarshal([]byte(schemaJson.(string)), &tableSchema)
	if err != nil {
		return nil, nil, []error{err}
	}
	relations := tableSchema.Relations

	var tablesToRemove []daptinid.DaptinReferenceId
	errorsList := make([]error, 0)

	for _, relation := range relations {
		switch relation.Relation {

		case "belongs_to":
			if relation.Subject == tableSchema.TableName {
				// nothing to do
			} else {
				// we can delete just the index or the index and the referencing column as well
				_, err = transaction.Exec("alter table " + relation.Subject + " drop column " + relation.ObjectName)
				if err != nil {
					errorsList = append(errorsList, err)
				}
			}
		case "has_one":
			if relation.Subject == tableSchema.TableName {
				// nothing to do
			} else {
				// we can delete just the index or the index and the referencing column as well
				_, err = transaction.Exec("alter table " + relation.Subject + " drop column " + relation.ObjectName)
				if err != nil {
					errorsList = append(errorsList, err)
				}
			}

		case "has_many_and_belongs_to_many":
		case "has_many":
			_, err = transaction.Exec("drop table " + relation.GetJoinTableName())
			if err != nil {
				errorsList = append(errorsList, err)
			}
			refId, err := GetReferenceIdByWhereClauseWithTransaction("world", transaction, goqu.Ex{"table_name": relation.GetJoinTableName()})
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

		otherTableData, err := d.cruds["world"].GetObjectByWhereClauseWithTransaction("world", "table_name", otherTable, transaction)
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

		_, err = d.cruds["world"].UpdateWithoutFilters(updatedObject, *req, transaction)
		if err != nil {
			errorsList = append(errorsList, err)
			return nil, nil, errorsList
		}

	}

	uuidVal := uuid.MustParse(tableData.GetID())
	tablesToRemove = append(tablesToRemove, daptinid.DaptinReferenceId(uuidVal))

	_, err = transaction.Exec("drop table " + tableData.GetAttributes()["table_name"].(string))
	if err != nil {
		errorsList = append(errorsList, err)
		return nil, nil, errorsList
	}

	for _, table := range tablesToRemove {
		err = d.cruds["world"].DeleteWithoutFilters(table, *req, transaction)
		if err != nil {
			errorsList = append(errorsList, err)
		}
	}

	//restart()

	return nil, []ActionResponse{NewActionResponse("client.notify", NewClientNotification("message", "Table deleted", "Success"))}, errorsList
}

func NewDeleteWorldPerformer(initConfig *CmsConfig, cruds map[string]*DbResource) (ActionPerformerInterface, error) {

	handler := deleteWorldPerformer{
		cruds:     cruds,
		cmsConfig: initConfig,
	}

	return &handler, nil

}
