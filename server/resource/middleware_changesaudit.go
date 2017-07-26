package resource

import (
	"github.com/artpar/api2go"
	log "github.com/sirupsen/logrus"
	//"gopkg.in/Masterminds/squirrel.v1"
	"errors"

	"github.com/artpar/goms/server/auth"
	"github.com/gorilla/context"
)

type ObjectModificationAuditMiddleware struct {
}

func (omam *ObjectModificationAuditMiddleware) String() string {
	return "TableAccessPermissionChecker"
}

func (omam *ObjectModificationAuditMiddleware) InterceptAfter(dr *DbResource, req *api2go.Request, results []map[string]interface{}) ([]map[string]interface{}, error) {

	if results == nil || len(results) < 1 {
		return results, nil
	}

	returnMap := make([]map[string]interface{}, 0)

	userIdString := context.Get(req.PlainRequest, "user_id")
	userGroupId := context.Get(req.PlainRequest, "usergroup_id")

	currentUserId := ""
	if userIdString != nil {
		currentUserId = userIdString.(string)

	}

	currentUserGroupId := []auth.GroupPermission{}
	if userGroupId != nil {
		currentUserGroupId = userGroupId.([]auth.GroupPermission)
	}

	notIncludedMapCache := make(map[string]bool)
	includedMapCache := make(map[string]bool)

	for _, result := range results {
		//log.Infof("Result: %v", result)

		referenceId := result["reference_id"].(string)
		_, ok := notIncludedMapCache[referenceId]
		if ok {
			continue
		}
		_, ok = includedMapCache[referenceId]
		if ok {
			returnMap = append(returnMap, result)
			continue
		}

		permission := dr.GetRowPermission(result)
		//log.Infof("Row Permission for [%v] for [%v]", permission, result)
		if permission.CanRead(currentUserId, currentUserGroupId) {
			returnMap = append(returnMap, result)
			includedMapCache[referenceId] = true
		} else {
			log.Infof("Result not to be included: %v", result["reference_id"])
			notIncludedMapCache[referenceId] = true
		}
	}

	return returnMap, nil

}

var (
	ERR_UNAUTHORIZED = errors.New("Unauthorized")
)

func (omam *TableAccessPermissionChecker) InterceptBefore(dr *DbResource, req *api2go.Request, results []map[string]interface{}) ([]map[string]interface{}, error) {

	//var err error
	//log.Infof("context: %v", context.GetAll(req.PlainRequest))
	userIdString := context.Get(req.PlainRequest, "user_id")
	userGroupId := context.Get(req.PlainRequest, "usergroup_id")

	currentUserId := ""
	if userIdString != nil {
		currentUserId = userIdString.(string)

	}

	currentUserGroupId := []auth.GroupPermission{}
	if userGroupId != nil {
		currentUserGroupId = userGroupId.([]auth.GroupPermission)
	}

	tableOwnership := dr.GetObjectPermissionByWhereClause("world", "table_name", dr.model.GetName())

	log.Infof("Permission check for TableAccessPermissionChecker type: [%v] on [%v] @%v", req.PlainRequest.Method, dr.model.GetName(), tableOwnership.Permission)
	if req.PlainRequest.Method == "GET" {
		if !tableOwnership.CanRead(currentUserId, currentUserGroupId) {
			return nil, ERR_UNAUTHORIZED
		}
	} else if req.PlainRequest.Method == "PUT" || req.PlainRequest.Method == "PATCH" || req.PlainRequest.Method == "POST" || req.PlainRequest.Method == "DELETE" {
		if !tableOwnership.CanWrite(currentUserId, currentUserGroupId) {
			return nil, ERR_UNAUTHORIZED

		}
	} else {
		return nil, ERR_UNAUTHORIZED

	}

	return results, nil

}
