package resource

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/artpar/api2go/v2"
	"github.com/artpar/api2go/v2/jsonapi"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/google/uuid"
)

// FindRelationship reads linkage through the parent resource's relation
// metadata. Both the parent check and related row lookup use the normal
// resource lifecycle in one transaction.
func (dbResource *DbResource) FindRelationship(id string, relation jsonapi.Reference, req api2go.Request) (uint, api2go.Responder, error) {
	if req.PlainRequest == nil {
		return 0, nil, fmt.Errorf("missing relationship request")
	}
	if id == "mine" && dbResource.tableInfo.TableName == "user_account" {
		user, _ := req.PlainRequest.Context().Value("user").(*auth.SessionUser)
		if user == nil {
			return 0, nil, api2go.NewHTTPError(nil, "Forbidden", http.StatusForbidden)
		}
		id = user.UserReferenceId.String()
	}
	parsedID, err := uuid.Parse(id)
	if err != nil {
		return 0, nil, api2go.NewHTTPError(err, "invalid relationship id", http.StatusBadRequest)
	}
	offset, hasOffset := req.QueryParams["page[offset]"]
	limit, hasLimit := req.QueryParams["page[limit]"]
	if hasOffset || hasLimit {
		if !hasOffset || !hasLimit || len(offset) == 0 || len(limit) == 0 || len(req.QueryParams["page[number]"]) > 0 || len(req.QueryParams["page[size]"]) > 0 {
			return 0, nil, api2go.NewHTTPError(nil, "use one relationship pagination style", http.StatusBadRequest)
		}
		if _, err := strconv.ParseUint(offset[0], 10, 32); err != nil {
			return 0, nil, api2go.NewHTTPError(err, "invalid relationship offset", http.StatusBadRequest)
		}
		value, err := strconv.ParseUint(limit[0], 10, 32)
		if err != nil || value == 0 {
			return 0, nil, api2go.NewHTTPError(err, "invalid relationship limit", http.StatusBadRequest)
		}
	} else {
		for _, key := range []string{"page[number]", "page[size]"} {
			if values := req.QueryParams[key]; len(values) > 0 {
				value, err := strconv.ParseUint(values[0], 10, 32)
				if err != nil || value == 0 {
					return 0, nil, api2go.NewHTTPError(err, "invalid relationship page", http.StatusBadRequest)
				}
			}
		}
	}

	var target *DbResource
	for _, declared := range dbResource.tableInfo.Relations {
		name, table := declared.GetObjectName(), declared.GetObject()
		if declared.GetSubject() != dbResource.tableInfo.TableName {
			name, table = declared.GetSubjectName(), declared.GetSubject()
		}
		if name == relation.Name && table == relation.Type {
			target = dbResource.Cruds[table]
			break
		}
	}
	if target == nil {
		return 0, nil, api2go.NewHTTPError(nil, "related resource unavailable", http.StatusNotFound)
	}

	tx, err := dbResource.Connection().Beginx()
	if err != nil {
		return 0, nil, err
	}
	defer tx.Rollback()
	parentRequest := req
	parentRequest.QueryParams = nil
	parentRequest.Pagination = nil
	parent, err := dbResource.FindOneWithTransaction(daptinid.DaptinReferenceId(parsedID), parentRequest, tx)
	if err != nil {
		return 0, nil, err
	}

	query := make(map[string][]string, len(req.QueryParams)+2)
	for key, values := range req.QueryParams {
		query[key] = append([]string(nil), values...)
	}
	delete(query, "included_relations")
	query[dbResource.tableInfo.TableName+"_id"] = []string{id}
	query[dbResource.tableInfo.TableName+"Name"] = []string{relation.Name}
	targetRequest := req
	targetRequest.QueryParams = query
	targetRequest.PlainRequest = req.PlainRequest.WithContext(WithMeteringInternal(req.PlainRequest.Context()))
	count, found, err := target.PaginatedFindAllWithTransaction(targetRequest, tx)
	if err != nil {
		return 0, nil, err
	}
	if found == nil {
		return 0, nil, fmt.Errorf("related resource returned no response")
	}
	linked := make([]jsonapi.RelationshipData, 0)
	switch rows := found.Result().(type) {
	case []api2go.Api2GoModel:
		linked = make([]jsonapi.RelationshipData, 0, len(rows))
		for _, row := range rows {
			identifier, err := relationshipTargetID(row)
			if err != nil {
				return 0, nil, err
			}
			linked = append(linked, jsonapi.RelationshipData{Type: relation.Type, ID: identifier})
		}
	case api2go.Api2GoModel:
		identifier, err := relationshipTargetID(rows)
		if err != nil {
			return 0, nil, err
		}
		linked = append(linked, jsonapi.RelationshipData{Type: relation.Type, ID: identifier})
	case nil:
	default:
		return 0, nil, fmt.Errorf("unexpected relationship result %T", rows)
	}
	if err := tx.Commit(); err != nil {
		return 0, nil, err
	}
	return count, NewResponse(parent.Metadata(), jsonapi.Relationship{
		Data: &jsonapi.RelationshipDataContainer{DataArray: linked},
	}, http.StatusOK, nil), nil
}

func relationshipTargetID(row api2go.Api2GoModel) (string, error) {
	if related := row.GetColumnOriginalValue("relation_reference_id"); related != nil {
		reference := daptinid.InterfaceToDIR(related)
		if reference == daptinid.NullReferenceId {
			return "", fmt.Errorf("invalid related resource reference id")
		}
		return reference.String(), nil
	}
	return row.GetID(), nil
}
